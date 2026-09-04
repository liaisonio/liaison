import SessionWatermark, {
  buildSessionWatermarkLabel,
  useSessionWatermarkTime,
} from '@/components/SessionWatermark';
import { useI18n } from '@/i18n';
import { history, useLocation, useModel, useParams, useSearchParams } from '@/lib/runtime';
import {
  createWebDesktopSession,
  deleteWebDesktopCredential,
  getWebDesktopTarget,
} from '@/services/api';
import { Button, Field, Input, Modal, Notice } from '@/components/ui';
import Guacamole from 'guacamole-common-js';
import {
  ArrowLeft,
  Check,
  Clock3,
  Fullscreen,
  LogIn,
  Minimize,
  Monitor,
  Plus,
  PlugZap,
  Send,
  Trash2,
} from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import './index.less';

const credentialKey = (username?: string, domain?: string) =>
  `${domain || ''}\\${username || ''}`;

const formatWebDesktopTime = (value?: string) => {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value.replace('T', ' ').replace(/Z$/, '');
  }
  const part = (item: number) => String(item).padStart(2, '0');
  return (
    date.getFullYear() +
    '-' +
    part(date.getMonth() + 1) +
    '-' +
    part(date.getDate()) +
    ' ' +
    part(date.getHours()) +
    ':' +
    part(date.getMinutes()) +
    ':' +
    part(date.getSeconds())
  );
};

