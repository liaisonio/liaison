import {useEffect, useRef, useState, type ReactNode} from 'react';
import {Sparkles} from 'lucide-react';
import {useI18n} from '@/i18n';
import {getAgentSession, resolveAgentApproval, runAgentTurn} from '@/services/agent';
import {MessageContent, ToolMessage} from '@/components/AgentWorkspace/MessageContent';
import SessionInfo from '@/components/SessionReference/SessionInfo';
import './ShellAgent.less';

// Mounted with a live handle as its React key: never shares Sidepanel history.
export default function ShellAgent({handleId, exitCode, ensureSession, initialDetail, controls}: {handleId: string; exitCode?: number; ensureSession: (handle: string) => Promise<API.AgentSessionDetail>; initialDetail?: API.AgentSessionDetail; controls?: ReactNode}) {
  const {tr} = useI18n();
  const [detail, setDetail] = useState<API.AgentSessionDetail>();
  const [expanded, setExpanded] = useState(false);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const live = useRef(true);
  const lock = useRef(false);
  const session = useRef('');
  useEffect(() => {
    if (initialDetail && !session.current) {
      session.current = initialDetail.session.id; setDetail(initialDetail);
    }
  }, [initialDetail]);
  const results = useRef<HTMLDivElement>(null);
  useEffect(() => {live.current = true; return () => {live.current = false;};}, []);
  const refresh = async () => {
    if (!session.current) return;
    const result = await getAgentSession(session.current);
    if (live.current) setDetail(result.data);
  };
  const pending = detail?.approvals.filter(a => a.status === 0) || [];
  const active = detail?.turns.find(t => t.id === detail.session.active_turn_id);
  const running = !!active && ![3,4,5].includes(active.status);
  useEffect(() => {results.current?.scrollTo({top: results.current.scrollHeight});}, [detail?.messages.length, pending.length, expanded]);
  useEffect(() => {
    if (!running || pending.length) return;
    const timer = window.setInterval(() => {void refresh().catch(e => {if (live.current) setError(e.message);});}, 1500);
    return () => window.clearInterval(timer);
  }, [running, pending.length]);

  const perform = async (prompt?: string, approval?: {id: string; decision: 'approve'|'deny'}) => {
    if (lock.current || !live.current) return;
    lock.current = true; setBusy(true); setExpanded(true); setError('');
    try {
      if (!session.current) {
        const result = await ensureSession(handleId);
        if (!live.current) return;
        session.current = result.session.id; setDetail(result);
      }
      if (approval) await resolveAgentApproval(session.current, approval.id, approval.decision);
      else if (prompt) await runAgentTurn(session.current, prompt);
      await refresh();
      if (live.current && prompt) setDraft('');
    } catch (e: any) {
      if (live.current) setError(e.message || tr('分析失败', 'Analysis failed'));
      try {await refresh();} catch { /* Preserve the original error; refresh is best effort. */ }
    } finally {
      lock.current = false;
      if (live.current) setBusy(false);
    }
  };
  const analyze = () => perform(tr(
    '请读取当前 Shell 上下文与最近输出，简短分析最近命令的结果，给出下一步建议。不要执行命令；如需进一步诊断，先说明建议。',
    'Read the current Shell context and recent output. Briefly analyze the latest command result and suggest a next step. Do not execute commands; explain any recommended diagnosis first.',
  ));
  return <div className="shell-agent" aria-label="Shell Agent">
    <div className="shell-agent-toolbar">
      <strong><Sparkles size={14}/> Shell Agent</strong><SessionInfo agentId={detail?.session.id}/>
      {controls}
      {exitCode !== undefined && exitCode !== 0 && <span className="shell-agent-failure">{tr('命令退出码', 'Command exit code')} {exitCode}</span>}
      <button type="button" disabled={busy || running || !!pending.length} onClick={() => void analyze()}>{tr('分析最近结果', 'Analyze latest result')}</button>
      <button type="button" onClick={() => setExpanded(!expanded)}>{expanded ? tr('收起', 'Collapse') : tr('建议与诊断', 'Advice & diagnosis')}</button>
    </div>
    {expanded && <div className="shell-agent-content">
      <p className="shell-agent-scope">{tr('分析会将当前连接的最近命令与输出发送给模型。诊断在独立 SSH 通道执行，每条命令需确认，不会输入到当前终端。', 'Analysis shares recent commands and output with the model. Diagnosis runs in a separate SSH channel; every command requires approval and is never typed into this terminal.')}</p>
      <div className="shell-agent-results" aria-live="polite" ref={results}>
        {detail?.messages.filter(m => m.value.role !== 'system').slice(-12).map(m => <article key={m.id}>
          {m.value.role === 'tool' ? <ToolMessage name={m.value.tool_name || 'terminal'} content={m.value.content || ''}/> : m.value.content && <><small>{m.value.role === 'user' ? tr('你', 'You') : 'Shell Agent'}</small><MessageContent text={m.value.content}/></>}
        </article>)}
        {pending.map(a => <div className="shell-agent-approval" key={a.id}>
          <strong>{tr('确认诊断命令', 'Review diagnostic command')}</strong>
          <pre>{typeof a.input === 'object' && a.input !== null && 'command' in a.input ? String(a.input.command) : JSON.stringify(a.input, null, 2)}</pre>
          <button disabled={busy} onClick={() => void perform(undefined,{id:a.id,decision:'deny'})}>{tr('拒绝', 'Deny')}</button>
          <button disabled={busy} onClick={() => void perform(undefined,{id:a.id,decision:'approve'})}>{tr('允许一次', 'Allow once')}</button>
        </div>)}
        {(busy || running) && !pending.length && <p role="status">{tr('正在分析…', 'Analyzing…')}</p>}
        {error && <p role="alert">{error}</p>}
      </div>
      <form onSubmit={e => {e.preventDefault();if(draft.trim() && !busy && !running && !pending.length)void perform(draft.trim());}}>
        <input aria-label={tr('Shell 分析问题', 'Shell analysis question')} placeholder={tr('例如：为什么失败？帮我检查磁盘占用', 'Why did this fail? Help inspect disk usage')} value={draft} maxLength={8192} onChange={e => setDraft(e.target.value)}/>
        <button disabled={busy || running || !!pending.length || !draft.trim()}>{tr('分析', 'Analyze')}</button>
      </form>
    </div>}
  </div>;
}
