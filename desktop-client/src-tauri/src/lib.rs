// Liaison desktop client. Menubar GUI wrapper around the existing
// liaison-edge CLI binary.

#![allow(dead_code)]

use std::path::PathBuf;
use std::sync::Mutex;
use std::time::Duration;

use tauri::{
    menu::{Menu, MenuEvent, MenuItem, PredefinedMenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    AppHandle, Emitter, Manager, PhysicalPosition, State, WindowEvent,
};
use tauri_plugin_opener::OpenerExt;

mod api_client;
mod auth;
mod cli_detector;
mod edge_supervisor;
mod state;
mod status_poller;

use edge_supervisor::{
    liaison_data_dir, liaison_log_dir, write_edge_config, EdgeConfig, EdgeSupervisor,
    IntendedState, SupervisorHandle, TrayState,
};

const DEFAULT_BASE_URL: &str = "https://liaison.cloud";
const DEFAULT_MANAGER_PORT: u16 = 30012;
const TRAY_EVENT: &str = "tray_state_changed";
const LOGIN_TIMEOUT_SECS: u64 = 300;
const DESKTOP_EDGE_NAME: &str = "Liaison Desktop";

/// Tray menu strings, kept in sync with src/i18n.ts on the React side.
/// Two locales for now (en / zh); add more as needed.
struct TrayLang {
    login: &'static str,
    logout: &'static str,
    pause: &'static str,
    resume: &'static str,
    dashboard: &'static str,
    quit: &'static str,
}

const TRAY_LANG_EN: TrayLang = TrayLang {
    login: "Sign in",
    logout: "Sign out",
    pause: "Pause",
    resume: "Resume",
    dashboard: "Open Dashboard…",
    quit: "Quit Liaison",
};

const TRAY_LANG_ZH: TrayLang = TrayLang {
    login: "登录",
    logout: "退出登录",
    pause: "暂停连接",
    resume: "继续连接",
    dashboard: "打开 Dashboard…",
    quit: "退出 Liaison",
};

/// Resolve `state.json`'s `locale` field (or the OS locale, on first
/// launch) into one of the two TrayLang tables.
fn pick_tray_locale(persisted: &Option<String>) -> &'static TrayLang {
    let chosen = persisted.as_deref().map(str::to_lowercase);
    let lang = chosen.unwrap_or_else(detect_os_locale);
    if lang.starts_with("zh") {
        &TRAY_LANG_ZH
    } else {
        &TRAY_LANG_EN
    }
}

/// Best-effort OS locale lookup for first-launch defaulting. Reads the
/// LANG / LC_* env vars Unix sets and that Windows respects via the
/// MSVC runtime; anything starting with "zh" maps to Chinese, all
/// other values fall through to English.
fn detect_os_locale() -> String {
    for var in ["LC_ALL", "LC_MESSAGES", "LANG"] {
        if let Ok(v) = std::env::var(var) {
            if !v.is_empty() {
                return v.to_lowercase();
            }
        }
    }
    String::new()
}

/// Server configuration that can change at runtime when the user
/// switches deployments via the popup. Mutex-wrapped on AppState so
/// commands can swap it atomically.
#[derive(Clone)]
pub struct ServerConfig {
    pub base_url: String,
    pub manager_addr: String,
}

pub struct AppState {
    pub server: Mutex<ServerConfig>,
    pub binary_path: Mutex<PathBuf>,
    pub config_path: PathBuf,
    pub log_file: PathBuf,
    pub supervisor: Mutex<Option<SupervisorHandle>>,
}

impl AppState {
    fn server_snapshot(&self) -> ServerConfig {
        self.server.lock().expect("poisoned").clone()
    }
}

#[derive(serde::Serialize, Clone)]
struct StatusPayload {
    tray: TrayState,
    logged_in: bool,
    cli_hits: Vec<cli_detector::CliHit>,
    base_url: String,
    locale: Option<String>,
}

fn current_tray_state(state: &AppState) -> (TrayState, bool, String) {
    let base_url = state.server.lock().expect("poisoned").base_url.clone();
    let logged_in = auth::get_pat_from_keychain(&base_url).is_ok();
    let tray = if let Some(handle) = state.supervisor.lock().expect("poisoned").as_ref() {
        handle.current()
    } else if logged_in {
        TrayState::Paused
    } else {
        TrayState::LoggedOut
    };
    (tray, logged_in, base_url)
}

