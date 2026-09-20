import { useEffect, useState } from 'react';
import { Plus, Star, Trash2, ChevronDown } from 'lucide-react';
import {ProviderIcon, ModelIcon} from '@/components/AgentWorkspace/ProviderIcon';
import { request } from '@/api/client';
import { Button, DangerConfirm, Field, Input, Modal, Notice, Select } from '@/components/ui';
import { useI18n } from '@/i18n';
import { useFeature } from '@/store/permissions';

type Provider = { id: string; type: string; base_url: string; model: string; models: string[]; has_api_key?: boolean; api_key?: string; clear_key?: boolean };
type Configuration = { enabled: boolean; output_language: 'zh'|'en'; default_provider: string; providers: Provider[] };
const presets = [
  { id: 'openai', label: 'OpenAI', url: 'https://api.openai.com/v1', model: 'gpt-5.4' },
  { id: 'anthropic', label: 'Anthropic', url: 'https://api.anthropic.com/v1', model: 'claude-sonnet-4-6' },
  { id: 'gemini', label: 'Gemini', url: 'https://generativelanguage.googleapis.com/v1beta/openai', model: 'gemini-2.5-pro' },
  { id: 'deepseek', label: 'DeepSeek', url: 'https://api.deepseek.com/v1', model: 'deepseek-v4-flash' },
  { id: 'zhipu', label: 'GLM', url: 'https://open.bigmodel.cn/api/paas/v4', model: 'glm-4.7' },
  { id: 'kimi', label: 'Kimi', url: 'https://api.moonshot.cn/v1', model: 'kimi-k2.6' },
  { id: 'minimax', label: 'MiniMax', url: 'https://api.minimaxi.com/v1', model: 'MiniMax-M2.7' },
  { id: 'xiaomi', label: 'MiMo', url: 'https://api.xiaomimimo.com/v1', model: 'mimo-v2.5-pro' },
  { id: 'custom', label: 'OpenAI-compatible', url: '', model: '' },
];

