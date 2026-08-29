import SessionWatermark, {
  buildSessionWatermarkLabel,
  useSessionWatermarkTime,
} from '@/components/SessionWatermark';
import { useI18n } from '@/i18n';
import { useModel, useParams } from '@/lib/runtime';
import {
  createWebDesktopSession,
  deleteWebDesktopCredential,
  getWebDesktopTarget,
} from '@/services/api';
import {
  DisconnectOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  SendOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  AutoComplete,
  Button,
  Checkbox,
  Form,
  Input,
  Modal,
  Popconfirm,
  Spin,
  message,
} from 'antd';
import Guacamole from 'guacamole-common-js';
import { useCallback, useEffect, useRef, useState } from 'react';
import './index.less';

const credentialKey = (username?: string, domain?: string) =>
  `${domain || ''}\\${username || ''}`;

const WebDesktopPage: React.FC = () => {
  const { tr } = useI18n();
  const { initialState } = useModel('@@initialState');
  const params = useParams();
  const proxyId = Number(params.proxyId);
  const [form] = Form.useForm<API.CreateWebDesktopSessionRequest>();
  const watchedUsername = Form.useWatch('username', form);
  const watchedDomain = Form.useWatch('domain', form);
  const [target, setTarget] = useState<API.WebDesktopTarget>();
  const [loading, setLoading] = useState(true);
  const [connecting, setConnecting] = useState(false);
  const [connected, setConnected] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [credentialOpen, setCredentialOpen] = useState(false);
  const [error, setError] = useState('');
  const displayHostRef = useRef<HTMLDivElement | null>(null);
  const displayContentRef = useRef<HTMLDivElement | null>(null);
  const clientRef = useRef<any>();
  const tunnelRef = useRef<any>();
  const keyboardRef = useRef<any>();
  const mouseRef = useRef<any>();
  const initialCredentialPromptRef = useRef(false);

  const active = target?.effective_status === 'active';
  const protocol = target?.protocol || 'rdp';
  const savedCredentials = target?.credentials || [];
  const selectedSavedCredential = savedCredentials.some(
    (item) =>
      credentialKey(item.username, item.domain) ===
      credentialKey(watchedUsername, watchedDomain),
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
    cleanupConnection(true, true);
  }, [cleanupConnection]);

  const disconnectAndPrompt = useCallback(() => {
    disconnect();
    setCredentialOpen(true);
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
      message.error(
        e?.message || tr('无法进入全屏', 'Unable to enter fullscreen'),
      );
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
        const credentials = res.data.credentials || [];
        if (credentials.length > 0) {
          const item = credentials[0];
          form.setFieldsValue({
            username: item.username || '',
            domain: item.domain || '',
            password: '',
            save_credential: false,
          });
        } else {
          form.setFieldsValue({ save_credential: false });
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
  }, [form, proxyId, tr]);

  useEffect(() => {
    loadTarget();
  }, [loadTarget]);

  useEffect(() => {
    if (
      !loading &&
      active &&
      !connected &&
      !initialCredentialPromptRef.current
    ) {
      initialCredentialPromptRef.current = true;
      setCredentialOpen(true);
    }
  }, [active, connected, loading]);

  useEffect(() => {
    document.body.classList.add('webdesktop-page-active');
    const footerElements = Array.from(
      document.querySelectorAll<HTMLElement>(
        '.ant-pro-layout-footer, .global-footer',
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
  }, []);

  useEffect(
    () => () => {
      disconnect();
    },
    [disconnect],
  );

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
    form.setFieldsValue({
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
      form.setFieldValue('password', '');
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
        cleanupConnection(true);
        setCredentialOpen(true);
      };
      tunnel.onstatechange = (state: number) => {
        if (state === Guacamole.Tunnel.State.OPEN) {
          setConnecting(false);
          setConnected(true);
          setCredentialOpen(false);
          inputPlane.focus({ preventScroll: true });
          if (shouldSaveCredential) {
            window.setTimeout(loadTarget, 1000);
          }
        }
        if (state === Guacamole.Tunnel.State.CLOSED) {
          cleanupConnection(false);
        }
      };
      client.onerror = (status: any) => {
        const text =
          status?.message ||
          tr('远程桌面连接失败', 'Remote desktop connection failed');
        setError(text);
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
      setConnecting(false);
      setConnected(false);
      setCredentialOpen(true);
    }
  };

  const clearCredential = async () => {
    try {
      await deleteWebDesktopCredential(proxyId, {
        protocol,
        username: String(form.getFieldValue('username') || '').trim(),
        domain: String(form.getFieldValue('domain') || '').trim(),
      });
      message.success(tr('已清除保存密码', 'Saved password cleared'));
      form.setFieldsValue({ password: '', save_credential: false });
      loadTarget();
    } catch (e: any) {
      message.error(
        e?.response?.data?.message ||
          tr('清除保存密码失败', 'Failed to clear saved password'),
      );
    }
  };

  return (
    <PageContainer title={false}>
      <div className="webdesktop-shell">
        <header className="webdesktop-toolbar">
          <div className="webdesktop-identity">
            <span className="webdesktop-screen-mark" aria-hidden="true">
              {protocol.toUpperCase()}
            </span>
            <div>
              <strong>
                {target?.proxy_name ||
                  tr('远程桌面会话', 'Remote desktop session')}
              </strong>
              <span>{tr('安全远程桌面', 'Secure remote desktop')}</span>
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
                className={`webdesktop-status ${
                  connected ? 'is-connected' : active ? 'is-ready' : 'is-error'
                }`}
              >
                <i />
                {connected
                  ? tr('已连接', 'Connected')
                  : active
                  ? tr('待连接', 'Ready')
                  : tr('不可用', 'Unavailable')}
              </strong>
            </div>
          </div>

          <div className="webdesktop-toolbar-actions">
            {!connected && (
              <Button
                type="primary"
                icon={<SendOutlined />}
                disabled={!active || loading}
                onClick={() => setCredentialOpen(true)}
              >
                {tr('连接', 'Connect')}
              </Button>
            )}
            <Button
              icon={
                fullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />
              }
              disabled={!connected}
              onClick={toggleFullscreen}
            >
              {fullscreen
                ? tr('退出全屏', 'Exit fullscreen')
                : tr('全屏', 'Fullscreen')}
            </Button>
            {connected && (
              <Button
                icon={<DisconnectOutlined />}
                onClick={disconnectAndPrompt}
              >
                {tr('断开', 'Disconnect')}
              </Button>
            )}
          </div>
        </header>

        {loading && (
          <div className="webdesktop-loading">
            <Spin />
          </div>
        )}
        {!loading && !active && (
          <Alert
            className="webdesktop-alert"
            type="warning"
            showIcon
            message={
              target?.effective_status_message ||
              tr('当前远程桌面访问不可用', 'Remote desktop entry unavailable')
            }
          />
        )}
        {!loading && error && !credentialOpen && (
          <Alert
            className="webdesktop-alert"
            type="error"
            showIcon
            message={error}
          />
        )}

        <div className="webdesktop-display" ref={displayHostRef}>
          <div className="webdesktop-display-stage" ref={displayContentRef} />
          {!connected && !connecting && !loading && (
            <div className="webdesktop-empty">
              <span className="webdesktop-screen-mark" aria-hidden="true">
                {protocol.toUpperCase()}
              </span>
              <strong>
                {tr('远程桌面尚未连接', 'Remote desktop disconnected')}
              </strong>
              <p>
                {tr(
                  '输入访问凭据后开始安全会话',
                  'Enter credentials to start a secure session',
                )}
              </p>
              <Button
                type="primary"
                disabled={!active}
                onClick={() => setCredentialOpen(true)}
              >
                {tr('连接桌面', 'Connect desktop')}
              </Button>
            </div>
          )}
          <SessionWatermark lines={watermarkLines} />
        </div>
      </div>

      <Modal
        className="webdesktop-credential-modal"
        title={tr('连接远程桌面', 'Connect remote desktop')}
        open={credentialOpen}
        footer={null}
        width={440}
        centered
        destroyOnClose={false}
        maskClosable={!connecting}
        closable={!connecting}
        onCancel={() => setCredentialOpen(false)}
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
          <Alert
            className="webdesktop-credential-error"
            type="error"
            showIcon
            message={error}
          />
        )}

        <Form
          form={form}
          layout="vertical"
          onFinish={connect}
          disabled={connecting || connected || loading}
        >
          {protocol === 'rdp' && (
            <>
              <Form.Item name="domain" label={tr('域', 'Domain')}>
                <Input placeholder={tr('可选', 'Optional')} />
              </Form.Item>
              <Form.Item
                name="username"
                label={tr('用户名', 'Username')}
                rules={[
                  {
                    required: true,
                    message: tr('请输入用户名', 'Username is required'),
                  },
                ]}
              >
                <AutoComplete
                  allowClear
                  defaultActiveFirstOption={false}
                  options={savedOptions}
                  onSelect={applySelectedCredential}
                >
                  <Input
                    autoFocus
                    autoComplete="username"
                    placeholder={tr('输入用户名', 'Enter username')}
                  />
                </AutoComplete>
              </Form.Item>
            </>
          )}
          <Form.Item
            name="password"
            label={tr('密码', 'Password')}
            rules={[
              {
                validator: async (_, value) => {
                  if (!selectedSavedCredential && !value) {
                    throw new Error(tr('请输入密码', 'Password is required'));
                  }
                },
              },
            ]}
          >
            <Input.Password
              autoFocus={protocol !== 'rdp'}
              autoComplete="current-password"
              placeholder={
                selectedSavedCredential
                  ? tr('留空使用保存密码', 'Leave blank to use saved password')
                  : tr('输入访问密码', 'Enter password')
              }
              onPressEnter={() => form.submit()}
            />
          </Form.Item>

          <div className="webdesktop-credential-options">
            <Form.Item name="save_credential" valuePropName="checked">
              <Checkbox>{tr('保存密码', 'Save password')}</Checkbox>
            </Form.Item>
            {selectedSavedCredential && (
              <Popconfirm
                title={tr('清除当前保存密码？', 'Clear this saved password?')}
                okText={tr('清除', 'Clear')}
                cancelText={tr('取消', 'Cancel')}
                onConfirm={clearCredential}
              >
                <Button type="link" disabled={connecting || connected}>
                  {tr('清除保存密码', 'Clear saved password')}
                </Button>
              </Popconfirm>
            )}
          </div>

          <div className="webdesktop-credential-actions">
            <Button
              onClick={() => setCredentialOpen(false)}
              disabled={connecting}
            >
              {tr('取消', 'Cancel')}
            </Button>
            <Button
              type="primary"
              htmlType="submit"
              icon={<SendOutlined />}
              loading={connecting}
              disabled={!active || loading}
            >
              {tr('连接', 'Connect')}
            </Button>
          </div>
        </Form>
      </Modal>
    </PageContainer>
  );
};

export default WebDesktopPage;