#[tauri::command]
fn cmd_get_status(state: State<'_, AppState>) -> StatusPayload {
    let (tray, logged_in, base_url) = current_tray_state(&state);
    let locale = crate::state::load().locale;
    debug_log(format!(
        "cmd_get_status: tray={:?} logged_in={} base_url={} locale={:?}",
        tray, logged_in, base_url, locale
    ));
    StatusPayload {
        tray,
        logged_in,
        cli_hits: cli_detector::detect_existing_cli(),
        base_url,
        locale,
    }
}

/// Persist the user's locale choice. Frontend strings repaint
/// immediately from the React side; tray-menu strings only refresh
/// on next app launch since rebuilding NSMenu / system tray menus
/// at runtime is more invasive than this MVP needs.
#[tauri::command]
fn cmd_set_locale(locale: String) -> Result<(), String> {
    let normalised = match locale.as_str() {
        "en" | "zh" => locale,
        other => return Err(format!("unsupported locale: {other}")),
    };
    let mut s = crate::state::load();
    s.locale = Some(normalised);
    crate::state::save(&s);
    Ok(())
}

#[tauri::command]
async fn cmd_login(app: AppHandle, state: State<'_, AppState>) -> Result<(), String> {
    let server = state.server_snapshot();
    let base_url = server.base_url.clone();
    let manager_addr = server.manager_addr;
    let config_path = state.config_path.clone();
    let log_file = state.log_file.clone();
    let binary_path = state.binary_path.lock().expect("poisoned").clone();

    debug_log(format!(
        "cmd_login: invoked base_url={} manager_addr={} config_path={} binary_path={}",
        base_url,
        manager_addr,
        config_path.display(),
        binary_path.display()
    ));

    // Probe both candidate dashboard mount paths (SaaS uses
    // /dashboard/cli-auth, private deployments commonly use /cli-auth)
    // so a single client binary works against either without a setting.
    let cli_auth_path = auth::discover_cli_auth_path(&base_url).await;
    debug_log(format!("cmd_login: cli_auth_path={}", cli_auth_path));

    let pending = auth::start_login(&base_url, &cli_auth_path, None)
        .map_err(|e| {
            let msg = format!("start_login failed: {e}");
            debug_log(format!("cmd_login: ERR {msg}"));
            msg
        })?;
    let url = pending.auth_url.clone();
    debug_log(format!("cmd_login: opening browser auth_url={}", url));
    app.opener()
        .open_url(&url, None::<&str>)
        .map_err(|e| {
            let msg = format!("open_url failed: {e}");
            debug_log(format!("cmd_login: ERR {msg}"));
            msg
        })?;

    debug_log("cmd_login: waiting for browser callback (timeout=300s)");
    let pat = pending
        .await_token(Duration::from_secs(LOGIN_TIMEOUT_SECS))
        .await
        .map_err(|e| {
            let msg = format!("await_token failed: {e}");
            debug_log(format!("cmd_login: ERR {msg}"));
            msg
        })?;
    debug_log(format!("cmd_login: token received len={}", pat.len()));

    auth::save_pat_to_keychain(&base_url, &pat).map_err(|e| {
        let msg = format!("save_pat_to_keychain failed: {e}");
        debug_log(format!("cmd_login: ERR {msg}"));
        msg
    })?;
    debug_log("cmd_login: pat saved to keychain");

    // From here on, any failure must roll the keychain entry back —
    // otherwise the popup sees a stored PAT, decides we're "logged
    // in but not running", paints the 恢复连接 button, and gets stuck
    // because no edge.yaml ever got written.
    let api = api_client::ApiClient::new(&base_url, &pat).map_err(|e| {
        debug_log(format!("cmd_login: ERR ApiClient::new failed: {e}"));
        login_rollback(&base_url, e.to_string())
    })?;
    debug_log("cmd_login: api client built");

    let keys = api
        .create_edge(DESKTOP_EDGE_NAME, "menubar GUI")
        .await
        .map_err(|e| {
            debug_log(format!("cmd_login: ERR create_edge failed: {e}"));
            login_rollback(&base_url, e.to_string())
        })?;
    debug_log(format!(
        "cmd_login: edge created on manager (access_key.len={})",
        keys.access_key.len()
    ));

    let cfg = EdgeConfig {
        manager_addr,
        access_key: keys.access_key,
        secret_key: keys.secret_key,
        tls_enable: true,
        insecure_skip_verify: true,
        log_file,
    };
    write_edge_config(&config_path, &cfg).map_err(|e| {
        debug_log(format!(
            "cmd_login: ERR write_edge_config to {} failed: {e}",
            config_path.display()
        ));
        login_rollback(&base_url, e.to_string())
    })?;
    debug_log(format!(
        "cmd_login: edge config written to {}",
        config_path.display()
    ));

    // Logging in implies the user wants to be connected. If state.json
    // still carries `intended = Stopped` from an earlier pause on the
    // old server, the new supervisor would inherit it and the popup
    // would land on Paused, forcing a manual 恢复连接 click. Reset
    // before start_supervisor reads state.json so the run loop sees
    // Running on the very first poll.
    persist_intended(IntendedState::Running);
    debug_log("cmd_login: persisted intended=Running, starting supervisor");

    start_supervisor(&app, &state, binary_path, config_path, Some(pat));
    debug_log("cmd_login: supervisor spawned, login complete");

    // Pull the popup back to the front. The browser took over focus
    // for the OAuth round-trip and the blur handler hid the popup
    // somewhere along the way; without this the user finishes login
    // and finds nothing visibly happened, then has to dig the tray
    // icon out of the system tray overflow to see the new state.
    if let Some(popup) = app.get_webview_window("popup") {
        position_popup_at_corner(&popup);
        let _ = popup.unminimize();
        let _ = popup.show();
        let _ = popup.set_focus();
    }
    Ok(())
}

