import { useEffect, useRef, useState } from "react";
import { Field, Input, Select } from "@/components/ui";
import { request } from "@/api/client";
import { useSession } from "@/store/session";
import { useI18n } from "@/i18n";

export const defaultOpeningPolicy: API.TCPOpeningPolicy = {
  mode: "continuous",
  duration_seconds: 10,
};

export function TCPOpeningFields({
  value,
  onChange,
}: {
  value: API.TCPOpeningPolicy;
  onChange: (value: API.TCPOpeningPolicy) => void;
}) {
  const { tr } = useI18n();
  return (
    <div className="is-full" style={{ display: "grid", gap: 16 }}>
      <Field label={tr("端口开放方式", "Port availability")}>
        <Select
          value={value.mode}
          onChange={(e) =>
            onChange({
              ...value,
              mode: e.target.value as API.TCPOpeningPolicy["mode"],
            })
          }
        >
          <option value="continuous">{tr("持续开放", "Always open")}</option>
          <option value="on_demand">{tr("按需开放", "On demand")}</option>
        </Select>
      </Field>
      {value.mode === "on_demand" && (
        <>
          <Field
            label={tr("每次开放时长（秒）", "Opening duration (seconds)")}
            hint={tr("5～300 秒，默认 10 秒。", "5–300 seconds; default 10.")}
          >
            <Input
              type="number"
              min={5}
              max={300}
              step={1}
              required
              value={value.duration_seconds || ""}
              onChange={(e) =>
                onChange({ ...value, duration_seconds: Number(e.target.value) })
              }
            />
          </Field>
          <p className="liaison-access-route-note">
            {tr(
              "使用前点击开放端口，到期拒绝新连接，已有连接不受影响。开放期间仍遵循防火墙规则，不绑定当前来源 IP；目标服务仍需启用认证。",
              "Open the port before connecting. Expiry blocks new connections without interrupting existing ones. Firewall rules still apply; the window is not bound to your source IP. Keep authentication enabled on the target service."
            )}
          </p>
        </>
      )}
    </div>
  );
}

type Opening = {
  request_id: string;
  state: "open" | "closed";
  server_time: string;
  expires_at?: string;
};
export function remainingOpeningMs(result: Opening, elapsed: number) {
  if (result.state === "closed") return 0;
  const duration =
    Date.parse(result.expires_at || "") - Date.parse(result.server_time);
  if (!Number.isFinite(duration) || duration < 0 || duration > 300000)
    throw Error("Invalid opening receipt");
  return Math.max(0, duration - elapsed);
}

export function TCPOpeningAction({ row }: { row: API.Proxy }) {
  const userID = useSession((s) => s.initialState.currentUser?.id);
  return userID ? (
    <OpeningAction key={userID} row={row} userID={userID} />
  ) : null;
}

function OpeningAction({ row, userID }: { row: API.Proxy; userID: number }) {
  const { tr } = useI18n();
  const key = `liaison.tcp-opening.${userID}.${row.id}`;
  const [requestID, setRequestID] = useState("");
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const [deadline, setDeadline] = useState(0);
  const [remaining, setRemaining] = useState(0);
  const alive = useRef(true),
    pending = useRef(false);
  const enabled =
    row.status === "running" &&
    (!row.effective_status || row.effective_status === "active") &&
    !!userID;
  const persist = (id: string) => {
    setRequestID(id);
    try {
      if (id) sessionStorage.setItem(key, id);
      else sessionStorage.removeItem(key);
    } catch {
      /* In-memory recovery still works. */
    }
  };
  const perform = async (method: "POST" | "GET" | "DELETE", id: string) => {
    if (pending.current || !enabled) return;
    pending.current = true;
    setBusy(true);
    setUncertain(false);
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 20000);
    const started = performance.now();
    try {
      const response = await request<API.Response<Opening>>(
        `/api/v1/proxies/${row.id}/tcp_openings${
          method === "POST" ? "" : "/" + encodeURIComponent(id)
        }`,
        {
          method,
          data: method === "POST" ? { request_id: id } : undefined,
          signal: controller.signal,
        }
      );
      if (!alive.current) return;
      if (
        response.code !== 200 ||
        !response.data ||
        response.data.request_id !== id ||
        !["open", "closed"].includes(response.data.state)
      )
        throw Error("Invalid receipt");
      const result = response.data;
      const ms = remainingOpeningMs(result, performance.now() - started);
      // Only a server-confirmed closed receipt clears the ID. A slow request
      // can exhaust the local estimate while the server window is still open.
      if (result.state === "closed") {
        persist("");
        setDeadline(0);
        setRemaining(0);
      } else {
        setDeadline(performance.now() + ms);
        setRemaining(Math.ceil(ms / 1000));
      }
    } catch {
      if (alive.current) {
        setUncertain(true);
        setDeadline(0);
      }
    } finally {
      window.clearTimeout(timeout);
      pending.current = false;
      if (alive.current) setBusy(false);
    }
  };
  useEffect(() => {
    alive.current = true;
    let saved = "";
    try {
      saved = sessionStorage.getItem(key) || "";
    } catch {
      /* Optional recovery storage. */
    }
    if (saved && enabled) {
      setRequestID(saved);
      void perform("GET", saved);
    }
    return () => {
      alive.current = false;
    };
  }, []);
  useEffect(() => {
    if (!deadline || !requestID || uncertain || !enabled) return;
    const timer = window.setInterval(() => {
      const ms = Math.max(0, deadline - performance.now());
      setRemaining(Math.ceil(ms / 1000));
      if (ms === 0) void perform("GET", requestID);
    }, 1000);
    return () => window.clearInterval(timer);
  }, [deadline, requestID, uncertain, enabled]);
  const open = () => {
    if (pending.current) return;
    const bytes = crypto.getRandomValues(new Uint8Array(16));
    const id = Array.from(bytes, (byte) =>
      byte.toString(16).padStart(2, "0")
    ).join("");
    persist(id);
    void perform("POST", id);
  };
  return (
    <span className="liaison-tcp-opening-action">
      <button
        type="button"
        className="liaison-table-link"
        disabled={!enabled || busy || (!uncertain && remaining > 0)}
        aria-busy={busy}
        title={tr("到期后已有连接保持，新连接需再次开放。", "Existing connections survive expiry; new connections require another opening.")}
        onClick={() => requestID ? void perform("GET", requestID) : open()}
      >
        {busy ? tr("处理中…", "Working…")
          : uncertain ? tr("重试", "Retry")
          : remaining > 0 ? tr(`已开放 · ${remaining}s`, `Open · ${remaining}s`)
          : requestID ? tr("查询状态", "Check status")
          : tr("临时开放", "Open temporarily")}
      </button>
      {uncertain && <small role="alert">
        {tr("暂时无法确认开放状态，请重试。", "Unable to confirm opening status. Please retry.")}
      </small>}
    </span>
  );
}
