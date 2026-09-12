import SessionWatermark, {
  buildSessionWatermarkLabel,
  useSessionWatermarkTime,
} from '@/components/SessionWatermark';
import AgentWorkspace from '@/components/AgentWorkspace';
import ShellAgent from '@/components/TerminalAssistant/ShellAgent';
import {createAgentSession} from '@/services/agent';
import SessionInfo from '@/components/SessionReference/SessionInfo';
import {request} from '@/api/client';
import { useSessionPath, SessionPathNotice } from '@/components/SessionReference/useSessionPath';
import '@/components/TerminalAssistant/index.less';
import { attachTerminalCompletion, type TerminalCompletionView } from '@/components/TerminalAssistant/terminalCompletion';
import CommandCompletion from '@/components/TerminalAssistant/CommandCompletion';
import { useFeature } from '@/store/permissions';
import { Button, Field, Input, Modal, Notice } from '@/components/ui';
import { useI18n } from '@/i18n';
import { history, useLocation, useModel, useParams, useSearchParams } from '@/lib/runtime';
import {
  createWebSSHSession,
  deleteWebSSHCredential,
  deleteWebSSHHostKey,
  getWebSSHTarget,
} from '@/services/api';
import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { ArrowLeft, Check, Clock3, Fullscreen, LogIn, Minimize, Plus, PlugZap, Send, Sparkles, Trash2 } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import './index.less';

const webSSHHeartbeatIntervalMs = 25_000;
const webSSHHeartbeatTimeoutMs = 75_000;
const webSSHOutputFlushDelayMs = 8;

const formatWebSSHTime = (value?: string) => {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value.replace('T', ' ').replace(/Z$/, '');
  const part = (item: number) => String(item).padStart(2, '0');
  return `${date.getFullYear()}-${part(date.getMonth() + 1)}-${part(date.getDate())} ${part(date.getHours())}:${part(date.getMinutes())}:${part(date.getSeconds())}`;
};

const isTerminalAtBottom = (terminal?: Terminal) => {
  if (!terminal) return true;
  const buffer = terminal.buffer.active;
  return buffer.viewportY >= buffer.baseY;
};