/// Wipe the per-host PAT and surface the original error. Used by
/// cmd_login when a step after save_pat_to_keychain fails so the
/// keychain doesn't end up populated for a session we couldn't
/// actually finish setting up.
fn login_rollback(base_url: &str, err: String) -> String {
    let _ = auth::delete_pat_from_keychain(base_url);
    err
}

#[tauri::command]
async fn cmd_logout(state: State<'_, AppState>) -> Result<(), String> {
    let base_url = state.server.lock().expect("poisoned").base_url.clone();
    if let Some(h) = state.supervisor.lock().expect("poisoned").take() {
        h.set_intended(IntendedState::Stopped);
    }
    let _ = auth::delete_pat_from_keychain(&base_url);
    let _ = std::fs::remove_file(&state.config_path);
    Ok(())
}

/// Switch the active Liaison server. Wipes the current PAT, edge
/// config, and supervisor so the popup returns to LoggedOut and the
/// user re-authenticates against the new deployment. Persists the
/// new base_url to state.json so the next launch picks it up.
#[tauri::command]
async fn cmd_set_server(
    app: AppHandle,
    state: State<'_, AppState>,
    new_base_url: String,
) -> Result<(), String> {
    let trimmed = new_base_url.trim().trim_end_matches('/').to_string();
    let parsed = url::Url::parse(&trimmed)
        .map_err(|e| format!("无效的服务器地址：{e}"))?;
    if !matches!(parsed.scheme(), "http" | "https") {
        return Err("服务器地址必须以 http:// 或 https:// 开头".into());
    }
    if parsed.host_str().unwrap_or("").is_empty() {
        return Err("服务器地址缺少主机名".into());
    }

    let manager_addr = derive_manager_addr(&trimmed);
    let old_base_url = state.server.lock().expect("poisoned").base_url.clone();

    // Tear down old session: kill supervisor → drop PAT → drop edge yaml.
    if let Some(h) = state.supervisor.lock().expect("poisoned").take() {
        h.set_intended(IntendedState::Stopped);
    }
    let _ = auth::delete_pat_from_keychain(&old_base_url);
    let _ = std::fs::remove_file(&state.config_path);

    // Swap in the new server. Persist before emitting so the next
    // cmd_get_status reflects the new state. Also reset intent to
    // Running — switching servers is a deliberate fresh start, so
    // the old server's pause state shouldn't carry forward to the
    // new deployment's first launch.
    {
        let mut s = state.server.lock().expect("poisoned");
        s.base_url = trimmed.clone();
        s.manager_addr = manager_addr;
    }
    let mut persisted = crate::state::load();
    persisted.base_url = trimmed;
    persisted.set_intended(IntendedState::Running);
    crate::state::save(&persisted);

    // Tell the popup to repaint as LoggedOut.
    let _ = app.emit(TRAY_EVENT, &TrayState::LoggedOut);
    Ok(())
}