export default function Models({onDirtyChange, assistant = false}: {onDirtyChange?: (dirty: boolean) => void; assistant?: boolean}) {
  const canWrite = useFeature('settings.global.update');
  const { tr } = useI18n();
  const [config, setConfig] = useState<Configuration>();
  const [saved, setSaved] = useState<Configuration>();
  const [remove, setRemove] = useState<{provider: Provider; model?: string}>();
  const [replacement, setReplacement] = useState('');
  const [expanded, setExpanded] = useState('');
  const [adding, setAdding] = useState(false);
  const [testResults, setTestResults] = useState<Record<string, {tone: 'danger'|'success'; text: string} | undefined>>({});
  const [newModel, setNewModel] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  useEffect(() => {
    let alive = true;
    request<API.Response<Configuration>>('/api/v1/settings/model').then(r => {
      if (r.code !== 200 || !r.data) throw new Error('Model settings unavailable');
      if (alive && r.data) { setConfig(r.data); setSaved(r.data); setExpanded(r.data.default_provider); }
    }).catch(() => {
      if (alive) setNotice({ tone: 'danger', text: tr('无法读取模型配置，仅根组织管理员可管理。', 'Unable to load model settings. Root organization administrator access is required.') });
    });
    return () => { alive = false; };
  }, [tr]);
  const patch = (id: string, fields: Partial<Provider>) => {
    setTestResults(results => ({...results, [id]: undefined}));
    setConfig(c => c && ({ ...c, providers: c.providers.map(p => p.id === id ? { ...p, ...fields } : p) }));
  };
  const submit = async (testProvider?: string) => {
    if (!config) return;
    setBusy(testProvider || 'save'); setNotice(undefined);
    if (testProvider) setTestResults(results => ({...results, [testProvider]: undefined}));
    const report = (result: {tone: 'danger'|'success'; text: string}) => testProvider ? setTestResults(results => ({...results, [testProvider]: result})) : setNotice(result);
    try {
      const response = await request<API.Response<Configuration>>(`/api/v1/settings/model${testProvider ? '/test' : ''}`, {
        method: testProvider ? 'POST' : 'PUT',
        data: { enabled: config.providers.length > 0, output_language: config.output_language || 'zh', default_provider: config.default_provider, test_provider: testProvider,
          providers: config.providers.map(p => ({ id: p.id, type: p.type, base_url: p.base_url, models: p.models, model: p.model, api_key: p.api_key || '', clear_key: !!p.clear_key })) },
      });
      if (response.code !== 200) throw new Error();
      if (!testProvider && response.data) { setConfig(response.data); setSaved(response.data); }
      report({ tone: 'success', text: testProvider ? tr('测试成功', 'Test successful') : tr('配置已保存，即时生效', 'Settings saved and applied') });
    } catch {
      report({ tone: 'danger', text: tr('操作失败，请检查地址、模型及密钥。更改地址时需重新填写或清除密钥。', 'Request failed. Check endpoint, model and key. Changing an endpoint requires replacing or clearing its saved key.') });
    } finally { setBusy(''); }
  };
  const addProvider = (addType: string) => {
    if (!config) return;
    const preset = presets.find(p => p.id === addType)!;
    const id = addType === 'custom' ? `custom-${Date.now()}` : addType;
    if (config.providers.some(p => p.id === id)) { setExpanded(id); return; }
    setConfig({ ...config, default_provider: config.default_provider || id, providers: [...config.providers, { id, type: addType, base_url: preset.url, model: preset.model, models: preset.model ? [preset.model] : [] }] });
    setExpanded(id);
    setAdding(false);
    setTestResults(results => ({...results, [id]: undefined}));
  };
  const available = presets.filter(p => p.id === 'custom' || !config?.providers.some(item => item.id === p.id));
  const addModel = (p: Provider) => {
    const name = newModel[p.id]?.trim();
    if (!name || name.length > 200 || p.models.length >= 100 || p.models.includes(name)) return;
    patch(p.id, { models: [...p.models, name], model: p.model || name });
    setNewModel({ ...newModel, [p.id]: '' });
  };
  const dirty = !!config && JSON.stringify(config) !== JSON.stringify(saved);
  useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => {
    const warn = (event: BeforeUnloadEvent) => { if (dirty) { event.preventDefault(); event.returnValue = ''; } };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [dirty]);
  const selected = config?.providers.find(p => p.id === config.default_provider);
  const active = saved?.providers.find(p => p.id === saved.default_provider);
  const remaining = config?.providers.flatMap(p => p.models.filter(name => !(remove?.provider.id === p.id && (!remove.model || remove.model === name))).map(name => ({provider: p.id, model: name}))) || [];
  const deletingDefault = !!remove && remove.provider.id === config?.default_provider && (!remove.model || remove.model === selected?.model);
  const confirmRemove = () => {
    if (!config || !remove) return;
    const next = config.providers.flatMap(p => {
      if (p.id !== remove.provider.id) return [{...p}];
      if (!remove.model) return [];
      const models = p.models.filter(name => name !== remove.model);
      // An empty provider cannot be saved; remove its configuration explicitly.
      return models.length ? [{...p, models, model: models.includes(p.model) ? p.model : models[0]}] : [];
    });
    let defaultProvider = config.default_provider;
    if (deletingDefault) {
      const chosen = remaining.find(item => JSON.stringify([item.provider, item.model]) === replacement);
      if (remaining.length && !chosen) return;
      defaultProvider = chosen?.provider || '';
      if (chosen) { const provider = next.find(p => p.id === chosen.provider); if (provider) provider.model = chosen.model; }
    }
    setConfig({...config, providers: next, default_provider: defaultProvider, enabled: next.length > 0});
    setTestResults({});
    setRemove(undefined);
  };
  return <section className="settings-section settings-models">
    <header className="settings-section-heading"><h2>{assistant ? tr('助理', 'Assistant') : tr('模型', 'Models')}</h2><p>{assistant ? tr('设置 AI 助理的默认输出语言。', 'Set the default response language for the AI assistant.') : tr('管理模型提供方，选择 Agent 对话与终端提示使用的默认模型。', 'Manage providers and choose the default model for Agent conversations and terminal suggestions.')}</p></header>
    {notice && <Notice tone={notice.tone}>{notice.text}</Notice>}
    {!canWrite && <Notice>{tr('当前为只读模式，修改需要设置管理权限。', 'Read-only. Settings management permission is required to make changes.')}</Notice>}
    {!config && !notice && <p role="status">{tr('加载中…', 'Loading…')}</p>}
    {config && <fieldset disabled={!!busy || !canWrite} className="model-provider-form">
      <div className="model-general-card">
        {!assistant && <div className="model-default-summary"><span>{tr('当前生效的默认模型', 'Active default model')}</span><strong>{active ? <><ProviderIcon provider={active.type}/><code>{active.model}</code></> : tr('未配置', 'Not configured')}</strong>{dirty && <span className="model-pending" role="status">{tr('有未保存的更改', 'Unsaved changes')}</span>}</div>}
        {assistant && <><div className="model-language-field"><Field label={tr('输出语言', 'Output language')}>
        <Select aria-label={tr('AI 输出语言', 'AI output language')} value={config.output_language || 'zh'} onChange={e=>setConfig({...config,output_language:e.target.value as 'zh'|'en'})}>
          <option value="zh">简体中文</option><option value="en">English</option>
        </Select>
        </Field></div>
        <small>{tr('初始语言在安装时确定，不跟随浏览器。修改后保存，对后续请求生效；命令和代码保持原样。', 'Initially chosen during installation, independent of the browser. Save to apply to subsequent requests; commands and code stay unchanged.')}</small></>}
      </div>
      {!assistant && <>
      <div className="model-provider-picker">
        <Button variant="ghost" className="model-picker-heading" aria-expanded={adding} aria-controls="model-provider-options" onClick={() => setAdding(!adding)}><Plus size={18}/><span>{tr('添加模型供应商', 'Add model provider')}</span><small>{available.length} {tr('个可选', 'available')}</small><ChevronDown size={16}/></Button>
        {adding && <div id="model-provider-options">{available.map(p => <Button key={p.id} variant="ghost" className="model-picker-option" onClick={() => addProvider(p.id)} aria-label={`${tr('添加', 'Add')} ${p.label}`}>
          <ProviderIcon provider={p.id} size={22}/><span><span>{p.id === 'custom' ? tr('自定义（OpenAI 兼容）', 'Custom (OpenAI-compatible)') : p.label}</span><small>{p.id === 'custom' ? tr('连接 Ollama、vLLM 或其他兼容服务，填写服务地址和模型 ID。', 'Connect Ollama, vLLM or another compatible service with its endpoint and model ID.') : tr(`配置 ${p.label} 的 API 密钥和模型。`, `Configure API credentials and models for ${p.label}.`)}</small></span><Plus size={17}/>
        </Button>)}</div>}
      </div>
      <div className="model-providers-heading"><h3>{tr('模型提供方', 'Model providers')}</h3><span>{tr('保存有效的模型配置后即可使用 AI，使用范围由用户权限控制。', 'Save a valid model configuration to use AI. User permissions control access.')}</span></div>
      <div className="model-provider-list">{config.providers.map(p => <article className="model-provider-card" key={p.id}>
        <div className="model-provider-header">
          <button className="model-provider-title" aria-expanded={expanded === p.id} onClick={() => setExpanded(expanded === p.id ? '' : p.id)}><ProviderIcon provider={p.type} size={20} /><span>{presets.find(meta => meta.id === p.type)?.label || p.type}<small>{p.models.length} {tr('个模型', p.models.length === 1 ? 'model' : 'models')}</small></span><ChevronDown size={16} /></button>
          <span className="model-provider-state">{saved?.providers.some(item => item.id === p.id) ? tr('已配置', 'Configured') : tr('待保存', 'Not saved')}</span>
        </div>
        {expanded === p.id && <div className="model-provider-body">
          <Field label="API Key" hint={tr('加密保存；留空保留原密钥，修改地址需替换或清除密钥。', 'Stored encrypted. Leave blank to keep the saved key; replace or clear it when changing endpoints.')}><Input type="password" autoComplete="new-password" value={p.api_key || ''} onChange={e => patch(p.id, { api_key: e.target.value, clear_key: false })} placeholder={p.has_api_key && !p.clear_key ? tr('已保存 · 输入以替换', 'Saved · enter to replace') : tr('输入密钥（本地服务可留空）', 'API key (optional for local services)')} /></Field>
          {p.has_api_key && <label className="model-enable"><input type="checkbox" checked={!!p.clear_key} onChange={e => patch(p.id, { clear_key: e.target.checked, api_key: '' })} />{tr('清除已保存密钥', 'Clear saved key')}</label>}
          <details className="model-endpoint" open={p.type === 'custom' || undefined}><summary>{tr('高级 · 服务地址', 'Advanced · Base URL')}</summary><Field label="Base URL" required><Input value={p.base_url} onChange={e => patch(p.id, { base_url: e.target.value })} placeholder="https://your-service/v1" /></Field></details>
          <section className="model-list-section"><h3>{tr('模型列表', 'Models')}</h3>
            <div className="model-rows">{p.models.map(name => <div className="model-row" key={name}>
              <span className="model-row-name"><ModelIcon model={name} provider={p.type}/><code>{name}</code></span>
              <div className="model-row-actions"><Button variant="ghost" disabled={selected?.id === p.id && selected.model === name} onClick={() => setConfig({...config, default_provider: p.id, providers: config.providers.map(item => item.id === p.id ? {...item, model: name} : item)})}><Star size={14}/>{active?.id === p.id && active.model === name ? tr('当前默认', 'Active default') : selected?.id === p.id && selected.model === name ? tr('待保存默认', 'Pending default') : tr('设为默认', 'Set default')}</Button><Button variant="ghost" aria-label={`${tr('删除模型', 'Delete model')} ${name}`} onClick={() => {setReplacement('');setRemove({provider:p,model:name});}}><Trash2 size={14}/></Button></div>
            </div>)}</div>
            {!p.models.length && <p>{tr('添加至少一个模型后保存。', 'Add at least one model before saving.')}</p>}
            <div className="model-add-row"><Input aria-label={`${tr('新增模型', 'New model')} ${p.id}`} value={newModel[p.id] || ''} placeholder={tr('输入模型 ID', 'Enter model ID')} onChange={e => setNewModel({ ...newModel, [p.id]: e.target.value })} onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addModel(p); } }} /><Button disabled={!newModel[p.id]?.trim() || p.models.includes(newModel[p.id]?.trim())} onClick={() => addModel(p)}><Plus size={15} />{tr('添加', 'Add')}</Button></div>
          </section>
          <div className="model-settings-actions"><Button loading={busy === p.id} onClick={() => void submit(p.id)}>{tr('测试连接', 'Test connection')}</Button><Button variant="ghost" onClick={() => {setReplacement('');setRemove({provider:p});}}><Trash2 size={14} />{tr('移除提供方', 'Remove provider')}</Button></div>
          {busy === p.id && <div role="status">{tr('正在测试连接…', 'Testing connection…')}</div>}
          {testResults[p.id] && <div className="model-test-result" role="status"><Notice tone={testResults[p.id]!.tone}>{testResults[p.id]!.text}</Notice></div>}
        </div>}
      </article>)}</div></>}
      <div className="model-save-bar"><Button variant="primary" loading={busy === 'save'} disabled={!dirty || config.providers.some(p => !p.models.length || !p.base_url.trim())} onClick={() => void submit()}>{tr('保存全部更改', 'Save all changes')}</Button><Button disabled={!dirty} onClick={() => {setConfig(saved);setNotice(undefined);setTestResults({});}}>{tr('放弃更改', 'Discard changes')}</Button><span>{assistant ? tr('输出语言在保存后生效。', 'The output language applies after saving.') : tr('默认模型及删除操作均在保存后生效。', 'Default model and deletions apply only after saving.')}</span></div>
    </fieldset>}
    <Modal open={!!remove} title={tr('删除模型配置', 'Remove model configuration')} onClose={() => setRemove(undefined)} footer={<><Button onClick={() => setRemove(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" disabled={!canWrite || !!busy || (deletingDefault && remaining.length > 0 && !replacement)} onClick={confirmRemove}>{tr('确认移除', 'Confirm removal')}</Button></>}>
      <DangerConfirm title={remove?.model || presets.find(p => p.id === remove?.provider.type)?.label || remove?.provider.id} description={remove?.model && remove.provider.models.length > 1 ? tr('仅移除 Liaison 中的模型配置，不删除上游模型。保存全部更改后生效。', 'Removes only the Liaison model configuration, not the upstream model. Save all changes to apply.') : tr('将移除此提供方及其模型、密钥配置，不影响上游服务。保存全部更改后生效。', 'Removes this provider, its models and saved credentials, not the upstream service. Save all changes to apply.')}/>
      {deletingDefault && (remaining.length ? <Field label={tr('选择新的默认模型', 'Choose a replacement default')}><Select value={replacement} onChange={e=>setReplacement(e.target.value)}><option value="">{tr('请选择', 'Select a model')}</option>{remaining.map(item=><option key={JSON.stringify([item.provider,item.model])} value={JSON.stringify([item.provider,item.model])}>{item.provider} · {item.model}</option>)}</Select></Field> : <Notice tone="warning">{tr('这是最后一个模型，保存后 AI 功能将不可用。', 'This is the last model. AI will be unavailable after saving.')}</Notice>)}
    </Modal>
  </section>;
}
