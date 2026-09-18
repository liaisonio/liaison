import {useState} from 'react';
import {useLocation, useNavigate} from 'react-router-dom';
import {Button, Field, Modal, Notice} from '@/components/ui';
import {accessTypeLabel, getProxyAccessType, isLLMAccessType, isWebAccessType} from '@/constants/accessTypes';
import {getProxyList} from '@/services/api';
import {AccessConfigurationRequired, directAccessPath} from '@/pages/Proxy/connection';
import {useI18n} from '@/i18n';
import {stageAccessDraft} from './handoff';

type Entry = {id: string; name: string; type: string; state: string};
export function accessResults(content: string): Entry[] {
  try {
    const result = JSON.parse(content);
    if (result.IsError || result.is_error) return [];
    let value = result.Content ?? result.content ?? result;
    if (typeof value === 'string') value = JSON.parse(value);
    if (!Array.isArray(value?.items)) return [];
    return value.items.filter((item: Entry) => item && /^[1-9][0-9]*$/.test(item.id) && Number.isSafeInteger(Number(item.id)) && typeof item.name === 'string' && typeof item.type === 'string' && typeof item.state === 'string').slice(0, 50);
  } catch { return []; }
}
const supportsDraft = (type: string) => isWebAccessType(type) && !isLLMAccessType(type) && !['websftp','websmb'].includes(type);

export default function AccessResults({content, question}: {content: string; question: string}) {
  const {tr} = useI18n(), navigate = useNavigate(), location = useLocation();
  const [selected, setSelected] = useState<Entry>(), [prompt, setPrompt] = useState(''), [carry, setCarry] = useState(false);
  const [opening, setOpening] = useState(false), [error, setError] = useState(''), [configure, setConfigure] = useState(false);
  const entries = accessResults(content);
  const select = async (entry: Entry) => {
    setError(''); setConfigure(false);
    // Historical tool messages used "web". Resolve against current scoped data,
    // never guess a protocol from the name or rewrite persisted chat history.
    if (entry.type === 'web') {
      setOpening(true);
      try {
        const response = await getProxyList({name:entry.name,page:1,page_size:50});
        const current = response.data?.proxies?.find(row=>String(row.id)===entry.id);
        const type = getProxyAccessType(current);
        if (!current || current.status !== 'running' || !type || !isWebAccessType(type)) throw Error('unavailable');
        entry = {...entry,name:current.name,type,state:current.status};
      } catch {
        setError(tr('无法确认此历史入口，请到访问页面检查权限和当前配置。','Unable to resolve this historical entry. Check permissions and current settings on the Access page.'));
        return;
      } finally {setOpening(false);}
    }
    if (!isWebAccessType(entry.type)) {navigate('/proxy');return;}
    setSelected(entry);setPrompt(question.slice(0,12000));setCarry(false);
  };
  const open = async () => {
    if (!selected || opening) return;
    setOpening(true); setError(''); setConfigure(false);
    try {
      const path = await directAccessPath(Number(selected.id), selected.type);
      const agentHandoff = carry && supportsDraft(selected.type) ? stageAccessDraft({accessId:Number(selected.id), name:selected.name, prompt}) : undefined;
      const from = location.pathname + location.search;
      navigate(`${path}${path.includes('?')?'&':'?'}from=${encodeURIComponent(from)}`, {state: {agentHandoff}});
    } catch (reason) {
      setConfigure(reason instanceof AccessConfigurationRequired);
      setError(reason instanceof AccessConfigurationRequired ? tr('此访问尚未配置连接，请先在访问页面完成配置。','This access needs connection settings. Configure it on the Access page first.') : tr('无法打开访问，可能已停用、删除或权限已变更。请检查后重试。','Unable to open access. It may be disabled, deleted or no longer permitted. Check access and retry.'));
    } finally { setOpening(false); }
  };
  if (!entries.length) return null;
  return <>
    <div className="agent-access-results" aria-label={tr('访问入口','Access entries')}>{entries.map(entry => <div key={entry.id} className="agent-access-result">
      <div><strong>{entry.name}</strong><small>{accessTypeLabel(entry.type)} · {entry.state === 'running' ? tr('已启用','Enabled') : tr('已停用','Disabled')}</small></div>
      <Button disabled={entry.state !== 'running'||opening} onClick={()=>void select(entry)}>{isWebAccessType(entry.type)||entry.type==='web' ? tr('打开','Open') : tr('查看访问','View access')}</Button>
    </div>)}</div>
    {error&&!selected&&<Notice tone="danger">{error}<Button onClick={()=>navigate('/proxy')}>{tr('查看访问','View access')}</Button></Notice>}
    <Modal open={!!selected} title={tr('打开访问','Open access')} onClose={()=>{if(!opening)setSelected(undefined);}} width={520} footer={<><Button disabled={opening} onClick={()=>setSelected(undefined)}>{tr('取消','Cancel')}</Button><Button variant="primary" loading={opening} onClick={()=>void open()} disabled={carry&&!prompt.trim()}>{tr('打开','Open')}</Button></>}>
      <div className="agent-handoff-form"><p>{selected?.name}</p>
      {selected && supportsDraft(selected.type) && <>
        <label><input type="checkbox" checked={carry} onChange={e=>setCarry(e.target.checked)}/> {tr('携带问题到工作台 Agent','Carry a question to the workspace Agent')}</label>
        {carry && <Field label={tr('要继续处理的问题','Question to continue')} hint={tr('仅携带下方文字和目标访问名称，不包含历史对话、工具结果或密钥。进入后确认填入草稿，不自动发送。','Only this text and the target access name are carried, not chat history, tool results or keys. Confirm the draft in the workspace; nothing is sent automatically.')}><textarea className="liaison-input" rows={4} maxLength={12000} value={prompt} onChange={e=>setPrompt(e.target.value)}/></Field>}
      </>}
      {error && <Notice tone="danger">{error}{configure && <Button onClick={()=>navigate(`/proxy?configure=${selected?.id}`)}>{tr('配置访问','Configure access')}</Button>}</Notice>}
      </div>
    </Modal>
  </>;
}