/// Headless cleanup invoked at uninstall time (see main.rs). Deletes
/// the active server's PAT from the OS credential store so the
/// uninstaller doesn't leave a leftover entry behind. Also tries the
/// legacy un-namespaced slot for users upgrading from a build that
/// pre-dates per-host keychain namespacing. Best-effort: any failure
/// is swallowed so an unfixable credential entry doesn't block the
/// uninstaller.
pub fn cleanup_credentials() {
    let s = crate::state::load();
    if !s.base_url.is_empty() {
        let _ = auth::delete_pat_from_keychain(&s.base_url);
    }
    // Legacy "pat" entry (no host suffix). delete_pat_from_keychain
    // routes through keychain_user("") -> no host -> legacy slot.
    let _ = auth::delete_pat_from_keychain("");
}

pub fn debug_log(msg: impl AsRef<str>) {
    use std::io::Write;
    let Ok(dir) = edge_supervisor::liaison_log_dir() else {
        return;
    };
    let _ = std::fs::create_dir_all(&dir);
    let path = dir.join("liaison-desktop-debug.log");
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(&path)
    {
        let ts = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0);
        let _ = writeln!(f, "[{ts}] {}", msg.as_ref());
    }
}

#[tauri::command]
fn cmd_pause(state: State<'_, AppState>) {
    debug_log("cmd_pause: invoked");
    let has = {
        let guard = state.supervisor.lock().expect("poisoned");
        if let Some(h) = guard.as_ref() {
            h.set_intended(IntendedState::Stopped);
            // Optimistically flip tray state so the popup's refresh()
            // call (fired the moment invoke('cmd_pause') resolves)
            // sees Paused, instead of racing the supervisor run loop.
            h.force_state(TrayState::Paused);
            true
        } else {
            false
        }
    };
    debug_log(format!("cmd_pause: supervisor present={has}"));
    persist_intended(IntendedState::Stopped);
    debug_log("cmd_pause: persisted Stopped");
}

#[tauri::command]
fn cmd_resume(app: AppHandle, state: State<'_, AppState>) {
    debug_log("cmd_resume: invoked");
    let has = {
        let guard = state.supervisor.lock().expect("poisoned");
        if let Some(h) = guard.as_ref() {
            h.set_intended(IntendedState::Running);
            // Optimistically flip to Connecting; the supervisor's run
            // loop will re-emit Connecting itself when it actually
            // spawns the child, and the poller promotes to Online once
            // the manager confirms.
            h.force_state(TrayState::Connecting);
            true
        } else {
            false
        }
    };
    persist_intended(IntendedState::Running);
    debug_log(format!("cmd_resume: supervisor_present={has}; persisted Running"));

    // If no supervisor exists (e.g. user paused, app restarted, or
    // edge crashed permanently), kick off a fresh one so resume
    // actually starts the connector again instead of silently no-oping.
    if !has {
        let base_url = state.server.lock().expect("poisoned").base_url.clone();
        if state.config_path.exists() && auth::get_pat_from_keychain(&base_url).is_ok() {
            let bin = state.binary_path.lock().expect("poisoned").clone();
            let cfg = state.config_path.clone();
            debug_log("cmd_resume: spawning fresh supervisor");
            start_supervisor(&app, &state, bin, cfg, None);
        }
    }
}

fn persist_intended(intended: IntendedState) {
    let mut s = crate::state::load();
    s.set_intended(intended);
    crate::state::save(&s);
}

#[tauri::command]
fn cmd_open_dashboard(app: AppHandle, state: State<'_, AppState>) -> Result<(), String> {
    let base_url = state.server.lock().expect("poisoned").base_url.clone();
    let url = format!("{}/dashboard/", base_url);
    app.opener()
        .open_url(&url, None::<&str>)
        .map_err(|e| e.to_string())
}

fn start_supervisor(
    app: &AppHandle,
    state: &AppState,
    binary_path: PathBuf,
    config_path: PathBuf,
    pat_for_poller: Option<String>,
) {
    let mut slot = state.supervisor.lock().expect("poisoned");
    if slot.is_some() {
        return;
    }
    let supervisor_log = state
        .log_file
        .parent()
        .unwrap_or_else(|| std::path::Path::new("."))
        .join("liaison-edge-supervisor.log");
    // Seed the supervisor with the persisted intent at construction
    // time so the watch channel's initial value matches what we want.
    // Calling set_intended() afterwards would bump the channel version
    // even when the value is unchanged, and the run loop's first
    // tokio::select! would interpret that as "intent changed mid-run"
    // and pointlessly kill+respawn the freshly-spawned edge child.
    let persisted = crate::state::load();
    let supervisor = EdgeSupervisor::with_intent(
        binary_path,
        config_path,
        supervisor_log,
        persisted.intended_as_enum(),
    );
    let handle = supervisor.handle();

    let app_for_events = app.clone();
    let mut rx = handle.subscribe();
    tauri::async_runtime::spawn(async move {
        while let Ok(s) = rx.recv().await {
            let r = app_for_events.emit(TRAY_EVENT, &s);
            debug_log(format!("bridge: app.emit {:?} -> ok={}", s, r.is_ok()));
        }
    });

    // Spawn the API poller so Online vs Connecting reflects the actual
    // tunnel state, not just process liveness.
    let server = state.server.lock().expect("poisoned").clone();
    let pat = pat_for_poller.or_else(|| auth::get_pat_from_keychain(&server.base_url).ok());
    if let Some(pat) = pat {
        status_poller::spawn_poller(
            server.base_url.clone(),
            pat,
            DESKTOP_EDGE_NAME.to_string(),
            handle.clone(),
        );
    }

    *slot = Some(handle);
    drop(slot);

    tauri::async_runtime::spawn(async move {
        supervisor.run().await;
    });
}