const WebSSHPage: React.FC = () => {
  const { tr } = useI18n();
  const { initialState } = useModel('@@initialState');
  const params = useParams();
  const location = useLocation();
  const [routeSearch] = useSearchParams();
  const [resolvedProxyId,setResolvedProxyId]=useState(0);
  const shortSession=Boolean(params.connectionReference);
  const proxyId = Number(params.proxyId || location.state?.sessionRoute?.proxyId || resolvedProxyId);
  const credentialId = Number(params.credentialId || location.state?.sessionRoute?.credentialId || 0);
  useEffect(()=>{
    if(!shortSession)return;
    let active=true;
    void request<API.Response<{access_id:number}>>(`/api/v1/webssh/session-references/${encodeURIComponent(params.connectionReference!)}`,{params:{agent:routeSearch.get('agent')||undefined}})
      .then(r=>{if(active&&r.data)setResolvedProxyId(r.data.access_id);})
      .catch(()=>{/* Expired sessions do not reconnect automatically. */});
    return()=>{active=false;};
  },[params.connectionReference]);
  const isTemporarySession = /\/session(?:\/|$)/.test(location.pathname);
  const isTerminalView = shortSession || isTemporarySession || credentialId > 0;
  const [credentials, setCredentials] = useState<API.CreateWebSSHSessionRequest>({ username: '', password: '', save_credential: false });
  const [pendingSessionCredentials, setPendingSessionCredentials] = useState<API.CreateWebSSHSessionRequest>();
  const watchedUsername = credentials.username;
  const [target, setTarget] = useState<API.WebSSHTarget>();
  const [loading, setLoading] = useState(true);
  const [connecting, setConnecting] = useState(false);
  const [connected, setConnected] = useState(false);
  const [agentHandleID, setAgentHandleID] = useState('');
  const [shellDetail, setShellDetail] = useState<{handle: string; detail: API.AgentSessionDetail}>();
  const shellSession = useRef<{handle: string; pending: Promise<API.AgentSessionDetail>}>();
  const ensureShellSession = async (handle: string) => {
    if (shellSession.current?.handle === handle) return shellSession.current.pending;
    const pending = createAgentSession(handle, 'Shell Agent', 'shell').then(result => {
      if (!result.data) throw new Error('Shell Agent session unavailable');
      if (completionBinding.current.handle === handle && completionBinding.current.allowed) setShellDetail({handle, detail: result.data});
      return result.data;
    });
    const binding = {handle, pending};
    shellSession.current = binding;
    try {return await pending;} catch (error) {
      if (shellSession.current === binding) shellSession.current = undefined;
      throw error;
    }
  };
  const [completion, setCompletion] = useState<TerminalCompletionView | null>(null);
  const completionRef = useRef<ReturnType<typeof attachTerminalCompletion>>();
  const [automaticAI, setAutomaticAI] = useState(true);
  const [contextMode, setContextMode] = useState<'none'|'commands'|'output'>('none');
  const [shellExitCode, setShellExitCode] = useState<number>();
  const [completionStatus, setCompletionStatus] = useState<'idle' | 'busy' | 'empty' | 'error' | 'unavailable'>('idle');
  const completionBinding = useRef({ handle: '', allowed: false, editor: crypto.randomUUID(), contextMode: 'none' });
  const completionRevision = useRef(0);
  const [connectionReferenceHandle, setConnectionReferenceHandle] = useState('');
  const [agentOpen, setAgentOpen] = useState(false);
  const canAI = useFeature('ai.access.use');
  completionBinding.current.handle = agentHandleID;
  completionBinding.current.allowed = canAI && connected;
  completionBinding.current.contextMode = contextMode;
  useEffect(() => {
    completionRef.current?.reset();
    completionRef.current?.setAutomatic(canAI && connected);
    setAutomaticAI(true); setContextMode('none'); setShellExitCode(undefined);
    const editor = crypto.randomUUID();
    completionBinding.current.editor = editor;
    return () => {
      completionRef.current?.reset();
      if (agentHandleID) void request('/api/v1/assistance/suggestions', {
        method: 'DELETE', data: { handle_id: agentHandleID, editor_id: editor }, skipErrorHandler: true,
      }).catch(() => undefined);
    };
  }, [agentHandleID, canAI, connected]);
  const sessionPath = useSessionPath(agentHandleID, agentOpen, setAgentOpen);
  const [fullscreen, setFullscreen] = useState(false);
  const [credentialOpen, setCredentialOpen] = useState(false);
  const [sessionDurationSeconds, setSessionDurationSeconds] = useState<
    number | null
  >(null);
  const [error, setError] = useState('');
  const terminalFrameRef = useRef<HTMLDivElement | null>(null);
  const terminalHostRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal>();
  const fitAddonRef = useRef<FitAddon>();
  const socketRef = useRef<WebSocket>();
  const heartbeatTimerRef = useRef<number>();
  const lastResizeRef = useRef<{ cols: number; rows: number }>();
  const lastServerMessageAtRef = useRef(0);
  const pendingCredentialSaveRef = useRef(false);
  const inputBufferRef = useRef('');
  const inputFlushQueuedRef = useRef(false);
  const outputBufferRef = useRef('');
  const outputFlushTimerRef = useRef<number>();
  const outputStickToBottomRef = useRef(true);
  const initialCredentialPromptRef = useRef(false);
  const initialSavedConnectRef = useRef<number>();
  const sessionStartedAtRef = useRef<number>();

  const active = target?.effective_status === 'active';
  const savedCredentials = target?.credentials || [];
  const selectedSavedCredential = credentialId > 0 && savedCredentials.some(
    (item) => item.id === credentialId && item.username === watchedUsername,
  );
  const watermarkTime = useSessionWatermarkTime();
  const watermarkUser =
    initialState?.currentUser?.email ||
    initialState?.currentUser?.name ||
    tr('未知用户', 'Unknown user');
  const watermarkLines = target
    ? [
        buildSessionWatermarkLabel([watermarkUser, 'SSH', target.proxy_name]),
        watermarkTime,
      ]
    : [buildSessionWatermarkLabel([watermarkUser, 'SSH']), watermarkTime];

  const formatSessionDuration = useCallback(
    (seconds: number) => {
      const hours = Math.floor(seconds / 3600);
      const minutes = Math.floor((seconds % 3600) / 60);
      const remainingSeconds = seconds % 60;
      if (hours > 0) {
        return tr(
          `${hours} 小时 ${minutes} 分钟`,
          `${hours}h ${minutes}m`,
        );
      }
      if (minutes > 0) {
        return tr(
          `${minutes} 分钟 ${remainingSeconds} 秒`,
          `${minutes}m ${remainingSeconds}s`,
        );
      }
      return tr(`${remainingSeconds} 秒`, `${remainingSeconds}s`);
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

  const requestedReturnPath = routeSearch.get('from') || '';
  const accessReturnPath = requestedReturnPath.startsWith('/proxy') && !requestedReturnPath.startsWith('//')
    ? requestedReturnPath
    : '/proxy?access_type=webssh';

  const connectionListPath = `/webssh/${proxyId}${
    requestedReturnPath ? `?from=${encodeURIComponent(accessReturnPath)}` : ''
  }`;
  const temporarySessionPath = `/webssh/${proxyId}/session${
    requestedReturnPath ? `?from=${encodeURIComponent(accessReturnPath)}` : ''
  }`;

  const returnToAccess = useCallback(() => {
    history.push(accessReturnPath);
  }, [accessReturnPath]);

  const returnToConnections = useCallback(() => {
    history.push(connectionListPath);
  }, [connectionListPath]);

  const openNewConnection = useCallback(() => {
    initialCredentialPromptRef.current = false;
    setError('');
    setCredentials({ username: '', password: '', save_credential: false });
    setCredentialOpen(true);
  }, []);

  const closeCredentialModal = useCallback(() => {
    setError('');
    setCredentialOpen(false);
    if (isTerminalView) returnToConnections();
  }, [isTerminalView, returnToConnections]);

  const openSavedConnection = useCallback(
    (credential: API.WebSSHCredential) => {
      if (!credential.id) return;
      initialSavedConnectRef.current = undefined;
      history.push(
        `/webssh/${proxyId}/connections/${credential.id}${
          requestedReturnPath ? `?from=${encodeURIComponent(accessReturnPath)}` : ''
        }`,
      );
    },
    [accessReturnPath, proxyId, requestedReturnPath],
  );

  const flushTerminalInput = useCallback(() => {
    inputFlushQueuedRef.current = false;
    const data = inputBufferRef.current;
    inputBufferRef.current = '';
    if (data && socketRef.current?.readyState === WebSocket.OPEN) {
      socketRef.current.send(JSON.stringify({ type: 'input', data }));
    }
  }, []);

  const sendTerminalInput = useCallback(
    (data: string) => {
      if (!data) return;
      inputBufferRef.current += data;
      if (inputFlushQueuedRef.current) return;
      inputFlushQueuedRef.current = true;
      queueMicrotask(flushTerminalInput);
    },
    [flushTerminalInput],
  );

  const flushTerminalOutput = useCallback(() => {
    if (outputFlushTimerRef.current !== undefined) {
      window.clearTimeout(outputFlushTimerRef.current);
      outputFlushTimerRef.current = undefined;
    }
    const terminal = terminalRef.current;
    const data = outputBufferRef.current;
    outputBufferRef.current = '';
    if (!terminal || !data) return;
    const stickToBottom = outputStickToBottomRef.current;
    outputStickToBottomRef.current = true;
    terminal.write(data, () => {
      if (stickToBottom) {
        terminal.scrollToBottom();
      }
    });
  }, []);

  const writeTerminalOutput = useCallback(
    (data: string) => {
      if (!data || !terminalRef.current) return;
      outputStickToBottomRef.current =
        outputStickToBottomRef.current &&
        isTerminalAtBottom(terminalRef.current);
      outputBufferRef.current += data;
      if (outputFlushTimerRef.current !== undefined) return;
      outputFlushTimerRef.current = window.setTimeout(
        flushTerminalOutput,
        webSSHOutputFlushDelayMs,
      );
    },
    [flushTerminalOutput],
  );

  const focusTerminal = useCallback((force = false) => {
    if (!force && document.activeElement?.closest('.agent-workspace, .terminal-assistant')) return;
    if (!terminalRef.current) {
      terminalFrameRef.current?.focus();
      return;
    }
    terminalRef.current.focus();
    requestAnimationFrame(() => {
      if (!document.activeElement?.closest('.agent-workspace, .terminal-assistant')) terminalRef.current?.focus();
    });
  }, []);

  const fitTerminal = useCallback(
    (stickToBottom = isTerminalAtBottom(terminalRef.current)) => {
      fitAddonRef.current?.fit();
      if (stickToBottom) {
        terminalRef.current?.scrollToBottom();
      }
      const terminal = terminalRef.current;
      if (socketRef.current?.readyState === WebSocket.OPEN && terminal) {
        const cols = terminal.cols;
        const rows = terminal.rows;
        if (
          lastResizeRef.current?.cols === cols &&
          lastResizeRef.current?.rows === rows
        ) {
          return;
        }
        lastResizeRef.current = { cols, rows };
        socketRef.current.send(
          JSON.stringify({
            type: 'resize',
            cols,
            rows,
          }),
        );
      }
    },
    [],
  );

  const scheduleTerminalFit = useCallback(
    (stickToBottom = isTerminalAtBottom(terminalRef.current)) => {
      const run = () => fitTerminal(stickToBottom);
      window.requestAnimationFrame(() => {
        run();
        window.requestAnimationFrame(run);
      });
      window.setTimeout(run, 80);
    },
    [fitTerminal],
  );

  const toggleFullscreen = useCallback(async () => {
    const frame = terminalFrameRef.current;
    if (!frame || !connected) return;
    try {
      if (document.fullscreenElement === frame) {
        await document.exitFullscreen();
        return;
      }
      await frame.requestFullscreen();
      scheduleTerminalFit(true);
      focusTerminal();
    } catch (e: any) {
      setError(e?.message || tr('无法进入全屏', 'Unable to enter fullscreen'));
    }
  }, [connected, focusTerminal, scheduleTerminalFit, tr]);

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
        if (
          Date.now() - lastServerMessageAtRef.current >
          webSSHHeartbeatTimeoutMs
        ) {
          const text = tr(
            'WebSSH 连接超时，请重新连接',
            'WebSSH connection timed out, please reconnect',
          );
          setError(text);
          recordSessionEnd();
          flushTerminalOutput();
          terminalRef.current?.writeln(`\r\n${text}`, () => {
            terminalRef.current?.scrollToBottom();
          });
          setConnected(false);
          setConnecting(false);
          stopHeartbeat();
          socket.close();
          return;
        }
        socket.send(JSON.stringify({ type: 'ping' }));
      }, webSSHHeartbeatIntervalMs);
    },
    [
      flushTerminalOutput,
      markWebSSHAlive,
      recordSessionEnd,
      stopHeartbeat,
      tr,
    ],
  );

  const loadTarget = useCallback(async () => {
    if (!proxyId) {
      setError(shortSession ? tr('此会话已结束或不可访问，请返回访问列表重新连接。', 'This session has ended or is unavailable. Return to Access to reconnect.') : tr('访问 ID 无效', 'Invalid entry ID'));
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const res = await getWebSSHTarget(proxyId);
      if (res.code === 200 && res.data) {
        setTarget(res.data);
        const savedCredentials = res.data.credentials || [];
        if (credentialId > 0) {
          const selected = savedCredentials.find(
            (item) => item.id === credentialId,
          );
          if (selected?.username) {
            setCredentials({
              username: selected.username,
              password: '',
              save_credential: false,
            });
          } else {
            setError(
              tr('保存的 SSH 连接不存在或已删除', 'Saved SSH connection no longer exists'),
            );
            return;
          }
        } else if (!isTemporarySession) {
          setCredentials({ username: '', password: '', save_credential: false });
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
  }, [credentialId, isTemporarySession, proxyId, tr]);

  useEffect(() => {
    loadTarget();
  }, [loadTarget]);

  useEffect(() => {
    if (
      isTemporarySession &&
      !loading &&
      active &&
      !connected &&
      !initialCredentialPromptRef.current
    ) {
      initialCredentialPromptRef.current = true;
      setCredentialOpen(true);
    }
  }, [active, connected, isTemporarySession, loading]);
  useEffect(() => {
    if (!isTerminalView) return;
    document.body.classList.add('webssh-page-active');
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
      document.body.classList.remove('webssh-page-active');
      previousFooterStyles.forEach(({ element, display, pointerEvents }) => {
        element.style.display = display;
        element.style.pointerEvents = pointerEvents;
      });
    };
  }, [isTerminalView]);

  useEffect(() => {
    if (!isTerminalView || !terminalHostRef.current || terminalRef.current) return;
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
    const completionController = attachTerminalCompletion(terminal, setCompletion, sendTerminalInput, async (text, _revision, signal) => {
      const { handle, editor, allowed, contextMode } = completionBinding.current;
      if (!handle || !allowed) throw new Error('AI access unavailable');
      const revision = ++completionRevision.current;
      const shell = await ensureShellSession(handle);
      if (signal.aborted || completionBinding.current.handle !== handle || !completionBinding.current.allowed) throw new Error('Stale AI suggestion');
      const response = await request<API.Response<{text: string; revision: number}>>('/api/v1/assistance/suggestions', {
        method: 'POST', signal, skipErrorHandler: true,
        data: { agent_session_id: shell.session.id, handle_id: handle, editor_id: editor, revision, text, cursor: new TextEncoder().encode(text).length,
          ...(contextMode !== 'none' ? {context_mode:contextMode} : {}) },
      });
      if (response.data?.revision !== revision || completionBinding.current.handle !== handle
        || !completionBinding.current.allowed || completionBinding.current.contextMode !== contextMode) throw new Error('Stale AI suggestion');
      return response.data?.text || '';
    }, setCompletionStatus, setShellExitCode);
    completionRef.current = completionController;
    completionController.setAutomatic(completionBinding.current.allowed);
    terminal.onData(sendTerminalInput);
    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;
    fitTerminal(true);

    const handleResize = () => {
      scheduleTerminalFit(isTerminalAtBottom(terminal));
    };
    window.addEventListener('resize', handleResize);
    const resizeObserver = new ResizeObserver(() => {
      scheduleTerminalFit(isTerminalAtBottom(terminal));
    });
    resizeObserver.observe(terminalHostRef.current);
    if (terminalFrameRef.current) {
      resizeObserver.observe(terminalFrameRef.current);
    }
    return () => {
      window.removeEventListener('resize', handleResize);
      resizeObserver.disconnect();
      stopHeartbeat();
      if (outputFlushTimerRef.current !== undefined) {
        window.clearTimeout(outputFlushTimerRef.current);
        outputFlushTimerRef.current = undefined;
      }
      socketRef.current?.close();
      completionController.dispose();
      completionRef.current = undefined;
      terminal.dispose();
      terminalRef.current = undefined;
      fitAddonRef.current = undefined;
    };
  }, [fitTerminal, isTerminalView, scheduleTerminalFit, sendTerminalInput, stopHeartbeat]);

  const disconnect = useCallback(() => {
    recordSessionEnd();
    pendingCredentialSaveRef.current = false;
    lastResizeRef.current = undefined;
    flushTerminalInput();
    flushTerminalOutput();
    stopHeartbeat();
    socketRef.current?.close();
    socketRef.current = undefined;
    setConnected(false);
    setConnecting(false);
    setAgentOpen(false);
    setAgentHandleID('');
  }, [flushTerminalInput, flushTerminalOutput, recordSessionEnd, stopHeartbeat]);

  const disconnectAndSummarize = useCallback(() => {
    disconnect();
  }, [disconnect]);

  useEffect(() => {
    if (connected) {
      scheduleTerminalFit(true);
      focusTerminal();
    }
  }, [connected, focusTerminal, scheduleTerminalFit]);

  useEffect(() => {
    const handleFullscreenChange = () => {
      const active = document.fullscreenElement === terminalFrameRef.current;
      setFullscreen(active);
      scheduleTerminalFit(true);
      focusTerminal();
    };
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
    };
  }, [focusTerminal, scheduleTerminalFit]);

  const connect = async (values: API.CreateWebSSHSessionRequest) => {
    if (!target || !active) return;
    disconnect();
    setSessionDurationSeconds(null);
    setCredentialOpen(false);
    setConnecting(true);
    setError('');
    fitTerminal(true);
    terminalRef.current?.reset();
    terminalRef.current?.writeln(
      tr('正在连接 SSH...', 'Connecting to SSH...'),
      () => {
        terminalRef.current?.scrollToBottom();
      },
    );
    try {
      const password = values.password || '';
      const shouldSaveCredential = Boolean(values.save_credential && password);
      pendingCredentialSaveRef.current = shouldSaveCredential;
      const res = await createWebSSHSession(proxyId, {
        username: values.username?.trim(),
        password,
        save_credential: shouldSaveCredential,
        use_saved_credential: Boolean(credentialId > 0 && !password),
        cols: terminalRef.current?.cols,
        rows: terminalRef.current?.rows,
      });
      setCredentials((value) => ({ ...value, password: '' }));
      if (res.code !== 200 || !res.data?.ws_url) {
        throw new Error(
          res.message ||
            tr('创建 WebSSH 会话失败', 'Failed to create WebSSH session'),
        );
      }
      setAgentHandleID(res.data.token);
      setConnectionReferenceHandle(res.data.token);
      const socket = new WebSocket(res.data.ws_url);
      lastResizeRef.current = undefined;
      socketRef.current = socket;
      socket.onopen = () => {
        startHeartbeat(socket);
        fitTerminal(true);
        focusTerminal();
      };
      socket.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          markWebSSHAlive();
          if (msg.type === 'output') {
            writeTerminalOutput(msg.data || '');
          } else if (msg.type === 'pong') {
            return;
          } else if (msg.type === 'credential_saved') {
            pendingCredentialSaveRef.current = false;
            loadTarget();
          } else if (msg.type === 'credential_error') {
            pendingCredentialSaveRef.current = false;
            setError(msg.message || tr('SSH 已连接，但保存密码失败', 'SSH connected, but saving password failed'));
          } else if (msg.type === 'status') {
            if (msg.status === 'connected') {
              sessionStartedAtRef.current = Date.now();
              setSessionDurationSeconds(null);
              setConnected(true);
              setConnecting(false);
              setCredentialOpen(false);
              scheduleTerminalFit(true);
              focusTerminal();
            }
            if (msg.status === 'closed') {
              recordSessionEnd();
              pendingCredentialSaveRef.current = false;
              setConnected(false);
              setConnecting(false);
              setAgentOpen(false);
              setAgentHandleID('');
            }
          } else if (msg.type === 'error') {
            pendingCredentialSaveRef.current = false;
            const text =
              msg.message || tr('SSH 连接失败', 'SSH connection failed');
            setError(text);
            flushTerminalOutput();
            terminalRef.current?.writeln(`\r\n${text}`, () => {
              terminalRef.current?.scrollToBottom();
            });
            setConnected(false);
            setConnecting(false);
            setCredentialOpen(true);
            socket.close();
          }
        } catch {
          writeTerminalOutput(String(event.data || ''));
        }
      };
      socket.onerror = () => {
        recordSessionEnd();
        pendingCredentialSaveRef.current = false;
        stopHeartbeat();
        setError(tr('WebSSH 连接异常', 'WebSSH connection error'));
        setConnected(false);
        setConnecting(false);
        setAgentOpen(false);
        setAgentHandleID('');
        setCredentialOpen(true);
      };
      socket.onclose = () => {
        recordSessionEnd();
        flushTerminalInput();
        flushTerminalOutput();
        stopHeartbeat();
        setConnected(false);
        setConnecting(false);
        setAgentOpen(false);
        setAgentHandleID('');
      };
    } catch (e: any) {
      pendingCredentialSaveRef.current = false;
      const text =
        e?.response?.data?.message ||
        e?.message ||
        tr('创建 WebSSH 会话失败', 'Failed to create WebSSH session');
      setError(text);
      flushTerminalOutput();
      terminalRef.current?.writeln(`\r\n${text}`, () => {
        terminalRef.current?.scrollToBottom();
      });
      setConnecting(false);
      setCredentialOpen(true);
    }
  };

  useEffect(() => {
    if (
      !pendingSessionCredentials ||
      !isTemporarySession ||
      loading ||
      !active ||
      !terminalRef.current
    ) {
      return;
    }
    const values = pendingSessionCredentials;
    setPendingSessionCredentials(undefined);
    void connect(values);
  }, [active, isTemporarySession, loading, pendingSessionCredentials]);

  const resetHostKey = async () => {
    try {
      await deleteWebSSHHostKey(proxyId);
      loadTarget();
    } catch (e: any) {
      setError(e?.response?.data?.message || tr('重置失败', 'Reset failed'));
    }
  };

  const clearCredential = async () => {
    const username = String(credentials.username || '').trim();
    if (!username) {
      setError(tr('请选择保存用户', 'Select a saved user'));
      return;
    }
    try {
      await deleteWebSSHCredential(proxyId, username);
      setCredentials((value) => ({ ...value, password: '', save_credential: false }));
      loadTarget();
    } catch (e: any) {
      setError(e?.response?.data?.message || tr('清除保存密码失败', 'Failed to clear saved password'));
    }
  };

  useEffect(() => {
    if (
      !isTerminalView ||
      sessionPath.connectionId ||
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
    const saved = target?.credentials?.find((item) => item.id === credentialId);
    if (!saved?.username) return;
    initialSavedConnectRef.current = credentialId;
    void connect({
      username: saved.username,
      password: '',
      save_credential: false,
    });
  }, [active, connected, connecting, credentialId, isTemporarySession, isTerminalView, loading, target]);

  const isNewCredentialFlow = !isTerminalView || isTemporarySession;
  const submitCredentialConnection = () => {
    const username = credentials.username?.trim();
    if (!username) {
      setError(tr('请输入用户名', 'Username is required'));
      return;
    }
    if (!selectedSavedCredential && !credentials.password) {
      setError(tr('请输入密码', 'Password is required'));
      return;
    }
    const values = { ...credentials, username };
    if (!isTerminalView) {
      initialCredentialPromptRef.current = true;
      setPendingSessionCredentials(values);
      history.push(temporarySessionPath);
      return;
    }
    void connect(values);
  };

  const credentialModal = (
    <Modal
      title={isNewCredentialFlow
        ? tr('新建 SSH 连接', 'New SSH Connection')
        : tr('SSH 连接', 'SSH Connection')}
      open={credentialOpen}
      width={400}
      className="webssh-credential-modal"
      closeOnMask={!connecting}
      onClose={closeCredentialModal}
    >
      {error && (
        <Notice tone="danger">{error}{error.includes('指纹变化') ? <Button variant="danger" onClick={resetHostKey}>{tr('重置指纹', 'Reset fingerprint')}</Button> : null}</Notice>
      )}

      <form className="webssh-native-form" onSubmit={(event) => { event.preventDefault(); submitCredentialConnection(); }}>
        <Field label={tr('用户名', 'Username')} required>
          <Input autoFocus autoComplete="username" value={credentials.username || ''} onChange={(event) => setCredentials((value) => ({ ...value, username: event.target.value }))} placeholder={tr('输入 SSH 用户名', 'Enter SSH username')} />
        </Field>
        <Field label={tr('密码', 'Password')} required={isNewCredentialFlow}>
          <Input type="password" autoComplete="new-password" value={credentials.password || ''} onChange={(event) => { const password = event.target.value; setCredentials((value) => ({ ...value, password })); }} placeholder={selectedSavedCredential ? '••••••••' : tr('输入 SSH 密码', 'Enter SSH password')} />
        </Field>

        <div className="webssh-credential-options">
          {selectedSavedCredential && !credentials.password ? (
            <span className="webssh-credential-saved"><Check size={13} />{tr('密码已保存', 'Password saved')}</span>
          ) : (
            <label className="liaison-checkbox"><input type="checkbox" checked={Boolean(credentials.save_credential)} onChange={(event) => setCredentials((value) => ({ ...value, save_credential: event.target.checked }))} /><span>
                {selectedSavedCredential
                  ? tr('更新保存密码', 'Update saved password')
                  : tr('保存密码', 'Save password')}
              </span></label>
          )}
          {selectedSavedCredential && (
              <Button variant="ghost" disabled={connecting || connected} onClick={() => { if (window.confirm(tr('清除当前用户保存密码？', 'Clear saved password for this user?'))) void clearCredential(); }}>
                {tr('清除保存密码', 'Clear saved password')}
              </Button>
          )}
        </div>

        <div className="webssh-credential-actions">
          <Button
            onClick={closeCredentialModal}
            disabled={connecting}
          >
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

  if (!isTerminalView) {
    const savedCredentials = target?.credentials || [];
    return (
      <section className="webssh-connections-page">
        <div className="webssh-connections-header">
          <button
            type="button"
            className="webssh-connections-back"
            onClick={returnToAccess}
          >
            <ArrowLeft size={14} />
            {tr('返回访问', 'Back to access')}
          </button>
          <nav className="webssh-connections-breadcrumb" aria-label={tr('页面层级', 'Breadcrumb')}>
            <span>{tr('访问', 'Access')}</span><i>/</i>
            <span>Web SSH</span><i>/</i>
            <strong>{target?.proxy_name || tr('连接', 'Connections')}</strong>
          </nav>
          {target && (
            <span className="webssh-connections-target">
              {target.target_host}:{target.target_port}
            </span>
          )}
        </div>

        <div className="webssh-connections-heading">
          <div>
            <h2>{tr('SSH 连接', 'SSH Connections')}</h2>
            <p>
              {savedCredentials.length
                ? tr('选择已保存连接可直接进入终端，也可以新建连接。', 'Choose a saved connection to enter the terminal, or create one.')
                : tr('还没有保存的连接，先新建一个连接配置。', 'No saved connection yet. Create one first.')}
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
          <div className="webssh-connections-loading">
            <span className="webssh-native-spinner" />
          </div>
        ) : !active ? (
          <Notice tone="warning">
            {target?.effective_status_message ||
              tr('当前 WebSSH 访问不可用', 'WebSSH entry unavailable')}
          </Notice>
        ) : savedCredentials.length ? (
          <div className="webssh-connection-list">
            {savedCredentials.map((credential) => (
              <article className="webssh-connection-card" key={credential.id}>
                <div className="webssh-connection-card-identity">
                  <span className="webssh-terminal-mark" aria-hidden="true">&gt;_</span>
                  <span>{tr('用户：', 'User:')}</span>
                  <strong>{credential.username}</strong>
                </div>
                <div className="webssh-connection-card-meta">
                  <span className="webssh-connection-protocol">SSH</span>
                  <span>{tr('应用：', 'Application:')}{target?.application_name || '-'}</span>
                  <span>{tr('目标：', 'Target:')}{target ? `${target.target_host}:${target.target_port}` : '-'}</span>
                  <span className="webssh-connection-password-state">
                    <Check size={12} />
                    {credential.saved
                      ? tr('密码已保存', 'Password saved')
                      : tr('密码未保存', 'Password not saved')}
                  </span>
                </div>
                <div className="webssh-connection-card-used">
                  <span>{tr('最近使用：', 'Last used:')}</span>
                  <time>
                    <Clock3 size={13} />
                    {credential.last_used_at
                      ? formatWebSSHTime(credential.last_used_at)
                      : tr('尚未使用', 'Never')}
                  </time>
                </div>
                <div className="webssh-connection-card-actions">
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
                      if (!window.confirm(tr(`删除 ${credential.username} 的保存连接？`, `Delete the saved connection for ${credential.username}?`))) return;
                      setCredentials({ username: credential.username || '', password: '', save_credential: false });
                      void deleteWebSSHCredential(proxyId, credential.username).then(loadTarget);
                    }}
                  >
                    <Trash2 size={14} />
                  </Button>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <div className="webssh-connections-empty">
            <span className="webssh-terminal-mark" aria-hidden="true">&gt;_</span>
            <strong>{tr('还没有保存的 SSH 连接', 'No saved SSH connections')}</strong>
            <p>{tr('新建连接后可以选择保存密码，之后直接进入终端。', 'Save credentials when creating a connection to enter the terminal directly next time.')}</p>
            <Button variant="primary" onClick={openNewConnection}>
              <Plus size={15} />
              {tr('新建连接', 'New connection')}
            </Button>
          </div>
        )}
        {credentialModal}
      </section>
    );
  }

  return (
    <>
      <SessionPathNotice show={!!sessionPath.connectionId && !agentHandleID && !connecting} href={sessionPath.reconnectURL} />
      <div className={`webssh-shell ${credentialOpen && !connected ? 'is-credential-setup' : ''}`}>
        <header className="webssh-toolbar">
          <div className="webssh-identity">
            <Button
              className="webssh-back-button"
              variant="ghost"
              aria-label={tr('返回连接', 'Back to connections')}
              onClick={returnToConnections}
            ><ArrowLeft size={16} /></Button>
            <span className="webssh-terminal-mark" aria-hidden="true">
              &gt;_
            </span>
            <div>
              <strong>
                {target?.proxy_name || tr('WebSSH 会话', 'WebSSH session')}
                <SessionInfo connectionId={sessionPath.connectionId} handle={connectionReferenceHandle} agentId={sessionPath.agentSessionId}/>
              </strong>
              <span>{tr('安全终端', 'Secure terminal')}</span>
            </div>
          </div>

          <div className="webssh-session-meta">
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
                className={`webssh-status ${
                  connected
                    ? 'is-connected'
                    : connecting
                    ? 'is-connecting'
                    : active
                    ? 'is-ready'
                    : 'is-error'
                }`}
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

          <div className="webssh-toolbar-actions">
            {canAI && connected && agentHandleID && (
              <Button onClick={sessionPath.toggleAgent}>
                <Sparkles size={14} />
                Agent
              </Button>
            )}
            {!connected && (
              <Button
                variant="primary"
                disabled={!active || loading || connecting}
                onClick={() => setCredentialOpen(true)}
              >
                <Send size={14} />
                {connecting
                  ? tr('连接中', 'Connecting')
                  : tr('连接', 'Connect')}
              </Button>
            )}
            <Button
              disabled={!connected}
              onClick={toggleFullscreen}
            >
              {fullscreen ? <Minimize size={14} /> : <Fullscreen size={14} />}
              {fullscreen
                ? tr('退出全屏', 'Exit fullscreen')
                : tr('全屏', 'Fullscreen')}
            </Button>
            {connected && (
              <Button
                onClick={disconnectAndSummarize}
              >
                <PlugZap size={14} />
                {tr('断开', 'Disconnect')}
              </Button>
            )}
          </div>
        </header>

        {loading && (
          <div className="webssh-loading">
            <span className="webssh-native-spinner" />
          </div>
        )}
        {!loading && !active && (
          <Notice tone="warning">{target?.effective_status_message || tr('当前 SSH 访问不可用', 'SSH entry unavailable')}</Notice>
        )}
        {!loading && error && !credentialOpen && (
          <div className="webssh-alert"><Notice tone="danger">{error}{error.includes('指纹变化') ? <Button variant="danger" onClick={resetHostKey}>{tr('重置指纹', 'Reset fingerprint')}</Button> : null}</Notice></div>
        )}

        <div
          className="webssh-terminal"
          ref={terminalFrameRef}
          tabIndex={0}
          onClick={() => focusTerminal(true)}
          onMouseDown={() => focusTerminal(true)}
        >
          <div className="webssh-terminal-screen" ref={terminalHostRef} />
          {connected && completion && (
            <CommandCompletion view={completion} onAccept={candidate=>completionRef.current?.accept(candidate)}/>
          )}
          {connecting && !connected && !loading && (
            <div className="webssh-connecting-state" role="status" aria-live="polite">
              <span className="webssh-connecting-spinner" aria-hidden="true" />
              <strong>{tr('正在建立安全连接', 'Establishing secure connection')}</strong>
              <p>
                <span>{credentials.username || '-'}</span>
                <i aria-hidden="true">→</i>
                <span>{target ? `${target.target_host}:${target.target_port}` : '-'}</span>
              </p>
              <small>{tr('正在验证凭据并初始化终端', 'Verifying credentials and initializing terminal')}</small>
            </div>
          )}
          {!connected && !connecting && !loading && (
            <div className="webssh-terminal-empty">
              {sessionDurationSeconds !== null ? (
                <>
                  <span className="webssh-session-finished-icon" aria-hidden="true">
                    <Clock3 size={22} />
                  </span>
                  <strong>{tr('本次会话已结束', 'Session ended')}</strong>
                  <p className="webssh-session-duration">
                    {tr('使用时长', 'Duration')}
                    <b>{formatSessionDuration(sessionDurationSeconds)}</b>
                  </p>
                  <div className="webssh-session-end-actions">
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
                  <span className="webssh-terminal-mark" aria-hidden="true">
                    &gt;_
                  </span>
                  <strong>{tr('终端尚未连接', 'Terminal disconnected')}</strong>
                  <p>
                    {tr(
                      '输入 SSH 凭据后开始安全会话',
                      'Enter SSH credentials to start a secure session',
                    )}
                  </p>
                  <Button
                    variant="primary"
                    disabled={!active}
                    onClick={() => setCredentialOpen(true)}
                  >
                    {tr('连接终端', 'Connect terminal')}
                  </Button>
                </>
              )}
            </div>
          )}
          <SessionWatermark lines={watermarkLines} />
        </div>
        {canAI && connected && agentHandleID && <section className="terminal-assistant">
          <ShellAgent key={agentHandleID} handleId={agentHandleID} initialDetail={shellDetail?.handle === agentHandleID ? shellDetail.detail : undefined} ensureSession={ensureShellSession} exitCode={shellExitCode} controls={<>
            <button type="button" aria-pressed={automaticAI} onClick={() => {
              focusTerminal(true); completionRef.current?.setAutomatic(!automaticAI); setAutomaticAI(!automaticAI);
            }}>{automaticAI ? tr('关闭自动提示', 'Disable automatic AI') : tr('开启自动提示', 'Enable automatic AI')}</button>
            <select aria-label={tr('AI 上下文共享', 'AI context sharing')} title={tr('补全复用 Agent 记忆；额外共享命令和输出需在此选择。点击分析会共享最近命令与输出。', 'Completion uses Agent memory; choose additional command/output sharing here. Analysis shares recent commands and output.')} value={contextMode} onChange={event=>{
              const mode=event.target.value as 'none'|'commands'|'output';
              completionRef.current?.reset();
              completionBinding.current.contextMode=mode; setContextMode(mode);
              completionRef.current?.setAutomatic(automaticAI);
            }}>
              <option value="none">{tr('草稿与 Agent 记忆', 'Draft & Agent memory')}</option>
              <option value="commands">{tr('共享目录与最近命令', 'Share directory & recent commands')}</option>
              <option value="output">{tr('同时共享最近输出', 'Also share recent output')}</option>
            </select>
          </>}/>
          <div className="terminal-assistant-help" role="status">
            {completionStatus === 'busy' ? tr('AI 正在生成…', 'AI is generating…')
              : completionStatus === 'empty' ? tr('AI 暂无建议，可补充输入后重试', 'No AI suggestion; add more input and retry')
              : completionStatus === 'error' ? tr('AI 提示暂不可用，请检查权限、模型配置或重试', 'AI unavailable; check permissions, model configuration or retry')
              : completionStatus === 'unavailable' ? tr('请在 Shell 提示符处输入命令，等待回显后重试', 'Type at a shell prompt and wait for remote echo before retrying')
              : tr('AI 自动提示会发送当前草稿 · Ctrl+Space 立即提示 · Tab 接受 · Esc 忽略 · 回车前请检查', 'Automatic AI sends the current draft · Ctrl+Space suggests · Tab accepts · Esc dismisses · Review before Enter')}
          </div>
        </section>}
        <AgentWorkspace
          docked
          open={sessionPath.agentOpen}
          accessSessionId={sessionPath.agentSessionId}
          connectionId={sessionPath.connectionId}
          accessId={proxyId}
          connectionAvailable={connected && sessionPath.matching}
          onSessionReady={sessionPath.onAgentSessionReady}
          handleId={agentHandleID}
          title={target?.proxy_name || tr('WebSSH 会话', 'WebSSH session')}
          protocol="WebSSH"
          onClose={sessionPath.closeAgent}
        />
      </div>

      {credentialModal}
    </>
  );
};

export default WebSSHPage;
