import AgentWorkspace from '@/components/AgentWorkspace';
import {HeaderUser} from '@/components/layout/HeaderUser';
import ModelSelector from '@/components/AgentWorkspace/ModelSelector';
import {useResourceMentions} from '@/components/AgentWorkspace/ResourceMentions';
import type {AgentModelSelection,AgentResourceReference} from '@/services/agent';
import { useI18n } from '@/i18n';
import { getApplicationList, getDeviceList, getEdgeList } from '@/services/api';
import { createManagementSession, getAgentStatus, listManagementSessions, runAgentTurn } from '@/services/agent';
import { ArrowUp, Cable, HardDrive, History, Layers, MessageSquare, Plus, Network } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import './index.less';

export default function ManagementAgent() {
  const { tr } = useI18n();
  const navigate = useNavigate();
  const { agentSessionId } = useParams();
  const [sessions, setSessions] = useState<API.AgentSession[]>([]);
  const [selected, setSelected] = useState<API.AgentSession>();
  const [draft, setDraft] = useState('');
  const [recoveredDraft,setRecoveredDraft]=useState<{prompt:string;references:AgentResourceReference[]}>();
  const [modelSelection,setModelSelection]=useState<AgentModelSelection>();
  const [error, setError] = useState('');
  const [creating, setCreating] = useState(false);
  const [enabled, setEnabled] = useState<boolean>();
  const [historyOpen, setHistoryOpen] = useState(false);
  const [resourcesEmpty, setResourcesEmpty] = useState(false);
  const input = useRef<HTMLTextAreaElement>(null);
  const mentions = useResourceMentions(input,draft,setDraft,enabled!==false&&!creating);
  const alive = useRef(true);
  const locked = useRef(false);
  useEffect(() => {
    setError('');
    setSelected(agentSessionId ? { id: agentSessionId, title: '', kind: 'management' } as API.AgentSession : undefined);
  }, [agentSessionId]);
  const startNewChat = () => {
    if (creating) return;
    setSelected(undefined);
    setRecoveredDraft(undefined);
    setDraft('');
    mentions.reset();
    setModelSelection(undefined);
    setError('');
    setHistoryOpen(false);
    navigate('/');
  };
  useEffect(() => {
    if (!selected && !historyOpen) input.current?.focus();
  }, [selected, historyOpen]);
  useEffect(() => {
    let active = true;
    void Promise.all([getEdgeList({ page_size: 1 }), getDeviceList({ page_size: 1 }), getApplicationList({ page_size: 1 })])
      .then(([edges, devices, apps]) => {
        if (active && edges.data && devices.data && apps.data) {
          setResourcesEmpty(!(edges.data.edges?.length || devices.data.devices?.length || apps.data.applications?.length));
        }
      }).catch(() => { /* An unavailable list is not evidence of an empty account. */ });
    return () => { active = false; };
  }, []);
  useEffect(() => {
    alive.current = true;
    void getAgentStatus().then(r => { if (alive.current) setEnabled(Boolean(r.data?.enabled)); })
      .catch((e: Error) => { if (alive.current) setError(e.message); });
    void listManagementSessions().then(items => { if (alive.current) setSessions(items); })
      .catch((e: Error) => { if (alive.current) setError(e.message); });
    return () => { alive.current = false; };
  }, []);
  const start = async (initialPrompt = draft) => {
    const prompt = initialPrompt.trim();
    if (!prompt || locked.current || !enabled) return;
    locked.current = true; setCreating(true); setError('');
    try {
      const response = await createManagementSession(Array.from(prompt).slice(0, 64).join(''));
      if (!response.data) throw new Error(tr('创建会话失败', 'Could not create session'));
      const session = response.data.session;
      if (!alive.current) return;
      setSessions(items => [session, ...items]); setSelected(session); setDraft('');
      navigate(`/agent/sessions/${session.id}`);
      // Submission runs once here, not in a mount effect (StrictMode/reopening
      // a conversation must never submit the initial prompt a second time).
      await runAgentTurn(session.id, prompt, modelSelection, initialPrompt===draft?mentions.references:undefined);
      mentions.reset();
    } catch (e) { if (alive.current) {setError((e as Error).message);setRecoveredDraft({prompt,references:initialPrompt===draft?mentions.references:[]});} }
    finally { locked.current = false; if (alive.current) setCreating(false); }
  };
  const presets = [
    { icon: Cable, label: tr('哪些连接器当前在线？', 'Which connectors are online?'), description: tr('查看连接状态', 'Review connection status'), prompt: tr('列出我可见的连接器和在线状态。', 'List my visible connectors and their online status.') },
    { icon: HardDrive, label: tr('有哪些数据库和存储可用？', 'Which databases and storage can I access?'), description: tr('查找数据库、缓存和文件入口', 'Find data, cache and file workspaces'), prompt: tr('查找我可见的数据库、缓存、S3、SMB 和 SFTP 访问，说明协议与启用状态，并提供返回的入口。未实际连接的不要声称连接正常。', 'Find my visible database, cache, S3, SMB and SFTP access entries. Show protocols, enabled states and returned entry paths. Do not claim live connectivity without testing.') },
    { icon: Layers, label: tr('我有哪些可以打开的工作区？', 'Which workspaces can I open?'), description: tr('按业务类型浏览访问', 'Browse access by business category'), prompt: tr('列出我可见的访问，按 Web、Database、Cache、Storage、Desktop、LLM、TCP、SSH/SFTP 分类，并说明如何打开。应用存在不等于已建立访问。', 'List my visible access entries grouped by Web, Database, Cache, Storage, Desktop, LLM, TCP and SSH/SFTP, and explain how to open them. An application alone does not imply an access exists.') },
    { icon: Network, label: tr('我能调用哪些模型？', 'Which models can I call?'), description: tr('查看模型、协议和我的 Token 用量', 'Review models, protocols and my token usage'), prompt: tr('查找我可见的 LLM 访问，查询可调用模型、客户端协议以及我最近 24 小时的 Token 用量。用量未确认时明确说明，不要查询或展示密钥。', 'Find my visible LLM accesses and inspect callable models, client protocols and my token usage over the last 24 hours. Mark unconfirmed usage explicitly; do not fetch or display key secrets.') },
  ];
  return <section className={`management-agent-page${historyOpen ? ' has-history' : ''}`}>
    <div className="management-agent-topbar">
      <div className="management-agent-actions">
        <button aria-expanded={historyOpen} aria-controls="management-history" onClick={() => setHistoryOpen(v => !v)}><History size={16} />{tr('历史会话', 'History')}</button>
        {selected && <button disabled={creating} onClick={startNewChat}><Plus size={16} />{tr('新会话', 'New chat')}</button>}
      </div>
      <HeaderUser />
    </div>
    <div className="management-agent-layout">
      <nav id="management-history" hidden={!historyOpen} className="management-agent-history" aria-label={tr('会话历史', 'Chat history')}>
        <h2>{tr('最近会话', 'Recent chats')}</h2>
        {!sessions.length && <p>{tr('发送第一条消息开始', 'Send a message to get started')}</p>}
        {sessions.map(session => <button key={session.id} aria-current={selected?.id === session.id ? 'page' : undefined} disabled={creating}
          onClick={() => { setRecoveredDraft(undefined); setSelected(session); setModelSelection(undefined); setError(''); setHistoryOpen(false); navigate(`/agent/sessions/${session.id}`); }}><MessageSquare size={14} /><span>{session.title}</span></button>)}
      </nav>
      <main className="management-agent-main">
        {error && <div className="management-agent-error" role="alert">{error}</div>}
        {selected ? <AgentWorkspace key={selected.id} open recoveredDraft={recoveredDraft} initialBusy={creating} initialModelSelection={modelSelection} managementSessionId={selected.id} title={selected.title}
          protocol={tr('资源管理', 'Resources')} onClose={startNewChat} /> :
          <div className="management-agent-welcome">
            <h1>{tr('今天想了解什么？', 'What would you like to explore?')}</h1>
            <p>{tr('查找访问入口，了解资源状态、可调用模型和 Token 用量。', 'Find access entries, resource status, available models and token usage.')}</p>
            <div className="management-agent-input">
              {mentions.tags}{mentions.picker}
              <textarea ref={input} value={draft} maxLength={16000} rows={3} aria-label={tr('消息', 'Message')}
                {...mentions.inputProps} onSelect={mentions.onSelect}
                onChange={e => mentions.onChange(e.target.value,e.target.selectionStart)} placeholder={tr('询问你的资源，输入 @ 引用连接器、设备或应用…', 'Ask about your resources. Type @ to mention one…')}
                onKeyDown={e => { if(mentions.onKeyDown(e))return; if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) { e.preventDefault(); void start(); } }} />
              <div><section className="agent-composer-options">{mentions.button}<ModelSelector value={modelSelection} onChange={setModelSelection} disabled={creating||enabled===false}/></section>
                <button aria-label={tr('发送', 'Send')} disabled={!draft.trim() || !enabled || creating} onClick={() => void start()}><ArrowUp size={18} /></button></div>
            </div>
            {resourcesEmpty ? <div className="management-agent-onboarding">
              <h2>{tr('还没有可见的资源', 'No resources yet')}</h2>
              <p>{tr('当前没有你可见的连接器、设备或应用。先添加连接器，接入设备和应用后，就可以在这里查询。', 'You have no visible connectors, devices or applications. Add a connector and connect your resources to explore them here.')}</p>
              <a href="/connector">{tr('前往连接器', 'Go to connectors')}<ArrowUp size={14} /></a>
            </div> : <div className="management-agent-presets">{presets.map(p => <button key={p.label} disabled={!enabled || creating} onClick={() => void start(p.prompt)}><span className="preset-title"><p.icon size={18} />{p.label}</span><span className="preset-description">{p.description}</span></button>)}</div>}
          </div>}
      </main>
    </div>
  </section>;
}