fn derive_manager_addr(base_url: &str) -> String {
    if let Ok(parsed) = url::Url::parse(base_url) {
        if let Some(host) = parsed.host_str() {
            return format!("{host}:{DEFAULT_MANAGER_PORT}");
        }
    }
    format!("liaison.cloud:{DEFAULT_MANAGER_PORT}")
}

fn build_initial_state() -> Result<AppState, String> {
    let data_dir = liaison_data_dir().map_err(|e| e.to_string())?;
    let log_dir = liaison_log_dir().map_err(|e| e.to_string())?;
    std::fs::create_dir_all(&data_dir).map_err(|e| e.to_string())?;
    std::fs::create_dir_all(&log_dir).map_err(|e| e.to_string())?;

    // Resolution order for the active server:
    //   1. LIAISON_BASE_URL env var (debug / dev override)
    //   2. state.json's persisted base_url (set last time the user
    //      chose a server via the popup)
    //   3. DEFAULT_BASE_URL (the public SaaS endpoint)
    let persisted = crate::state::load();
    let base_url = std::env::var("LIAISON_BASE_URL")
        .ok()
        .filter(|s| !s.is_empty())
        .unwrap_or(persisted.base_url);

    // Same precedence for manager_addr; default is derived from the
    // resolved base_url host. Explicit env var wins for the rare case
    // where dashboard and manager don't share a hostname.
    let manager_addr = std::env::var("LIAISON_MANAGER_ADDR")
        .ok()
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| derive_manager_addr(&base_url));

    eprintln!(
        "[liaison-desktop] base_url={base_url} manager_addr={manager_addr} \
         (override via LIAISON_BASE_URL / LIAISON_MANAGER_ADDR)"
    );

    Ok(AppState {
        server: Mutex::new(ServerConfig {
            base_url,
            manager_addr,
        }),
        binary_path: Mutex::new(PathBuf::new()),
        config_path: data_dir.join("liaison-edge.yaml"),
        log_file: log_dir.join("liaison-edge.log"),
        supervisor: Mutex::new(None),
    })
}

fn edge_binary_name() -> String {
    if cfg!(target_os = "windows") {
        "liaison-edge-windows-amd64.exe".to_string()
    } else if cfg!(target_os = "macos") {
        let arch = if cfg!(target_arch = "aarch64") {
            "arm64"
        } else {
            "amd64"
        };
        format!("liaison-edge-darwin-{arch}")
    } else {
        let arch = if cfg!(target_arch = "aarch64") {
            "arm64"
        } else {
            "amd64"
        };
        format!("liaison-edge-linux-{arch}")
    }
}

fn resolve_edge_binary(app: &AppHandle) -> PathBuf {
    let bin_name = edge_binary_name();

    let mut candidates: Vec<PathBuf> = Vec::new();
    if let Ok(rd) = app.path().resource_dir() {
        candidates.push(rd.join("resources").join(&bin_name));
        candidates.push(rd.join(&bin_name));
    }
    // Dev fallback: cargo run leaves resources next to Cargo.toml.
    candidates.push(
        PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("resources")
            .join(&bin_name),
    );

    for cand in &candidates {
        if cand.exists() {
            eprintln!("[liaison-desktop] edge binary: {}", cand.display());
            return cand.clone();
        }
    }

    eprintln!(
        "[liaison-desktop] edge binary NOT FOUND. Tried these paths in order:"
    );
    for cand in &candidates {
        eprintln!("  - {}", cand.display());
    }
    eprintln!(
        "  hint: run `make desktop-client-copy-edge-host` from repo root \
         to populate desktop-client/src-tauri/resources/{bin_name}"
    );

    candidates
        .into_iter()
        .next()
        .unwrap_or_else(|| PathBuf::from(&bin_name))
}

