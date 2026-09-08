import AgentWorkspace from '@/components/AgentWorkspace';
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
    { icon: HardDrive, label: tr('我的设备都运行什么系统？', 'What systems do my devices run?'), description: tr('了解设备信息', 'Explore device details'), prompt: tr('列出我可见的设备及操作系统。', 'List my visible devices and their operating systems.') },
    { icon: Layers, label: tr('我有哪些可以访问的应用？', 'What applications can I access?'), description: tr('浏览应用与协议', 'Browse applications and protocols'), prompt: tr('列出我可见的应用及其协议和目标地址。', 'List my visible applications, protocols and targets.') },
    { icon: Network, label: tr('应用分别指向哪些服务？', 'Where do my applications connect?'), description: tr('梳理服务地址', 'Review service targets'), prompt: tr('列出我可见应用的名称、协议、目标 IP 和端口，缺失的信息不要推测。', 'List the names, protocols, target IPs and ports of my visible applications. Do not infer missing information.') },
  ];
  return <section className={`management-agent-page${historyOpen ? ' has-history' : ''}`}>
    <div className="management-agent-topbar">
      <div className="management-agent-actions">
        <button aria-expanded={historyOpen} aria-controls="management-history" onClick={() => setHistoryOpen(v => !v)}><History size={16} />{tr('历史会话', 'History')}</button>
        {selected && <button disabled={creating} onClick={startNewChat}><Plus size={16} />{tr('新会话', 'New chat')}</button>}
      </div>
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
            <p>{tr('从一个问题开始，了解你的连接器、设备和应用。', 'Ask a question about your connectors, devices and applications.')}</p>
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
            <small>{tr('当前支持资源查询，不会修改配置或执行终端命令。', 'Resource queries only. No configuration changes or terminal commands.')}</small>
          </div>}
      </main>
    </div>
  </section>;
}