const WebDesktopPage: React.FC = () => {
  const { tr } = useI18n();
  const { initialState } = useModel('@@initialState');
  const params = useParams();
  const location = useLocation();
  const [routeSearch] = useSearchParams();
  const proxyId = Number(params.proxyId);
  const credentialId = Number(params.credentialId || 0);
  const isTemporarySession = location.pathname.endsWith('/session');
  const isSessionView = isTemporarySession || credentialId > 0;
  const [credentials, setCredentials] =
    useState<API.CreateWebDesktopSessionRequest>({
      username: '',
      domain: '',
      password: '',
      save_credential: false,
    });
  const [target, setTarget] = useState<API.WebDesktopTarget>();
  const [loading, setLoading] = useState(true);
  const [connecting, setConnecting] = useState(false);
  const [connected, setConnected] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [credentialOpen, setCredentialOpen] = useState(false);
  const [pendingSessionCredentials, setPendingSessionCredentials] =
    useState<API.CreateWebDesktopSessionRequest>();
  const [sessionDurationSeconds, setSessionDurationSeconds] = useState<
    number | null
  >(null);
  const [error, setError] = useState('');
  const displayHostRef = useRef<HTMLDivElement | null>(null);
  const displayContentRef = useRef<HTMLDivElement | null>(null);
  const clientRef = useRef<any>();
  const tunnelRef = useRef<any>();
  const keyboardRef = useRef<any>();
  const mouseRef = useRef<any>();
  const initialCredentialPromptRef = useRef(false);
  const initialSavedConnectRef = useRef<number>();
  const sessionStartedAtRef = useRef<number>();

  const active = target?.effective_status === 'active';
  const protocol = target?.protocol || 'rdp';
  const savedCredentials = target?.credentials || [];
  const selectedSavedCredential = credentialId > 0 && savedCredentials.some(
    (item) =>
      credentialKey(item.username, item.domain) ===
      credentialKey(credentials.username, credentials.domain),
  );
  const savedOptions = savedCredentials.map((item) => {
    const key = credentialKey(item.username, item.domain);
    const label =
      protocol === 'vnc'
        ? tr('已保存的 VNC 密码', 'Saved VNC password')
        : item.domain
        ? `${item.domain}\\${item.username}`
        : item.username || '';
    return { label, value: key };
  });
  const requestedReturnPath = routeSearch.get('from') || '';
  const accessReturnPath =
    requestedReturnPath.startsWith('/proxy') &&
    !requestedReturnPath.startsWith('//')
      ? requestedReturnPath
      : '/proxy?access_type=' + (protocol === 'vnc' ? 'webvnc' : 'webrdp');
  const connectionListPath =
    '/webdesktop/' +
    proxyId +
    (requestedReturnPath
      ? '?from=' + encodeURIComponent(accessReturnPath)
      : '');
  const temporarySessionPath =
    '/webdesktop/' +
    proxyId +
    '/session' +
    (requestedReturnPath
      ? '?from=' + encodeURIComponent(accessReturnPath)
      : '');

  const watermarkTime = useSessionWatermarkTime();
  const watermarkUser =
    initialState?.currentUser?.email ||
    initialState?.currentUser?.name ||
    tr('未知用户', 'Unknown user');
  const watermarkLines = target
    ? [
        buildSessionWatermarkLabel([
          watermarkUser,
          target.protocol.toUpperCase(),
          target.proxy_name,
        ]),
        watermarkTime,
      ]
    : [
        buildSessionWatermarkLabel([watermarkUser, 'WebDesktop']),
        watermarkTime,
      ];

  const formatSessionDuration = useCallback(
    (seconds: number) => {
      const hours = Math.floor(seconds / 3600);
      const minutes = Math.floor((seconds % 3600) / 60);
      const remainingSeconds = seconds % 60;
      if (hours > 0) {
        return tr(
          hours + ' 小时 ' + minutes + ' 分钟',
          hours + 'h ' + minutes + 'm',
        );
      }
      if (minutes > 0) {
        return tr(
          minutes + ' 分钟 ' + remainingSeconds + ' 秒',
          minutes + 'm ' + remainingSeconds + 's',
        );
      }
      return tr(remainingSeconds + ' 秒', remainingSeconds + 's');
    },
    [tr],
  );

  const recordSessionEnd = useCallback(() => {
    if (sessionStartedAtRef.current === undefined) return;
    setSessionDurationSeconds(
      Math.max(1, Math.round((Date.now() - sessionStartedAtRef.current) / 1000)),
    );
    sessionStartedAtRef.current = undefined;
  }, []);

  const returnToAccess = useCallback(() => {
    history.push(accessReturnPath);
  }, [accessReturnPath]);

  const returnToConnections = useCallback(() => {
    history.push(connectionListPath);
  }, [connectionListPath]);

  const openNewConnection = useCallback(() => {
    initialCredentialPromptRef.current = false;
    setError('');
    setCredentials({
      username: '',
      domain: '',
      password: '',
      save_credential: false,
    });
    setCredentialOpen(true);
  }, []);

  const closeCredentialModal = useCallback(() => {
    setError('');
    setCredentialOpen(false);
    if (isSessionView) returnToConnections();
  }, [isSessionView, returnToConnections]);

  const openSavedConnection = useCallback(
    (credential: API.WebDesktopCredential) => {
      if (!credential.id) return;
      initialSavedConnectRef.current = undefined;
      history.push(
        '/webdesktop/' +
          proxyId +
          '/connections/' +
          credential.id +
          (requestedReturnPath
            ? '?from=' + encodeURIComponent(accessReturnPath)
            : ''),
      );
    },
    [accessReturnPath, proxyId, requestedReturnPath],
  );

  const cleanupConnection = useCallback(
    (close = false, clearDisplay = false) => {
      const keyboard = keyboardRef.current;
      const mouse = mouseRef.current;
      const client = clientRef.current;
      const tunnel = tunnelRef.current;
      keyboard?.reset?.();
      if (keyboard) {
        keyboard.onkeydown = null;
        keyboard.onkeyup = null;
      }
      mouse?.cleanup?.();
      keyboardRef.current = undefined;
      mouseRef.current = undefined;
      clientRef.current = undefined;
      tunnelRef.current = undefined;
      if (close) {
        client?.disconnect?.();
        tunnel?.disconnect?.();
      }
      if (clearDisplay && displayContentRef.current) {
        displayContentRef.current.innerHTML = '';
      }
      setConnected(false);
      setConnecting(false);
    },
    [],
  );

  const disconnect = useCallback(() => {
    recordSessionEnd();
    cleanupConnection(true, true);
  }, [cleanupConnection, recordSessionEnd]);

  const disconnectAndSummarize = useCallback(() => {
    disconnect();
  }, [disconnect]);

  const focusRemoteCanvas = useCallback(() => {
    const canvas = displayContentRef.current?.querySelector<HTMLCanvasElement>(
      '.webdesktop-input-plane',
    );
    canvas?.focus({ preventScroll: true });
  }, []);

  const toggleFullscreen = useCallback(async () => {
    const host = displayHostRef.current;
    if (!host || !connected) return;
    try {
      if (document.fullscreenElement === host) {
        await document.exitFullscreen();
        return;
      }
      await host.requestFullscreen();
      window.setTimeout(focusRemoteCanvas, 0);
    } catch (e: any) {
      setError(e?.message || tr('无法进入全屏', 'Unable to enter fullscreen'));
    }
  }, [connected, focusRemoteCanvas, tr]);

  const loadTarget = useCallback(async () => {
    if (!proxyId) {
      setError(tr('访问 ID 无效', 'Invalid entry ID'));
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const res = await getWebDesktopTarget(proxyId);
      if (res.code === 200 && res.data) {
        setTarget(res.data);
        const savedCredentials = res.data.credentials || [];
        if (credentialId > 0) {
          const item = savedCredentials.find(
            (credential) => credential.id === credentialId,
          );
          if (!item) {
            setError(
              tr(
                '保存的远程桌面连接不存在或已删除',
                'Saved remote desktop connection no longer exists',
              ),
            );
            return;
          }
          setCredentials({
            username: item.username || '',
            domain: item.domain || '',
            password: '',
            save_credential: false,
          });
        } else if (!isTemporarySession) {
          setCredentials({
            username: '',
            domain: '',
            password: '',
            save_credential: false,
          });
        }
        setError('');
      } else {
        setError(
          res.message ||
            tr('获取远程桌面目标失败', 'Failed to load remote desktop target'),
        );
      }
    } catch (e: any) {
      setError(
        e?.response?.data?.message ||
          tr('获取远程桌面目标失败', 'Failed to load remote desktop target'),
      );
    } finally {
      setLoading(false);
    }
  }, [credentialId, isTemporarySession, proxyId, tr]);

  useEffect(() => {
    loadTarget();
  }, [loadTarget]);

  useEffect(() => {
    if (!isSessionView) return;
    document.body.classList.add('webdesktop-page-active');
    const footerElements = Array.from(
      document.querySelectorAll<HTMLElement>(
        '.global-footer',
      ),
    );
    const previousFooterStyles = footerElements.map((element) => ({
      element,
      display: element.style.display,
      pointerEvents: element.style.pointerEvents,
    }));
    footerElements.forEach((element) => {
      element.style.display = 'none';
      element.style.pointerEvents = 'none';
    });
    return () => {
      document.body.classList.remove('webdesktop-page-active');
      previousFooterStyles.forEach(({ element, display, pointerEvents }) => {
        element.style.display = display;
        element.style.pointerEvents = pointerEvents;
      });
    };
  }, [isSessionView]);

  useEffect(() => {
    if (!isSessionView) return;
    return () => {
      disconnect();
    };
  }, [disconnect, isSessionView]);

  useEffect(() => {
    const handleFullscreenChange = () => {
      const active = document.fullscreenElement === displayHostRef.current;
      setFullscreen(active);
      if (active) {
        window.setTimeout(focusRemoteCanvas, 0);
      }
    };
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
    };
  }, [focusRemoteCanvas]);

  const applySelectedCredential = (value: string) => {
    const item = savedCredentials.find(
      (candidate) =>
        credentialKey(candidate.username, candidate.domain) === value,
    );
    if (!item) return;
    setCredentials({
      username: item.username || '',
      domain: item.domain || '',
      password: '',
      save_credential: false,
    });
  };

  const connect = async (values: API.CreateWebDesktopSessionRequest) => {
    if (
      !target ||
      !active ||
      !displayHostRef.current ||
      !displayContentRef.current
    )
      return;
    disconnect();
    setSessionDurationSeconds(null);
    setConnecting(true);
    setError('');
    const width = Math.max(displayHostRef.current.clientWidth || 0, 1024);
    const height = Math.max(displayHostRef.current.clientHeight || 0, 640);
    try {
      const password = values.password || '';
      const shouldSaveCredential = Boolean(values.save_credential && password);
      const res = await createWebDesktopSession(proxyId, {
        username: values.username?.trim(),
        domain: values.domain?.trim(),
        password,
        save_credential: shouldSaveCredential,
        width,
        height,
        dpi: 96,
      });
      setCredentials((value) => ({ ...value, password: '' }));
      if (res.code !== 200 || !res.data?.ws_url) {
        throw new Error(
          res.message ||
            tr(
              '创建 WebDesktop 会话失败',
              'Failed to create WebDesktop session',
            ),
        );
      }

      const tunnel = new Guacamole.WebSocketTunnel(res.data.ws_url);
      const client = new Guacamole.Client(tunnel);
      tunnelRef.current = tunnel;
      clientRef.current = client;
      const display = client.getDisplay();
      const displayElement = display.getElement() as HTMLElement;
      const inputPlane = document.createElement('canvas');
      inputPlane.className = 'webdesktop-input-plane';
      inputPlane.tabIndex = 0;
      displayElement.classList.add('webdesktop-native-display');
      displayContentRef.current.innerHTML = '';
      displayContentRef.current.appendChild(displayElement);
      displayContentRef.current.appendChild(inputPlane);

      const fitDisplayToHost = () => {
        const host = displayHostRef.current;
        if (!host) return;
        const remoteWidth = Math.max(
          display.getWidth?.() || inputPlane.width || width,
          1,
        );
        const remoteHeight = Math.max(
          display.getHeight?.() || inputPlane.height || height,
          1,
        );
        const availableWidth = Math.max(host.clientWidth, 1);
        const availableHeight = Math.max(host.clientHeight, 1);
        const scale = Math.min(
          availableWidth / remoteWidth,
          availableHeight / remoteHeight,
        );
        display.scale(scale);
        if (inputPlane.width !== remoteWidth) inputPlane.width = remoteWidth;
        if (inputPlane.height !== remoteHeight)
          inputPlane.height = remoteHeight;
        inputPlane.style.width = `${Math.floor(remoteWidth * scale)}px`;
        inputPlane.style.height = `${Math.floor(remoteHeight * scale)}px`;
      };
      let fitPending = false;
      const scheduleDisplayFit = () => {
        if (fitPending) return;
        fitPending = true;
        window.requestAnimationFrame(() => {
          fitPending = false;
          fitDisplayToHost();
        });
      };
      const resizeObserver =
        typeof ResizeObserver !== 'undefined'
          ? new ResizeObserver(scheduleDisplayFit)
          : undefined;
      resizeObserver?.observe(displayHostRef.current);
      window.addEventListener('resize', scheduleDisplayFit);
      fitDisplayToHost();
      display.onresize = (width: number, height: number) => {
        if (inputPlane.width !== width) inputPlane.width = width;
        if (inputPlane.height !== height) inputPlane.height = height;
        scheduleDisplayFit();
      };

      tunnel.onerror = (status: any) => {
        const text =
          status?.message ||
          tr('WebDesktop 连接异常', 'WebDesktop connection error');
        setError(text);
        recordSessionEnd();
        cleanupConnection(true);
        setCredentialOpen(true);
      };
      tunnel.onstatechange = (state: number) => {
        if (state === Guacamole.Tunnel.State.OPEN) {
          sessionStartedAtRef.current = Date.now();
          setSessionDurationSeconds(null);
          setConnecting(false);
          setConnected(true);
          setCredentialOpen(false);
          inputPlane.focus({ preventScroll: true });
          if (shouldSaveCredential) {
            window.setTimeout(loadTarget, 1000);
          }
        }
        if (state === Guacamole.Tunnel.State.CLOSED) {
          recordSessionEnd();
          cleanupConnection(false);
        }
      };
      client.onerror = (status: any) => {
        const text =
          status?.message ||
          tr('远程桌面连接失败', 'Remote desktop connection failed');
        setError(text);
        recordSessionEnd();
        cleanupConnection(true);
        setCredentialOpen(true);
      };

      const mouseState = {
        x: 0,
        y: 0,
        left: false,
        middle: false,
        right: false,
        up: false,
        down: false,
      };
      const sendMouseStateNow = () => {
        if (clientRef.current !== client) return;
        client.sendMouseState({ ...mouseState });
      };
      const updateMousePosition = (event: MouseEvent | WheelEvent) => {
        const rect = inputPlane.getBoundingClientRect();
        const visibleWidth = rect.width || inputPlane.width || 1;
        const visibleHeight = rect.height || inputPlane.height || 1;
        const x =
          ((event.clientX - rect.left) * inputPlane.width) / visibleWidth;
        const y =
          ((event.clientY - rect.top) * inputPlane.height) / visibleHeight;
        mouseState.x = Math.max(
          0,
          Math.min(inputPlane.width - 1, Math.round(x)),
        );
        mouseState.y = Math.max(
          0,
          Math.min(inputPlane.height - 1, Math.round(y)),
        );
      };
      const updateButton = (event: MouseEvent, pressed: boolean) => {
        if (event.button === 0) mouseState.left = pressed;
        if (event.button === 1) mouseState.middle = pressed;
        if (event.button === 2) mouseState.right = pressed;
      };
      const handleMouseMove = (event: MouseEvent) => {
        event.preventDefault();
        updateMousePosition(event);
        sendMouseStateNow();
      };
      const handlePointerDown = (event: PointerEvent) => {
        inputPlane.setPointerCapture?.(event.pointerId);
      };
      const handlePointerUp = (event: PointerEvent) => {
        inputPlane.releasePointerCapture?.(event.pointerId);
      };
      const handleMouseDown = (event: MouseEvent) => {
        event.preventDefault();
        inputPlane.focus({ preventScroll: true });
        updateMousePosition(event);
        updateButton(event, true);
        sendMouseStateNow();
      };
      const handleMouseUp = (event: MouseEvent) => {
        event.preventDefault();
        updateMousePosition(event);
        updateButton(event, false);
        sendMouseStateNow();
      };
      const handleMouseLeave = () => {
        mouseState.left = false;
        mouseState.middle = false;
        mouseState.right = false;
        mouseState.up = false;
        mouseState.down = false;
        sendMouseStateNow();
      };
      const handleWheel = (event: WheelEvent) => {
        event.preventDefault();
        updateMousePosition(event);
        mouseState.up = event.deltaY < 0;
        mouseState.down = event.deltaY > 0;
        sendMouseStateNow();
        mouseState.up = false;
        mouseState.down = false;
        sendMouseStateNow();
      };
      const handleContextMenu = (event: MouseEvent) => {
        event.preventDefault();
      };
      const handleClick = () => inputPlane.focus({ preventScroll: true });
      const mouseMoveEvent =
        'onpointerrawupdate' in window
          ? 'pointerrawupdate'
          : 'onpointermove' in window
          ? 'pointermove'
          : 'mousemove';
      inputPlane.addEventListener(
        mouseMoveEvent,
        handleMouseMove as EventListener,
      );
      inputPlane.addEventListener('pointerdown', handlePointerDown);
      inputPlane.addEventListener('pointerup', handlePointerUp);
      inputPlane.addEventListener('pointercancel', handlePointerUp);
      inputPlane.addEventListener('mousedown', handleMouseDown);
      inputPlane.addEventListener('mouseup', handleMouseUp);
      inputPlane.addEventListener('mouseleave', handleMouseLeave);
      inputPlane.addEventListener('wheel', handleWheel, { passive: false });
      inputPlane.addEventListener('contextmenu', handleContextMenu);
      inputPlane.addEventListener('click', handleClick);
      display.oncursor = (canvas: HTMLCanvasElement, x: number, y: number) => {
        inputPlane.style.cursor = `url(${canvas.toDataURL(
          'image/png',
        )}) ${x} ${y}, auto`;
        display.showCursor(false);
      };
      display.showCursor(false);
      mouseRef.current = {
        cleanup: () => {
          resizeObserver?.disconnect();
          window.removeEventListener('resize', scheduleDisplayFit);
          inputPlane.removeEventListener(
            mouseMoveEvent,
            handleMouseMove as EventListener,
          );
          inputPlane.removeEventListener('pointerdown', handlePointerDown);
          inputPlane.removeEventListener('pointerup', handlePointerUp);
          inputPlane.removeEventListener('pointercancel', handlePointerUp);
          inputPlane.removeEventListener('mousedown', handleMouseDown);
          inputPlane.removeEventListener('mouseup', handleMouseUp);
          inputPlane.removeEventListener('mouseleave', handleMouseLeave);
          inputPlane.removeEventListener('wheel', handleWheel);
          inputPlane.removeEventListener('contextmenu', handleContextMenu);
          inputPlane.removeEventListener('click', handleClick);
        },
      };

      const keyboard = new Guacamole.Keyboard(inputPlane);
      keyboard.onkeydown = (keysym: number) => {
        client.sendKeyEvent(1, keysym);
      };
      keyboard.onkeyup = (keysym: number) => {
        client.sendKeyEvent(0, keysym);
      };
      keyboardRef.current = keyboard;

      client.connect('');
    } catch (e: any) {
      const text =
        e?.response?.data?.message ||
        e?.message ||
        tr('创建 WebDesktop 会话失败', 'Failed to create WebDesktop session');
      setError(text);
      recordSessionEnd();
      setConnecting(false);
      setConnected(false);
      setCredentialOpen(true);
    }
  };

  useEffect(() => {
    if (
      !pendingSessionCredentials ||
      !isTemporarySession ||
      loading ||
      !active ||
      !displayHostRef.current
    ) {
      return;
    }
    const values = pendingSessionCredentials;
    setPendingSessionCredentials(undefined);
    void connect(values);
  }, [
    active,
    isTemporarySession,
    loading,
    pendingSessionCredentials,
  ]);

  useEffect(() => {
    if (
      !isSessionView ||
      isTemporarySession ||
      credentialId <= 0 ||
      loading ||
      !active ||
      connected ||
      connecting ||
      initialSavedConnectRef.current === credentialId
    ) {
      return;
    }
    const saved = target?.credentials?.find(
      (credential) => credential.id === credentialId,
    );
    if (!saved) return;
    initialSavedConnectRef.current = credentialId;
    void connect({
      username: saved.username || '',
      domain: saved.domain || '',
      password: '',
      save_credential: false,
    });
  }, [
    active,
    connected,
    connecting,
    credentialId,
    isSessionView,
    isTemporarySession,
    loading,
    target,
  ]);

  const submitCredentialConnection = () => {
    const username = credentials.username?.trim() || '';
    if (protocol === 'rdp' && !username) {
      setError(tr('请输入用户名', 'Username is required'));
      return;
    }
    if (!selectedSavedCredential && !credentials.password) {
      setError(tr('请输入密码', 'Password is required'));
      return;
    }
    const values = {
      ...credentials,
      username,
      domain: credentials.domain?.trim() || '',
    };
    if (!isSessionView) {
      initialCredentialPromptRef.current = true;
      setCredentialOpen(false);
      setPendingSessionCredentials(values);
      history.push(temporarySessionPath);
      return;
    }
    void connect(values);
  };

  const deleteSavedConnection = async (
    credential: API.WebDesktopCredential,
  ) => {
    await deleteWebDesktopCredential(proxyId, {
      protocol,
      username: credential.username || '',
      domain: credential.domain || '',
    });
    await loadTarget();
  };

  const clearCredential = async () => {
    try {
      await deleteWebDesktopCredential(proxyId, {
        protocol,
        username: String(credentials.username || '').trim(),
        domain: String(credentials.domain || '').trim(),
      });
      setCredentials((value) => ({
        ...value,
        password: '',
        save_credential: false,
      }));
      loadTarget();
    } catch (e: any) {
      setError(
        e?.response?.data?.message ||
          tr('清除保存密码失败', 'Failed to clear saved password'),
      );
    }
  };

  const protocolLabel = protocol === 'vnc' ? 'VNC' : 'RDP';
  const connectionModal = (
    <Modal
      title={tr(
        protocol === 'vnc' ? '新建 VNC 连接' : '新建 RDP 连接',
        protocol === 'vnc' ? 'New VNC connection' : 'New RDP connection',
      )}
      open={credentialOpen}
      width={400}
      className="webdesktop-credential-modal"
      closeOnMask={!connecting}
      onClose={closeCredentialModal}
    >
      {error && <Notice tone="danger">{error}</Notice>}
      <form
        className="webdesktop-native-form"
        onSubmit={(event) => {
          event.preventDefault();
          submitCredentialConnection();
        }}
      >
        {protocol === 'rdp' && (
          <>
            <Field label={tr('域', 'Domain')}>
              <Input
                value={credentials.domain || ''}
                placeholder={tr('可选', 'Optional')}
                onChange={(event) =>
                  setCredentials((value) => ({
                    ...value,
                    domain: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label={tr('用户名', 'Username')} required>
              <Input
                autoFocus
                autoComplete="username"
                value={credentials.username || ''}
                placeholder={tr('输入 RDP 用户名', 'Enter RDP username')}
                onChange={(event) =>
                  setCredentials((value) => ({
                    ...value,
                    username: event.target.value,
                  }))
                }
              />
            </Field>
          </>
        )}
        <Field label={tr('密码', 'Password')} required>
          <Input
            type="password"
            autoFocus={protocol === 'vnc'}
            autoComplete="new-password"
            value={credentials.password || ''}
            placeholder={tr(
              protocol === 'vnc' ? '输入 VNC 密码' : '输入 RDP 密码',
              protocol === 'vnc' ? 'Enter VNC password' : 'Enter RDP password',
            )}
            onChange={(event) =>
              setCredentials((value) => ({
                ...value,
                password: event.target.value,
              }))
            }
          />
        </Field>
        <div className="webdesktop-credential-options">
          <label className="liaison-checkbox">
            <input
              type="checkbox"
              checked={Boolean(credentials.save_credential)}
              onChange={(event) =>
                setCredentials((value) => ({
                  ...value,
                  save_credential: event.target.checked,
                }))
              }
            />
            <span>{tr('保存密码', 'Save password')}</span>
          </label>
        </div>
        <div className="webdesktop-credential-actions">
          <Button onClick={closeCredentialModal} disabled={connecting}>
            {tr('取消', 'Cancel')}
          </Button>
          <Button
            variant="primary"
            type="submit"
            disabled={!active || loading || connecting}
          >
            <Send size={14} />
            {tr('连接', 'Connect')}
          </Button>
        </div>
      </form>
    </Modal>
  );

  if (!isSessionView) {
    return (
      <section className="webdesktop-connections-page">
        <div className="webdesktop-connections-header">
          <button
            type="button"
            className="webdesktop-connections-back"
            onClick={returnToAccess}
          >
            <ArrowLeft size={14} />
            {tr('返回访问', 'Back to access')}
          </button>
          <nav
            className="webdesktop-connections-breadcrumb"
            aria-label={tr('页面层级', 'Breadcrumb')}
          >
            <span>{tr('访问', 'Access')}</span>
            <i>/</i>
            <span>{protocol === 'vnc' ? 'Web VNC' : 'Web RDP'}</span>
            <i>/</i>
            <strong>{target?.proxy_name || tr('连接', 'Connections')}</strong>
          </nav>
          {target && (
            <span className="webdesktop-connections-target">
              {target.target_host}:{target.target_port}
            </span>
          )}
        </div>

        <div className="webdesktop-connections-heading">
          <div>
            <h2>
              {tr(
                protocol === 'vnc' ? 'VNC 连接' : 'RDP 连接',
                protocol === 'vnc' ? 'VNC connections' : 'RDP connections',
              )}
            </h2>
            <p>
              {savedCredentials.length
                ? tr(
                    '选择已保存连接可直接进入远程桌面，也可以新建临时连接。',
                    'Open a saved connection directly, or create a temporary one.',
                  )
                : tr(
                    '还没有保存的连接，可以新建一个远程桌面连接。',
                    'No saved connection yet. Create a remote desktop connection.',
                  )}
            </p>
          </div>
          <Button
            variant="primary"
            disabled={!active || loading}
            onClick={openNewConnection}
          >
            <Plus size={15} />
            {tr('新建连接', 'New connection')}
          </Button>
        </div>

        {loading ? (
          <div className="webdesktop-connections-loading">
            <span className="webdesktop-native-spinner" />
          </div>
        ) : !active ? (
          <Notice tone="warning">
            {target?.effective_status_message ||
              tr('当前远程桌面访问不可用', 'Remote desktop entry unavailable')}
          </Notice>
        ) : savedCredentials.length ? (
          <div className="webdesktop-connection-list">
            {savedCredentials.map((credential) => (
              <article
                className="webdesktop-connection-card"
                key={credential.id}
              >
                <div className="webdesktop-connection-card-identity">
                  <span className="webdesktop-screen-mark" aria-hidden="true">
                    {protocolLabel}
                  </span>
                  <strong>
                    {target?.proxy_name ||
                      tr('远程桌面连接', 'Remote desktop connection')}
                  </strong>
                </div>
                <div className="webdesktop-connection-card-meta">
                  <span className="webdesktop-connection-protocol">
                    {protocolLabel}
                  </span>
                  <span>
                    {tr('应用：', 'Application:')}
                    {target?.application_name || '-'}
                  </span>
                  <span>
                    {tr('目标：', 'Target:')}
                    {target
                      ? target.target_host + ':' + target.target_port
                      : '-'}
                  </span>
                  {protocol === 'rdp' && (
                    <span>
                      {tr('用户：', 'User:')}
                      {credential.domain
                        ? credential.domain + '\\' + credential.username
                        : credential.username || '-'}
                    </span>
                  )}
                  <span className="webdesktop-connection-password-state">
                    <Check size={12} />
                    {tr('密码已保存', 'Password saved')}
                  </span>
                </div>
                <div className="webdesktop-connection-card-used">
                  <span>{tr('最近使用：', 'Last used:')}</span>
                  <time>
                    <Clock3 size={13} />
                    {credential.last_used_at
                      ? formatWebDesktopTime(credential.last_used_at)
                      : tr('尚未使用', 'Never')}
                  </time>
                </div>
                <div className="webdesktop-connection-card-actions">
                  <Button
                    variant="primary"
                    disabled={!active}
                    onClick={() => openSavedConnection(credential)}
                  >
                    <LogIn size={14} />
                    {tr('进入', 'Enter')}
                  </Button>
                  <Button
                    variant="ghost"
                    aria-label={tr('删除连接', 'Delete connection')}
                    onClick={() => {
                      if (
                        !window.confirm(
                          tr('删除这个保存连接？', 'Delete this saved connection?'),
                        )
                      )
                        return;
                      void deleteSavedConnection(credential);
                    }}
                  >
                    <Trash2 size={14} />
                  </Button>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <div className="webdesktop-connections-empty">
            <Monitor size={28} />
            <strong>
              {tr(
                protocol === 'vnc'
                  ? '还没有保存的 VNC 连接'
                  : '还没有保存的 RDP 连接',
                protocol === 'vnc'
                  ? 'No saved VNC connections'
                  : 'No saved RDP connections',
              )}
            </strong>
            <p>
              {tr(
                '新建连接时可以选择保存密码，之后直接进入远程桌面。',
                'Save the password when connecting to enter the desktop directly next time.',
              )}
            </p>
            <Button variant="primary" onClick={openNewConnection}>
              <Plus size={15} />
              {tr('新建连接', 'New connection')}
            </Button>
          </div>
        )}
        {connectionModal}
      </section>
    );
  }

  return (
    <>
      <div
        className={
          'webdesktop-shell' +
          (credentialOpen && !connected ? ' is-credential-setup' : '')
        }
      >
        <header className="webdesktop-toolbar">
          <div className="webdesktop-identity">
            <Button
              className="webdesktop-back-button"
              variant="ghost"
              aria-label={tr('返回连接', 'Back to connections')}
              onClick={returnToConnections}
            >
              <ArrowLeft size={16} />
            </Button>
            <span className="webdesktop-screen-mark" aria-hidden="true">
              {protocol.toUpperCase()}
            </span>
            <div>
              <strong>
                {target?.proxy_name ||
                  tr('远程桌面会话', 'Remote desktop session')}
              </strong>
              <span>
                {protocol === 'vnc'
                  ? tr('VNC 远程桌面', 'VNC remote desktop')
                  : tr('RDP 远程桌面', 'RDP remote desktop')}
              </span>
            </div>
          </div>

          <div className="webdesktop-session-meta">
            <div>
              <span>{tr('应用', 'Application')}</span>
              <strong>{target?.application_name || '-'}</strong>
            </div>
            <div>
              <span>{tr('目标', 'Target')}</span>
              <strong>
                {target ? `${target.target_host}:${target.target_port}` : '-'}
              </strong>
            </div>
            <div>
              <span>{tr('状态', 'Status')}</span>
              <strong
                className={
                  'webdesktop-status ' +
                  (connected
                    ? 'is-connected'
                    : connecting
                    ? 'is-connecting'
                    : active
                    ? 'is-ready'
                    : 'is-error')
                }
              >
                <i />
                {connected
                  ? tr('已连接', 'Connected')
                  : connecting
                  ? tr('连接中', 'Connecting')
                  : active
                  ? tr('待连接', 'Ready')
                  : tr('不可用', 'Unavailable')}
              </strong>
            </div>
          </div>

          <div className="webdesktop-toolbar-actions">
            {!connected && (
              <Button
                variant="primary"
                disabled={!active || loading || connecting}
                onClick={() => setCredentialOpen(true)}
              >
                <Send size={16} />
                {connecting
                  ? tr('连接中', 'Connecting')
                  : tr('连接', 'Connect')}
              </Button>
            )}
            <Button
              disabled={!connected}
              onClick={toggleFullscreen}
            >
              {fullscreen ? <Minimize size={16} /> : <Fullscreen size={16} />}
              {fullscreen
                ? tr('退出全屏', 'Exit fullscreen')
                : tr('全屏', 'Fullscreen')}
            </Button>
            {connected && (
              <Button onClick={disconnectAndSummarize}>
                <PlugZap size={16} />
                {tr('断开', 'Disconnect')}
              </Button>
            )}
          </div>
        </header>

        {loading && (
          <div className="webdesktop-loading">
            <span className="ui-spinner" aria-label={tr('加载中', 'Loading')} />
          </div>
        )}
        {!loading && !active && (
          <Notice
            className="webdesktop-alert"
            tone="warning"
          >
            {target?.effective_status_message ||
              tr('当前远程桌面访问不可用', 'Remote desktop entry unavailable')}
          </Notice>
        )}
        {!loading && error && !credentialOpen && (
          <Notice
            className="webdesktop-alert"
            tone="danger"
          >{error}</Notice>
        )}

        <div className="webdesktop-display" ref={displayHostRef}>
          <div className="webdesktop-display-stage" ref={displayContentRef} />
          {connecting && (
            <div className="webdesktop-connecting-state">
              <span className="webdesktop-connecting-spinner" />
              <strong>
                {tr(
                  protocol === 'vnc'
                    ? '正在连接 VNC 桌面'
                    : '正在连接 RDP 桌面',
                  protocol === 'vnc'
                    ? 'Connecting to VNC desktop'
                    : 'Connecting to RDP desktop',
                )}
              </strong>
              <p>
                <span>{target?.application_name || '-'}</span>
                <i>·</i>
                <span>
                  {target
                    ? target.target_host + ':' + target.target_port
                    : '-'}
                </span>
              </p>
              <small>
                {tr(
                  '正在建立安全通道并初始化远程画面',
                  'Opening the secure tunnel and preparing the display',
                )}
              </small>
            </div>
          )}
          {!connected && !connecting && !loading && (
            <div className="webdesktop-empty">
              {sessionDurationSeconds !== null ? (
                <>
                  <span
                    className="webdesktop-session-finished-icon"
                    aria-hidden="true"
                  >
                    <Check size={18} />
                  </span>
                  <strong>
                    {tr('远程桌面会话已结束', 'Remote desktop session ended')}
                  </strong>
                  <p className="webdesktop-session-duration">
                    <span>{tr('本次使用', 'Duration')}</span>
                    <b>{formatSessionDuration(sessionDurationSeconds)}</b>
                  </p>
                  <div className="webdesktop-session-end-actions">
                    <Button onClick={returnToConnections}>
                      {tr('返回连接', 'Back to connections')}
                    </Button>
                    <Button
                      variant="primary"
                      disabled={!active}
                      onClick={() => setCredentialOpen(true)}
                    >
                      {tr('重新连接', 'Reconnect')}
                    </Button>
                  </div>
                </>
              ) : (
                <>
                  <span
                    className="webdesktop-screen-mark"
                    aria-hidden="true"
                  >
                    {protocol.toUpperCase()}
                  </span>
                  <strong>
                    {tr('远程桌面尚未连接', 'Remote desktop disconnected')}
                  </strong>
                  <p>
                    {tr(
                      '选择连接凭据后开始安全会话',
                      'Choose connection credentials to start a secure session',
                    )}
                  </p>
                  <Button
                    variant="primary"
                    disabled={!active}
                    onClick={() => setCredentialOpen(true)}
                  >
                    {tr('连接桌面', 'Connect desktop')}
                  </Button>
                </>
              )}
            </div>
          )}
          <SessionWatermark lines={watermarkLines} />
        </div>
      </div>

      <Modal
        title={tr(
          protocol === 'vnc' ? 'VNC 连接' : 'RDP 连接',
          protocol === 'vnc' ? 'VNC connection' : 'RDP connection',
        )}
        open={credentialOpen}
        onClose={closeCredentialModal}
        width={400}
        className="webdesktop-credential-modal"
        closeOnMask={!connecting}
      >
        <div className="webdesktop-credential-target">
          <div>
            <span>{tr('应用', 'Application')}</span>
            <strong>{target?.application_name || '-'}</strong>
          </div>
          <div>
            <span>{tr('目标', 'Target')}</span>
            <strong>
              {target ? `${target.target_host}:${target.target_port}` : '-'}
            </strong>
          </div>
        </div>

        {error && (
          <Notice
            className="webdesktop-credential-error"
            tone="danger"
          >{error}</Notice>
        )}

        <form
          className="webdesktop-credential-form"
          onSubmit={(event) => {
            event.preventDefault();
            submitCredentialConnection();
          }}
        >
          {protocol === 'rdp' && (
            <>
              <Field label={tr('域', 'Domain')}>
                <Input value={credentials.domain || ''} placeholder={tr('可选', 'Optional')} onChange={(event) => setCredentials((value) => ({ ...value, domain: event.target.value }))} />
              </Field>
              <Field label={tr('用户名', 'Username')} required>
                <Input list="webdesktop-saved-credentials" autoFocus autoComplete="username" value={credentials.username || ''} placeholder={tr('输入用户名', 'Enter username')} onChange={(event) => setCredentials((value) => ({ ...value, username: event.target.value }))} />
                <datalist id="webdesktop-saved-credentials">
                  {savedOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                </datalist>
              </Field>
            </>
          )}
          <Field label={tr('密码', 'Password')} required={!selectedSavedCredential}>
            <Input
              type="password"
              autoFocus={protocol !== 'rdp'}
              autoComplete="current-password"
              value={credentials.password || ''}
              placeholder={
                selectedSavedCredential
                  ? tr('留空使用保存密码', 'Leave blank to use saved password')
                  : tr('输入访问密码', 'Enter password')
              }
              onChange={(event) => setCredentials((value) => ({ ...value, password: event.target.value }))}
            />
          </Field>

          <div className="webdesktop-credential-options">
            {selectedSavedCredential && !credentials.password ? (
              <span className="webdesktop-credential-saved">
                <Check size={13} />
                {tr('密码已保存', 'Password saved')}
              </span>
            ) : (
              <label className="liaison-checkbox"><input type="checkbox" checked={Boolean(credentials.save_credential)} onChange={(event) => setCredentials((value) => ({ ...value, save_credential: event.target.checked }))} /><span>{selectedSavedCredential ? tr('更新保存密码', 'Update saved password') : tr('保存密码', 'Save password')}</span></label>
            )}
            {selectedSavedCredential && (
                <Button variant="ghost" disabled={connecting || connected} onClick={() => {
                  if (window.confirm(tr('清除当前保存密码？', 'Clear this saved password?'))) void clearCredential();
                }}>
                  {tr('清除保存密码', 'Clear saved password')}
                </Button>
            )}
          </div>

          <div className="webdesktop-credential-actions">
            <Button
              onClick={closeCredentialModal}
              disabled={connecting}
            >
              {tr('取消', 'Cancel')}
            </Button>
            <Button
              variant="primary"
              type="submit"
              loading={connecting}
              disabled={!active || loading}
            >
              <Send size={16} />
              {tr('连接', 'Connect')}
            </Button>
          </div>
        </form>
      </Modal>
    </>
  );
};

export default WebDesktopPage;