fn handle_menu_event(app: &AppHandle, event: MenuEvent) {
    let id = event.id().as_ref().to_string();
    let app = app.clone();
    match id.as_str() {
        "login" => {
            tauri::async_runtime::spawn(async move {
                let state = app.state::<AppState>();
                if let Err(e) = cmd_login(app.clone(), state).await {
                    eprintln!("login failed: {e}");
                }
            });
        }
        "logout" => {
            tauri::async_runtime::spawn(async move {
                let state = app.state::<AppState>();
                let _ = cmd_logout(state).await;
            });
        }
        "pause" => cmd_pause(app.state::<AppState>()),
        "resume" => cmd_resume(app.clone(), app.state::<AppState>()),
        "dashboard" => {
            let state = app.state::<AppState>();
            let _ = cmd_open_dashboard(app.clone(), state);
        }
        "quit" => {
            let state = app.state::<AppState>();
            if let Some(h) = state.supervisor.lock().expect("poisoned").take() {
                h.set_intended(IntendedState::Stopped);
            }
            app.exit(0);
        }
        _ => {}
    }
}

fn toggle_popup(app: &AppHandle, click_pos: Option<PhysicalPosition<f64>>) {
    if let Some(window) = app.get_webview_window("popup") {
        if window.is_visible().unwrap_or(false) {
            let _ = window.hide();
        } else {
            if let Some(pos) = click_pos {
                position_popup_under(&window, pos);
            }
            let _ = window.show();
            let _ = window.set_focus();
        }
    }
}

fn position_popup_under(
    window: &tauri::WebviewWindow,
    click_pos: PhysicalPosition<f64>,
) {
    let outer = match window.outer_size() {
        Ok(s) => s,
        Err(_) => return,
    };

    // Center the popup horizontally on the click point. Vertical
    // direction depends on which edge of the screen the tray sits on:
    //   - macOS: tray at top, popup goes BELOW (click_y + offset).
    //   - Windows: tray at bottom-right by default, popup must go
    //     ABOVE the click or the buttons fall off the screen.
    // We pick by checking how much room remains below the click.
    const GAP: i32 = 6;
    let mut x = click_pos.x as i32 - (outer.width as i32 / 2);

    let (min_x, max_x, room_below, monitor_top) =
        if let Ok(Some(monitor)) = window.current_monitor() {
            let mp = monitor.position();
            let ms = monitor.size();
            let bottom = mp.y + ms.height as i32;
            let room = bottom - click_pos.y as i32;
            let min_x = mp.x + 8;
            let max_x = mp.x + ms.width as i32 - outer.width as i32 - 8;
            (min_x, max_x, room, mp.y)
        } else {
            // No monitor info: fall back to "below" with no clamping.
            (i32::MIN, i32::MAX, i32::MAX, 0)
        };

    x = x.clamp(min_x, max_x);

    let needed = outer.height as i32 + GAP * 2;
    let y = if room_below >= needed {
        click_pos.y as i32 + GAP
    } else {
        // Not enough room below — anchor popup ABOVE the click point.
        // Clamp to the top of the monitor so we never overflow upward.
        let candidate = click_pos.y as i32 - outer.height as i32 - GAP;
        candidate.max(monitor_top + 8)
    };

    let _ = window.set_position(PhysicalPosition::new(x, y));
}

