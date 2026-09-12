import { Button } from '@/components/ui';
import SessionReference from '@/components/SessionReference';
import SessionInfo from '@/components/SessionReference/SessionInfo';
import { connectionReference } from '@/components/SessionReference/useSessionPath';
import { useFeature } from '@/store/permissions';
import { useI18n } from '@/i18n';
import {
  createAgentSession,
  getAgentSession,
  getAgentStatus,
  resolveAgentApproval,
  runAgentTurn,
  streamAgentEvents,
} from '@/services/agent';
import { ArrowUp, Bot, Check, ShieldAlert, Sparkles, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import './index.less';
import { MessageContent, ToolMessage } from './MessageContent';
import ModelSelector from './ModelSelector';
import {ReferenceTags,useResourceMentions} from './ResourceMentions';
import type {AgentModelSelection,AgentResourceReference} from '@/services/agent';

type AgentWorkspaceProps = {
  recoveredDraft?: {prompt:string;references:AgentResourceReference[]};
  initialModelSelection?: AgentModelSelection;
  accessSessionId?: string;
  connectionId?: string;
  accessId?: number;
  connectionAvailable?: boolean;
  onSessionReady?: (id: string) => void;
  managementSessionId?: string;
  initialBusy?: boolean;
  open: boolean;
  handleId?: string;
  title: string;
  protocol: string;
  onClose: () => void;
  docked?: boolean;
  dockBreakpoint?: number;
};

const terminalTurnStatuses = new Set([3, 4, 5]);

const toolName = (tool?: API.AgentToolID) =>
  tool ? `${tool.namespace}.${tool.name}` : 'tool';

const displayInput = (input: unknown) => {
  if (input === undefined || input === null) return '';
  if (typeof input === 'string') return input;
  try {
    return JSON.stringify(input, null, 2);
  } catch {
    return String(input);
  }
};

export default function AgentWorkspace(props: AgentWorkspaceProps) {
  const allowed = useFeature(props.managementSessionId ? 'ai.home.use' : 'ai.access.use');
  return allowed ? <AgentWorkspaceContent {...props} /> : null;
}

function AgentWorkspaceContent({ open, handleId, title, protocol, onClose, docked = false, dockBreakpoint = 850, managementSessionId, initialBusy = false, accessSessionId, connectionId, accessId, connectionAvailable = true, onSessionReady, initialModelSelection, recoveredDraft }: AgentWorkspaceProps) {
  const { tr } = useI18n();
  const [compact, setCompact] = useState(() => window.matchMedia(`(max-width: ${dockBreakpoint}px)`).matches);
  useEffect(() => {
    const media = window.matchMedia(`(max-width: ${dockBreakpoint}px)`);
    const update = () => setCompact(media.matches);
    update();
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, [dockBreakpoint]);
  const embedded = docked && !compact;
  const riskLabels = [tr('只读', 'Read only'), tr('低风险', 'Low'), tr('需确认', 'Approval'), tr('高风险', 'High'), tr('高危', 'Critical')];
  const [detail, setDetail] = useState<API.AgentSessionDetail>();
  const [modelSelection,setModelSelection]=useState<AgentModelSelection|undefined>(initialModelSelection);
  const selectionRestored=useRef(false);
  useEffect(()=>{
    if(!detail||selectionRestored.current)return;
    const steps=[...detail.steps].reverse();
    for(const step of steps){
      const input=step.input as {model_selection?:AgentModelSelection}|undefined;
      if(input?.model_selection?.provider_id){setModelSelection(input.model_selection);break;}
    }
    selectionRestored.current=true;
  },[detail]);
  const [prompt, setPrompt] = useState('');
  const [streamText, setStreamText] = useState('');
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const bodyRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const mentions=useResourceMentions(inputRef,prompt,setPrompt,!!managementSessionId);
  useEffect(() => {
    const input = inputRef.current;
    if (!input) return;
    input.style.height = 'auto';
    input.style.height = `${Math.min(144, input.scrollHeight)}px`;
  }, [prompt, open]);
  const sessionIDRef = useRef('');
  const handleRef = useRef(handleId);
  handleRef.current = handleId;
  const pendingSessionRef = useRef<{ handle: string; promise: ReturnType<typeof createAgentSession> }>();

  const refresh = useCallback(async (sessionId = sessionIDRef.current) => {
    if (!sessionId) return;
    const response = await getAgentSession(sessionId);
    if (response.data && sessionIDRef.current === sessionId) setDetail(response.data);
  }, []);

  useEffect(() => {
    sessionIDRef.current = '';
    setDetail(undefined);
    setStreamText('');
    setError('');
    setSending(false);
    setPrompt('');
  }, [handleId]);

  useEffect(()=>{if(recoveredDraft){setPrompt(recoveredDraft.prompt);mentions.setReferences(recoveredDraft.references);}},[recoveredDraft]);

  useEffect(() => {
    const requestedID = managementSessionId || accessSessionId;
    if (!requestedID || !open) return;
    let active = true;
    sessionIDRef.current = requestedID;
    setDetail(undefined);
    setLoading(true);
    setError('');
    void getAgentSession(requestedID).then(async response => {
      if (!active) return;
      if (response.data?.session.kind !== (managementSessionId ? 'management' : 'access')) throw new Error(tr('会话类型不匹配', 'Session type mismatch'));
      if (accessSessionId) {
        const attachments = response.data.attachments.filter(a => a.access_id === accessId);
        const references = await Promise.all(attachments.map(a => connectionReference(a.id)));
        if (!connectionId || !references.includes(connectionId)) throw new Error(tr('对话不属于此连接会话', 'Chat does not belong to this connection'));
      }
      if (!active) return;
      setDetail(response.data);
    }).catch((reason: Error) => { if (active) setError(reason.message); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; sessionIDRef.current = ''; };
  }, [managementSessionId, accessSessionId, connectionId, accessId, handleId, open, tr]);

  useEffect(() => {
    if (!open || !handleId || accessSessionId) return;
    if (sessionIDRef.current) {
      let active = true;
      setLoading(true);
      void refresh().catch((reason: Error) => {
        if (active) setError(reason.message);
      }).finally(() => { if (active) setLoading(false); });
      return () => { active = false; };
    }
    let active = true;
    setLoading(true);
    setError('');
    if (pendingSessionRef.current?.handle !== handleId) {
      const promise = getAgentStatus().then((status) => {
        if (!status.data?.enabled) {
          throw new Error(tr('Agent 尚未启用，请联系管理员配置模型。', 'Agent is not enabled. Ask your administrator to configure a model.'));
        }
        return createAgentSession(handleId, title);
      });
      pendingSessionRef.current = { handle: handleId, promise };
    }
    pendingSessionRef.current.promise
      .then((response) => {
        if (handleRef.current !== handleId || !response.data) return;
        sessionIDRef.current = response.data.session.id;
        if (active) onSessionReady?.(response.data.session.id);
        if (active) setDetail(response.data);
      })
      .catch((reason: Error) => {
        if (pendingSessionRef.current?.handle === handleId) pendingSessionRef.current = undefined;
        if (active) setError(reason.message);
      })
      .finally(() => active && setLoading(false));
    return () => { active = false; };
  }, [handleId, open, title, refresh, tr, accessSessionId, onSessionReady]);

  useEffect(() => {
    if (open && connectionId && detail?.session.id && !accessSessionId) onSessionReady?.(detail.session.id);
  }, [open, connectionId, detail?.session.id, accessSessionId, onSessionReady]);

  useEffect(() => {
    if (!open || !detail?.session.id) return;
    const controller = new AbortController();
    let refreshTimer = 0;
    let reconnectTimer = 0;
    let retryDelay = 1000;
    const scheduleRefresh = () => {
      window.clearTimeout(refreshTimer);
      refreshTimer = window.setTimeout(() => void refresh().catch((reason: Error) => {
        if (!controller.signal.aborted) setError(reason.message);
      }), 120);
    };
    const connect = async () => {
      try {
        await streamAgentEvents(detail.session.id, controller.signal, (event) => {
          if (controller.signal.aborted) return;
          retryDelay = 1000;
          if (event.type === 'model.delta' && event.payload?.delta) {
            setStreamText((current) => current + event.payload!.delta);
            return;
          }
          if (event.type === 'tool.started' || event.type === 'turn.completed' || event.type === 'turn.failed') setStreamText('');
          scheduleRefresh();
        });
      } catch (reason: any) {
        if (controller.signal.aborted) return;
        if ([401, 403, 404].includes(reason?.response?.status)) {
          controller.abort();
          window.clearInterval(snapshotTimer);
          setError(reason.message);
          return;
        }
      }
      if (controller.signal.aborted) return;
      // Deltas are transient; recover authoritative messages from the snapshot.
      setStreamText('');
      scheduleRefresh();
      reconnectTimer = window.setTimeout(() => void connect(), retryDelay);
      retryDelay = Math.min(retryDelay * 2, 15000);
    };
    void connect();
    // Also covers events missed between fetching a snapshot and subscribing.
    const snapshotTimer = window.setInterval(scheduleRefresh, 5000);
    return () => {
      controller.abort();
      window.clearTimeout(refreshTimer);
      window.clearTimeout(reconnectTimer);
      window.clearInterval(snapshotTimer);
      setStreamText('');
    };
  }, [detail?.session.id, open, refresh]);

  useEffect(() => {
    bodyRef.current?.scrollTo({ top: bodyRef.current.scrollHeight, behavior: 'smooth' });
  }, [detail?.messages.length, streamText]);

  const activeTurn = useMemo(
    () => detail?.turns.find((turn) => turn.id === detail.session.active_turn_id),
    [detail],
  );
  const pendingApprovals = useMemo(
    () => (detail?.approvals || []).filter((approval) => approval.status === 0),
    [detail?.approvals],
  );
  const busy = initialBusy || sending || Boolean(activeTurn && !terminalTurnStatuses.has(activeTurn.status));

  const send = async () => {
    const value = prompt.trim();
    const sessionId = sessionIDRef.current;
    if (!value || !sessionId || busy || pendingApprovals.length > 0 || !connectionAvailable) return;
    setPrompt('');
    setSending(true);
    const references=mentions.references;
    mentions.reset();
    setStreamText('');
    setError('');
    try {
      await runAgentTurn(sessionId, value, managementSessionId ? modelSelection : undefined, managementSessionId ? references : undefined);
      await refresh(sessionId);
    } catch (reason: any) {
      if (sessionIDRef.current !== sessionId) return;
      setPrompt((draft) => draft || value);
      mentions.setReferences(current=>current.length?current:references);
      setError(reason?.response?.status === 503
        ? tr('Agent 模型尚未配置，请先在服务端启用。', 'Agent model is not configured on this server.')
        : reason?.message || tr('Agent 执行失败', 'Agent run failed'));
      await refresh(sessionId).catch(() => undefined);
    } finally {
      if (sessionIDRef.current === sessionId) {
        setSending(false);
        setStreamText('');
      }
    }
  };

  const decide = async (approval: API.AgentApproval, decision: 'approve' | 'deny') => {
    const sessionId = sessionIDRef.current;
    if (!sessionId || !connectionAvailable) return;
    setSending(true);
    setError('');
    try {
      await resolveAgentApproval(sessionId, approval.id, decision);
      await refresh(sessionId);
    } catch (reason: any) {
      if (sessionIDRef.current !== sessionId) return;
      setError(reason?.message || tr('审批处理失败', 'Failed to resolve approval'));
      await refresh(sessionId).catch(() => undefined);
    } finally {
      if (sessionIDRef.current === sessionId) {
        setSending(false);
        setStreamText('');
      }
    }
  };

  if (!open) return null;
  const workspace = (
    <div className={`agent-workspace-root${embedded ? ' is-docked' : ''}${managementSessionId ? ' is-management' : ''}`}>
      {!embedded && !managementSessionId && <button className="agent-workspace-mask" type="button" aria-label={tr('关闭 Agent', 'Close Agent')} onClick={onClose} />}
      {embedded && <div className="agent-workspace-divider" role="separator" aria-orientation="vertical" aria-label={tr('调整 Agent 宽度', 'Resize Agent')} tabIndex={0}
        onKeyDown={e => { if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return; e.preventDefault(); const host = e.currentTarget.parentElement?.parentElement; if (!host) return; const width = parseFloat(getComputedStyle(host).getPropertyValue('--agent-width')) || 420; host.style.setProperty('--agent-width', `${Math.max(300, Math.min(560, width + (e.key === 'ArrowLeft' ? 20 : -20)))}px`); }}
        onPointerDown={e => { e.currentTarget.setPointerCapture(e.pointerId); }}
        onPointerMove={e => { if (!e.currentTarget.hasPointerCapture(e.pointerId)) return; const host = e.currentTarget.parentElement?.parentElement; if (host) host.style.setProperty('--agent-width', `${Math.max(300, Math.min(560, host.getBoundingClientRect().right - e.clientX))}px`); }} />}
      <aside className="agent-workspace" aria-label={tr('Agent 工作区', 'Agent workspace')}>
        <header>
          <div className="agent-workspace-heading">
            {!managementSessionId && <span className="agent-workspace-icon"><Sparkles size={16} /></span>}
            <div><strong>{managementSessionId ? (detail?.session.title || title || tr('新会话', 'New conversation')) : 'Agent'}<SessionInfo agentId={detail?.session.id || managementSessionId || accessSessionId} connectionId={connectionId} handle={handleId}/></strong>{!managementSessionId && <span>{protocol} · {title || detail?.session.title}</span>}
            </div>
          </div>
          <button type="button" onClick={onClose} aria-label={tr('关闭', 'Close')}><X size={18} /></button>
        </header>
        <div className="agent-workspace-body" ref={bodyRef}>
          {loading ? <div className="agent-workspace-state"><span className="ui-spinner" />{tr('正在准备上下文…', 'Preparing context…')}</div> : null}
          {!loading && !detail && !error ? <div className="agent-workspace-state">{tr('当前连接不可用于 Agent。', 'Agent is unavailable for this connection.')}</div> : null}
          {detail && detail.messages.filter((message) => message.value.role !== 'system').length === 0 ? (
            <div className="agent-workspace-empty">
              <Bot size={24} />
              <strong>{managementSessionId ? tr('从你的资源开始', 'Start with your resources') : tr('从当前连接开始', 'Work with this connection')}</strong>
              <p>{managementSessionId ? tr('查询你有权限查看的连接器、设备和应用。', 'Explore connectors, devices and applications you have access to.') : tr('Agent 只会看到当前协议允许披露的工具和上下文。执行敏感操作前会请求你的确认。', 'Agent only sees tools and context disclosed for this connection. Sensitive actions require your approval.')}</p>
            </div>
          ) : null}
          {detail?.messages.filter((message) => message.value.role !== 'system').map((message) => (
            <article key={message.id} className={`agent-message is-${message.value.role}`}>
              {message.value.role === 'tool' ? <ToolMessage name={message.value.tool_name || 'tool'} content={message.value.content || ''} /> : <>
                <span>{message.value.role === 'user' ? tr('你', 'You') : 'Agent'}</span>
                {message.value.references && <ReferenceTags references={message.value.references}/>}
                {message.value.content && <MessageContent text={message.value.content} />}
              </>}
            </article>
          ))}
          {streamText ? <article className="agent-message is-assistant is-streaming"><span>Agent</span><MessageContent text={streamText} /></article> : null}
          {pendingApprovals.map((approval) => (
            <article className="agent-approval" key={approval.id}>
              <div className="agent-approval-title"><ShieldAlert size={16} /><strong>{tr('需要确认', 'Approval required')}</strong><span>{riskLabels[approval.risk] || tr('未知风险', 'Unknown risk')}</span></div>
              <p>{tr('将在当前连接执行以下操作，请确认。', 'Review this operation before running it on this connection.')}</p>
              <div className="agent-tool-name">{toolName(approval.tool_id)}</div>
              {approval.input !== undefined ? <pre>{displayInput(typeof approval.input === 'object' && approval.input !== null ? ('command' in approval.input ? approval.input.command : 'statement' in approval.input ? approval.input.statement : approval.input) : approval.input)}</pre> : null}
              <div className="agent-approval-actions">
                <Button disabled={sending || !connectionAvailable} onClick={() => void decide(approval, 'deny')}>{tr('拒绝', 'Deny')}</Button>
                <Button variant="primary" disabled={sending || !connectionAvailable} onClick={() => void decide(approval, 'approve')}><Check size={14} />{tr('允许一次', 'Allow once')}</Button>
              </div>
            </article>
          ))}
          {busy && !streamText ? <div className="agent-workspace-state"><span className="ui-spinner" />{tr('Agent 正在处理…', 'Agent is working…')}</div> : null}
          {error || detail?.turns.at(-1)?.status === 5 ? <div className="agent-workspace-error">
            {error.startsWith('agent connection unavailable;')
              ? tr('连接已断开或不可用，请重新连接并开启新的 Agent 会话。', 'Connection unavailable. Reconnect and start a new Agent session.')
              : error || tr('本轮处理失败，可以重新提问。', 'This turn failed. You can try again.')}
            <SessionReference id={detail?.turns.at(-1)?.id} label={tr('轮次', 'Turn')} />
          </div> : null}
        </div>
        <footer>
          <div className="agent-composer">
          {managementSessionId&&<>{mentions.tags}{mentions.picker}</>}
          <textarea
            ref={inputRef}
            aria-label={tr('消息', 'Message')}
            value={prompt}
            {...(managementSessionId?mentions.inputProps:{})}
            onSelect={managementSessionId?mentions.onSelect:undefined}
            onChange={(event) => managementSessionId?mentions.onChange(event.target.value,event.target.selectionStart):setPrompt(event.target.value)}
            onKeyDown={(event) => {
              if(managementSessionId&&mentions.onKeyDown(event))return;
              if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
                event.preventDefault();
                void send();
              }
            }}
            placeholder={managementSessionId ? tr('继续询问你的资源…', 'Ask about your resources…') : tr('询问当前连接，或描述要执行的操作…', 'Ask about this connection or describe an action…')}
            rows={2}
          />
          <div className="agent-composer-toolbar">
            {managementSessionId ? <section className="agent-composer-options">{mentions.button}<ModelSelector value={modelSelection} onChange={value=>{selectionRestored.current=true;setModelSelection(value);}} disabled={busy||initialBusy||pendingApprovals.length>0}/></section> : <span>{busy ? tr('可继续编写下一条消息', 'You can draft your next message') : tr('Shift + Enter 换行', 'Shift + Enter for a new line')}</span>}
            <Button variant="primary" aria-label={tr('发送', 'Send')} disabled={!connectionAvailable || !prompt.trim() || !detail || busy || pendingApprovals.length > 0} onClick={() => { inputRef.current?.focus(); void send(); }}><ArrowUp size={16} /></Button>
          </div>
          </div>
        </footer>
      </aside>
    </div>
  );
  return docked && compact ? createPortal(workspace, document.body) : workspace;
}
