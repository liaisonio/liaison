import { useI18n } from '@/i18n';
import {
  createWebSSHSession,
  deleteWebSSHCredential,
  deleteWebSSHHostKey,
  getWebSSHTarget,
} from '@/services/api';
import {
  DisconnectOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SendOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import { useParams } from '@umijs/max';
import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import {
  Alert,
  AutoComplete,
  Button,
  Checkbox,
  Form,
  Input,
  Popconfirm,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  ClipboardEvent,
  FormEvent,
  KeyboardEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import './index.less';

const { Text } = Typography;
const webSSHHeartbeatIntervalMs = 25_000;
const webSSHHeartbeatTimeoutMs = 75_000;

const WebSSHPage: React.FC = () => {
  const { tr } = useI18n();
  const params = useParams();
  const proxyId = Number(params.proxyId);
  const [form] = Form.useForm<API.CreateWebSSHSessionRequest>();
  const watchedUsername = Form.useWatch('username', form);
  const [target, setTarget] = useState<API.WebSSHTarget>();
  const [loading, setLoading] = useState(true);
  const [connecting, setConnecting] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState('');
  const terminalFrameRef = useRef<HTMLDivElement | null>(null);
  const terminalHostRef = useRef<HTMLDivElement | null>(null);
  const keyboardCaptureRef = useRef<HTMLTextAreaElement | null>(null);
  const terminalRef = useRef<Terminal>();
  const fitAddonRef = useRef<FitAddon>();
  const socketRef = useRef<WebSocket>();
  const heartbeatTimerRef = useRef<number>();
  const lastServerMessageAtRef = useRef(0);
  const pendingCredentialSaveRef = useRef(false);

  const active = target?.effective_status === 'active';
  const savedCredentials = target?.credentials || [];
  const credentialSaved = savedCredentials.length > 0;
  const selectedSavedCredential = savedCredentials.some(
    (item) => item.username === watchedUsername,
  );
  const savedUsernameOptions = savedCredentials.map((item) => ({
    label: item.username,
    value: item.username,
  }));

  const sendTerminalInput = useCallback((data: string) => {
    if (data && socketRef.current?.readyState === WebSocket.OPEN) {
      socketRef.current.send(JSON.stringify({ type: 'input', data }));
    }
  }, []);

  const focusTerminal = useCallback(() => {
    terminalRef.current?.focus();
    terminalFrameRef.current?.focus();
    keyboardCaptureRef.current?.focus();
    requestAnimationFrame(() => {
      terminalRef.current?.focus();
      terminalFrameRef.current?.focus();
      keyboardCaptureRef.current?.focus();
    });
  }, []);

  const keyEventToTerminalInput = useCallback(
    (event: Pick<KeyboardEvent, 'altKey' | 'ctrlKey' | 'key' | 'metaKey'>) => {
      if (event.ctrlKey && event.key.length === 1) {
        const code = event.key.toUpperCase().charCodeAt(0);
        if (code >= 65 && code <= 90) {
          return String.fromCharCode(code - 64);
        }
      }

      switch (event.key) {
        case 'Enter':
          return '\r';
        case 'Backspace':
          return '\x7f';
        case 'Tab':
          return '\t';
        case 'Escape':
          return '\x1b';
        case 'ArrowUp':
          return '\x1b[A';
        case 'ArrowDown':
          return '\x1b[B';
        case 'ArrowRight':
          return '\x1b[C';
        case 'ArrowLeft':
          return '\x1b[D';
        case 'Delete':
          return '\x1b[3~';
        case 'Home':
          return '\x1b[H';
        case 'End':
          return '\x1b[F';
        default:
          if (!event.metaKey && !event.altKey && event.key.length === 1) {
            return event.key;
          }
      }
      return '';
    },
    [],
  );

  const handleTerminalKeyDown = useCallback(
    (event: KeyboardEvent<HTMLDivElement>) => {
      const targetNode = event.target as Node | null;
      if (
        !terminalFrameRef.current ||
        !targetNode ||
        !terminalFrameRef.current.contains(targetNode)
      ) {
        return;
      }

      const data = keyEventToTerminalInput(event);
      if (!data) return;
      if (
        event.target === keyboardCaptureRef.current &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.altKey &&
        event.key.length === 1
      ) {
        return;
      }
      event.preventDefault();
      sendTerminalInput(data);
    },
    [keyEventToTerminalInput, sendTerminalInput],
  );

  const handleCaptureInput = useCallback(
    (event: FormEvent<HTMLTextAreaElement>) => {
      const value = event.currentTarget.value;
      if (value) {
        sendTerminalInput(value);
        event.currentTarget.value = '';
      }
    },
    [sendTerminalInput],
  );

  const handleCapturePaste = useCallback(
    (event: ClipboardEvent<HTMLTextAreaElement>) => {
      const text = event.clipboardData.getData('text');
      if (!text) return;
      event.preventDefault();
      sendTerminalInput(text);
      event.currentTarget.value = '';
    },
    [sendTerminalInput],
  );

  const stopHeartbeat = useCallback(() => {
    if (heartbeatTimerRef.current !== undefined) {
      window.clearInterval(heartbeatTimerRef.current);
      heartbeatTimerRef.current = undefined;
    }
  }, []);

  const markWebSSHAlive = useCallback(() => {
    lastServerMessageAtRef.current = Date.now();
  }, []);

  const startHeartbeat = useCallback(
    (socket: WebSocket) => {
      stopHeartbeat();
      markWebSSHAlive();
      heartbeatTimerRef.current = window.setInterval(() => {
        if (socketRef.current !== socket) {
          stopHeartbeat();
          return;
        }
        if (socket.readyState !== WebSocket.OPEN) {
          stopHeartbeat();
          return;
        }
        if (Date.now() - lastServerMessageAtRef.current > webSSHHeartbeatTimeoutMs) {
          const text = tr(
            'WebSSH 连接超时，请重新连接',
            'WebSSH connection timed out, please reconnect',
          );
          setError(text);
          terminalRef.current?.writeln(`\r\n${text}`);
          setConnected(false);
          setConnecting(false);
          stopHeartbeat();
          socket.close();
          return;
        }
        socket.send(JSON.stringify({ type: 'ping' }));
      }, webSSHHeartbeatIntervalMs);
    },
    [markWebSSHAlive, stopHeartbeat, tr],
  );

  const loadTarget = useCallback(async () => {
    if (!proxyId) {
      setError(tr('访问 ID 无效', 'Invalid entry ID'));
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const res = await getWebSSHTarget(proxyId);
      if (res.code === 200 && res.data) {
        setTarget(res.data);
        const credentials = res.data.credentials || [];
        if (credentials.length > 0) {
          const currentUsername = form.getFieldValue('username');
          const username =
            credentials.find((item) => item.username === currentUsername)
              ?.username || credentials[0].username;
          form.setFieldsValue({
            username,
            password: '',
            save_credential: false,
          });
        } else {
          form.setFieldsValue({ save_credential: false });
        }
        setError('');
      } else {
        setError(
          res.message || tr('获取 SSH 目标失败', 'Failed to load SSH target'),
        );
      }
    } catch (e: any) {
      setError(
        e?.response?.data?.message ||
          tr('获取 SSH 目标失败', 'Failed to load SSH target'),
      );
    } finally {
      setLoading(false);
    }
  }, [form, proxyId, tr]);

  useEffect(() => {
    loadTarget();
  }, [loadTarget]);

  useEffect(() => {
    if (!terminalHostRef.current || terminalRef.current) return;
    const terminal = new Terminal({
      cursorBlink: true,
      convertEol: true,
      fontFamily: 'Menlo, Monaco, Consolas, "Liberation Mono", monospace',
      fontSize: 13,
      theme: {
        background: '#101418',
        foreground: '#d7dee8',
        cursor: '#7dd3fc',
        selectionBackground: '#334155',
      },
    });
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(terminalHostRef.current);
    fitAddon.fit();
    terminal.onData(sendTerminalInput);
    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;

    const handleResize = () => {
      fitAddon.fit();
      if (socketRef.current?.readyState === WebSocket.OPEN) {
        socketRef.current.send(
          JSON.stringify({
            type: 'resize',
            cols: terminal.cols,
            rows: terminal.rows,
          }),
        );
      }
    };
    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
      stopHeartbeat();
      socketRef.current?.close();
      terminal.dispose();
      terminalRef.current = undefined;
      fitAddonRef.current = undefined;
    };
  }, [sendTerminalInput, stopHeartbeat]);

  const disconnect = useCallback(() => {
    pendingCredentialSaveRef.current = false;
    stopHeartbeat();
    socketRef.current?.close();
    socketRef.current = undefined;
    setConnected(false);
    setConnecting(false);
  }, [stopHeartbeat]);

  useEffect(() => {
    if (connected) {
      focusTerminal();
    }
  }, [connected, focusTerminal]);

  useEffect(() => {
    if (!connected) return;

    const handleDocumentKeyDown = (event: globalThis.KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (terminalFrameRef.current?.contains(target)) return;
      if (
        target?.closest(
          'input, textarea, select, button, [contenteditable="true"]',
        )
      ) {
        return;
      }

      const activeElement = document.activeElement;
      if (
        activeElement &&
          activeElement !== document.body &&
          activeElement !== document.documentElement &&
          !terminalFrameRef.current?.contains(activeElement)
      ) {
        return;
      }

      const data = keyEventToTerminalInput(event);
      if (!data) return;
      event.preventDefault();
      sendTerminalInput(data);
    };

    document.addEventListener('keydown', handleDocumentKeyDown, true);
    return () => {
      document.removeEventListener('keydown', handleDocumentKeyDown, true);
    };
  }, [connected, keyEventToTerminalInput, sendTerminalInput]);

  const connect = async (values: API.CreateWebSSHSessionRequest) => {
    if (!target || !active) return;
    disconnect();
    setConnecting(true);
    setError('');
    fitAddonRef.current?.fit();
    terminalRef.current?.reset();
    terminalRef.current?.writeln(tr('正在连接 SSH...', 'Connecting to SSH...'));
    try {
      const password = values.password || '';
      const shouldSaveCredential = Boolean(values.save_credential && password);
      pendingCredentialSaveRef.current = shouldSaveCredential;
      const res = await createWebSSHSession(proxyId, {
        username: values.username?.trim(),
        password,
        save_credential: shouldSaveCredential,
        cols: terminalRef.current?.cols,
        rows: terminalRef.current?.rows,
      });
      form.setFieldValue('password', '');
      if (res.code !== 200 || !res.data?.ws_url) {
        throw new Error(
          res.message ||
            tr('创建 WebSSH 会话失败', 'Failed to create WebSSH session'),
        );
      }
      const socket = new WebSocket(res.data.ws_url);
      socketRef.current = socket;
      socket.onopen = () => {
        startHeartbeat(socket);
        socket.send(
          JSON.stringify({
            type: 'resize',
            cols: terminalRef.current?.cols,
            rows: terminalRef.current?.rows,
          }),
        );
        focusTerminal();
      };
      socket.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          markWebSSHAlive();
          if (msg.type === 'output') {
            terminalRef.current?.write(msg.data || '');
          } else if (msg.type === 'pong') {
            return;
          } else if (msg.type === 'credential_saved') {
            pendingCredentialSaveRef.current = false;
            message.success(tr('已保存 SSH 密码', 'SSH password saved'));
            loadTarget();
          } else if (msg.type === 'credential_error') {
            pendingCredentialSaveRef.current = false;
            message.warning(
              msg.message ||
                tr('SSH 已连接，但保存密码失败', 'SSH connected, but saving password failed'),
            );
          } else if (msg.type === 'status') {
            if (msg.status === 'connected') {
              setConnected(true);
              setConnecting(false);
              focusTerminal();
            }
            if (msg.status === 'closed') {
              pendingCredentialSaveRef.current = false;
              setConnected(false);
              setConnecting(false);
            }
          } else if (msg.type === 'error') {
            pendingCredentialSaveRef.current = false;
            const text =
              msg.message || tr('SSH 连接失败', 'SSH connection failed');
            setError(text);
            terminalRef.current?.writeln(`\r\n${text}`);
            setConnected(false);
            setConnecting(false);
            socket.close();
          }
        } catch {
          terminalRef.current?.write(String(event.data || ''));
        }
      };
      socket.onerror = () => {
        pendingCredentialSaveRef.current = false;
        stopHeartbeat();
        setError(tr('WebSSH 连接异常', 'WebSSH connection error'));
        setConnected(false);
        setConnecting(false);
      };
      socket.onclose = () => {
        stopHeartbeat();
        setConnected(false);
        setConnecting(false);
      };
    } catch (e: any) {
      pendingCredentialSaveRef.current = false;
      const text =
        e?.response?.data?.message ||
        e?.message ||
        tr('创建 WebSSH 会话失败', 'Failed to create WebSSH session');
      setError(text);
      terminalRef.current?.writeln(`\r\n${text}`);
      setConnecting(false);
    }
  };

  const resetHostKey = async () => {
    try {
      await deleteWebSSHHostKey(proxyId);
      message.success(tr('已重置信任指纹', 'Trusted fingerprint reset'));
      loadTarget();
    } catch (e: any) {
      message.error(
        e?.response?.data?.message || tr('重置失败', 'Reset failed'),
      );
    }
  };

  const clearCredential = async () => {
    const username = String(form.getFieldValue('username') || '').trim();
    if (!username) {
      message.warning(tr('请选择保存用户', 'Select a saved user'));
      return;
    }
    try {
      await deleteWebSSHCredential(proxyId, username);
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
    <PageContainer title={tr('WebSSH', 'WebSSH')}>
      <div className="webssh-shell">
        <div className="webssh-toolbar">
          <Space size={12} wrap>
            <Text strong>
              {target?.proxy_name || tr('SSH 访问', 'SSH Entry')}
            </Text>
            {target && (
              <Text type="secondary">
                {target.application_name} · {target.target_host}:
                {target.target_port}
              </Text>
            )}
            {target?.host_key?.trusted && (
              <Tag icon={<SafetyCertificateOutlined />} color="processing">
                {target.host_key.fingerprint_sha256}
              </Tag>
            )}
            {target && (
              <Tag color={active ? 'success' : 'error'}>
                {active ? tr('可用', 'Available') : tr('不可用', 'Unavailable')}
              </Tag>
            )}
            {credentialSaved && (
              <Tag color="green">
                {tr(
                  `已保存 ${savedCredentials.length} 个用户`,
                  `${savedCredentials.length} saved users`,
                )}
              </Tag>
            )}
          </Space>
          <Space>
            <Button icon={<ReloadOutlined />} onClick={loadTarget}>
              {tr('刷新', 'Refresh')}
            </Button>
            <Button
              icon={<DisconnectOutlined />}
              disabled={!connected && !connecting}
              onClick={disconnect}
            >
              {tr('断开', 'Disconnect')}
            </Button>
          </Space>
        </div>

        {loading && (
          <div className="webssh-loading">
            <Spin />
          </div>
        )}
        {!loading && !active && (
          <Alert
            className="webssh-alert"
            type="warning"
            showIcon
            message={
              target?.effective_status_message ||
              tr('当前 SSH 访问不可用', 'SSH entry unavailable')
            }
          />
        )}
        {!loading && error && (
          <Alert
            className="webssh-alert"
            type="error"
            showIcon
            message={error}
            action={
              error.includes('指纹变化') ? (
                <Button size="small" danger onClick={resetHostKey}>
                  {tr('重置指纹', 'Reset fingerprint')}
                </Button>
              ) : undefined
            }
          />
        )}

        <div className="webssh-body">
          <div className="webssh-login">
            <Form
              form={form}
              layout="inline"
              onFinish={connect}
              disabled={connecting || connected || loading}
            >
              <Form.Item
                name="username"
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
                  options={savedUsernameOptions}
                  filterOption={(input, option) =>
                    String(option?.value || '')
                      .toLowerCase()
                      .includes(input.toLowerCase())
                  }
                >
                  <Input
                    autoComplete="username"
                    placeholder={tr('用户名', 'Username')}
                  />
                </AutoComplete>
              </Form.Item>
              <Form.Item
                name="password"
                rules={[
                  {
                    validator: async (_, value) => {
                      if (!selectedSavedCredential && !value) {
                        throw new Error(
                          tr('请输入密码', 'Password is required'),
                        );
                      }
                    },
                  },
                ]}
              >
                <Input.Password
                  autoComplete="current-password"
                  placeholder={
                    selectedSavedCredential
                      ? tr('留空使用该用户保存密码', 'Leave blank to use the saved password')
                      : tr('密码', 'Password')
                  }
                  onPressEnter={() => form.submit()}
                />
              </Form.Item>
              <Form.Item name="save_credential" valuePropName="checked">
                <Checkbox>
                  {credentialSaved
                    ? tr('保存/更新该用户密码', 'Save or update this user password')
                    : tr('保存密码', 'Save password')}
                </Checkbox>
              </Form.Item>
              {selectedSavedCredential && (
                <Popconfirm
                  title={tr('清除当前用户保存密码？', 'Clear saved password for this user?')}
                  okText={tr('清除', 'Clear')}
                  cancelText={tr('取消', 'Cancel')}
                  onConfirm={clearCredential}
                >
                  <Button type="link" disabled={connecting || connected}>
                    {tr('清除当前用户密码', 'Clear this user password')}
                  </Button>
                </Popconfirm>
              )}
              <Button
                type="primary"
                htmlType="submit"
                loading={connecting}
                disabled={!active || connected || loading}
              >
                <SendOutlined />{' '}
                {connected ? tr('已连接', 'Connected') : tr('连接', 'Connect')}
              </Button>
            </Form>
          </div>
          <div
            className="webssh-terminal"
            ref={terminalFrameRef}
            tabIndex={0}
            onClick={focusTerminal}
            onKeyDownCapture={handleTerminalKeyDown}
            onMouseDown={focusTerminal}
          >
            <div className="webssh-terminal-screen" ref={terminalHostRef} />
            <textarea
              ref={keyboardCaptureRef}
              aria-label="WebSSH keyboard input"
              autoCapitalize="off"
              autoComplete="off"
              autoCorrect="off"
              className="webssh-key-capture"
              onInput={handleCaptureInput}
              onPaste={handleCapturePaste}
              spellCheck={false}
              wrap="off"
            />
          </div>
        </div>
      </div>
    </PageContainer>
  );
};

export default WebSSHPage;