/// Force the popup window into a fully borderless, fully transparent,
/// rounded-corner state on macOS. Tauri's `transparent: true` /
/// `decorations: false` / `shadow: false` configs *should* be enough,
/// but in practice the NSWindow still ends up with a dark default
/// material drawn at the four corners outside our 12px rounded path
/// (visible as a black wedge against any light backdrop). Force
/// every relevant flag via objc so the corners are genuinely
/// transparent:
///
///   - styleMask = NSWindowStyleMaskBorderless (no chrome at all)
///   - opaque = NO, backgroundColor = clearColor (NSWindow draws nothing)
///   - hasShadow = NO (kill the OS-drawn shadow that was leaking
///     past the rounded mask)
///   - contentView.layer.cornerRadius + masksToBounds = clip every
///     pixel outside the 12px path, including the layer's own
///     background fill
///   - contentView.layer.backgroundColor = clear (in case the auto-
///     created CALayer defaulted to a system color like windowBg)
#[cfg(target_os = "macos")]
fn round_macos_window_corners(window: &tauri::WebviewWindow, radius: f64) {
    use objc2::msg_send;
    use objc2::runtime::{AnyClass, AnyObject};

    let Ok(ns_window) = window.ns_window() else {
        return;
    };
    if ns_window.is_null() {
        return;
    }
    unsafe {
        let ns_window = ns_window as *mut AnyObject;

        // NSWindowStyleMaskBorderless == 0
        let _: () = msg_send![ns_window, setStyleMask: 0_u64];
        let _: () = msg_send![ns_window, setOpaque: false];
        let _: () = msg_send![ns_window, setHasShadow: false];

        let nscolor = AnyClass::get("NSColor").expect("NSColor class");
        let clear_color: *mut AnyObject = msg_send![nscolor, clearColor];
        if !clear_color.is_null() {
            let _: () = msg_send![ns_window, setBackgroundColor: clear_color];
        }

        let content_view: *mut AnyObject = msg_send![ns_window, contentView];
        if content_view.is_null() {
            return;
        }
        let _: () = msg_send![content_view, setWantsLayer: true];
        let layer: *mut AnyObject = msg_send![content_view, layer];
        if layer.is_null() {
            return;
        }
        let _: () = msg_send![layer, setCornerRadius: radius];
        let _: () = msg_send![layer, setMasksToBounds: true];
        let _: () = msg_send![layer, setBorderWidth: 0.0_f64];

        if !clear_color.is_null() {
            let cg_clear: *mut AnyObject = msg_send![clear_color, CGColor];
            if !cg_clear.is_null() {
                let _: () = msg_send![layer, setBackgroundColor: cg_clear];
            }
        }
    }
}

/// Drop the popup at the corner of the primary monitor where the tray
/// icon typically lives. Used both at first launch and when a
/// second-launch (desktop-shortcut double-click forwarded by the
/// single-instance plugin) needs to surface the popup — without this
/// the show() may bring back the popup at its last hidden position,
/// which can be off-screen if the user has since disconnected an
/// external monitor.
fn position_popup_at_corner(window: &tauri::WebviewWindow) {
    let Ok(Some(monitor)) = window.current_monitor() else { return };
    let Ok(outer) = window.outer_size() else { return };
    let mp = monitor.position();
    let ms = monitor.size();
    // Right-edge inset. macOS gets a wider margin so the popup
    // doesn't appear flush against the very edge of the desktop —
    // 8px looked OK in screenshots but feels cramped under the
    // actual menu-bar tray icons. Windows / Linux can stay tight.
    #[cfg(target_os = "macos")]
    let right_margin = 16;
    #[cfg(not(target_os = "macos"))]
    let right_margin = 8;
    let x = mp.x + ms.width as i32 - outer.width as i32 - right_margin;
    #[cfg(target_os = "macos")]
    let y = mp.y + 28;
    #[cfg(not(target_os = "macos"))]
    let y = mp.y + ms.height as i32 - outer.height as i32 - 48;
    let _ = window.set_position(PhysicalPosition::new(x, y));
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let mut builder = tauri::Builder::default();

    // Single-instance lock: when a second copy is launched (e.g. user
    // double-clicks the .lnk again, or the .msi auto-runs after
    // install), shoulder-tap the running one to surface its popup
    // and exit the new process. Avoids two tray icons fighting over
    // the same keychain entry, yaml, and edge subprocess.
    #[cfg(desktop)]
    {
        builder = builder.plugin(tauri_plugin_single_instance::init(
            |app, _args, _cwd| {
                // Second-launch path: user double-clicked the desktop
                // shortcut while we were already running. Re-anchor the
                // popup at the tray corner first — its prior position
                // could be off-screen (multi-monitor disconnect, dragged
                // by user) and the previous show()+set_focus() pair
                // looked silent. Then unminimize → show → focus to bring
                // it forward; on Windows set_focus alone can be ignored
                // due to foreground-window restrictions, but the
                // alwaysOnTop=true config in tauri.conf.json plus the
                // explicit reposition makes the popup definitely visible.
                if let Some(window) = app.get_webview_window("popup") {
                    position_popup_at_corner(&window);
                    let _ = window.unminimize();
                    let _ = window.show();
                    let _ = window.set_focus();
                }
            },
        ));
    }

    builder
        .plugin(tauri_plugin_opener::init())
        .setup(|app| {
            let state = build_initial_state()
                .map_err(|e| Box::<dyn std::error::Error>::from(e))?;
            *state.binary_path.lock().expect("poisoned") =
                resolve_edge_binary(&app.handle());
            app.manage(state);

            // Tray menu strings come from whatever locale was persisted.
            // First-launch users get the OS-locale-derived default;
            // changing locale via the popup updates state.json but the
            // tray menu only repaints on next app launch (acceptable
            // MVP — rebuilding the NSMenu / system tray menu at
            // runtime adds platform-specific complexity we don't need
            // until the locale picker sees real use).
            let tray_lang = pick_tray_locale(&crate::state::load().locale);
            let menu = Menu::with_items(
                app,
                &[
                    &MenuItem::with_id(app, "login", tray_lang.login, true, None::<&str>)?,
                    &MenuItem::with_id(app, "logout", tray_lang.logout, true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "pause", tray_lang.pause, true, None::<&str>)?,
                    &MenuItem::with_id(app, "resume", tray_lang.resume, true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "dashboard", tray_lang.dashboard, true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "quit", tray_lang.quit, true, None::<&str>)?,
                ],
            )?;

            let _tray = TrayIconBuilder::with_id("main")
                .menu(&menu)
                .show_menu_on_left_click(false)
                .icon(app.default_window_icon().unwrap().clone())
                .tooltip("Liaison")
                .on_menu_event(|app, event| handle_menu_event(app, event))
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        position,
                        ..
                    } = event
                    {
                        toggle_popup(tray.app_handle(), Some(position));
                    }
                })
                .build(app)?;

            // Hide popup when it loses focus (clicking elsewhere).
            // Skipped during the brief launch window so the splash
            // popup we explicitly show below isn't auto-hidden by
            // the event that immediately follows window creation.
            let blur_armed = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
            if let Some(popup) = app.get_webview_window("popup") {
                let popup_for_blur = popup.clone();
                let armed = blur_armed.clone();
                popup.on_window_event(move |event| {
                    if let WindowEvent::Focused(false) = event {
                        if armed.load(std::sync::atomic::Ordering::Relaxed) {
                            let _ = popup_for_blur.hide();
                        }
                    }
                });
            }

            // Auto-start supervisor if a PAT + edge config already exist
            // (subsequent-launch path).
            {
                let st = app.state::<AppState>();
                let base_url = st.server.lock().expect("poisoned").base_url.clone();
                if auth::get_pat_from_keychain(&base_url).is_ok() && st.config_path.exists() {
                    let bin = st.binary_path.lock().expect("poisoned").clone();
                    let cfg = st.config_path.clone();
                    start_supervisor(&app.handle(), &st, bin, cfg, None);
                }
            }

            // Show the popup on launch so the user has visible feedback
            // that the app has started. Position it near the corner of
            // the primary monitor where the tray icon typically lives:
            //   macOS: upper-right (just below the menubar).
            //   Windows / Linux: lower-right (just above the taskbar).
            // The user can drag it elsewhere via the header drag-region.
            if let Some(popup) = app.get_webview_window("popup") {
                #[cfg(target_os = "macos")]
                round_macos_window_corners(&popup, 12.0);
                position_popup_at_corner(&popup);
                let _ = popup.show();
                let _ = popup.set_focus();
            }
            // Arm the blur-hide handler after a short delay so the
            // initial focus events fired during window creation don't
            // immediately collapse the popup we just opened.
            let blur_armed_for_arm = blur_armed.clone();
            tauri::async_runtime::spawn(async move {
                tokio::time::sleep(std::time::Duration::from_millis(800)).await;
                blur_armed_for_arm.store(true, std::sync::atomic::Ordering::Relaxed);
            });

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            cmd_login,
            cmd_logout,
            cmd_pause,
            cmd_resume,
            cmd_get_status,
            cmd_open_dashboard,
            cmd_set_server,
            cmd_set_locale,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn derive_uses_base_url_host_with_default_port() {
        assert_eq!(
            derive_manager_addr("https://liaison.cloud"),
            "liaison.cloud:30012"
        );
        assert_eq!(
            derive_manager_addr("https://liaison.lan"),
            "liaison.lan:30012"
        );
        assert_eq!(
            derive_manager_addr("https://example.com:8443/dashboard/"),
            "example.com:30012"
        );
    }

    #[test]
    fn derive_falls_back_when_url_unparseable() {
        assert_eq!(derive_manager_addr("not a url"), "liaison.cloud:30012");
        assert_eq!(derive_manager_addr(""), "liaison.cloud:30012");
    }
}
