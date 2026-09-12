import { useEffect, useState } from 'react';
import { Plus, Star, Trash2, ChevronDown } from 'lucide-react';
import {ProviderIcon, ModelIcon} from '@/components/AgentWorkspace/ProviderIcon';
import BrandSelect from '@/components/AgentWorkspace/BrandSelect';
import { request } from '@/api/client';
import { Button, Field, Input, Notice, Select } from '@/components/ui';
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

export default function Models() {
  const canWrite = useFeature('settings.global.update');
  const { tr } = useI18n();
  const [config, setConfig] = useState<Configuration>();
  const [expanded, setExpanded] = useState('');
  const [addType, setAddType] = useState('openai');
  const [newModel, setNewModel] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  useEffect(() => {
    let alive = true;
    request<API.Response<Configuration>>('/api/v1/settings/model').then(r => {
      if (alive && r.data) { setConfig(r.data); setExpanded(r.data.default_provider); }
    }).catch(() => {
      if (alive) setNotice({ tone: 'danger', text: tr('无法读取模型配置，仅根组织管理员可管理。', 'Unable to load model settings. Root organization administrator access is required.') });
    });
    return () => { alive = false; };
  }, [tr]);
  const patch = (id: string, fields: Partial<Provider>) => setConfig(c => c && ({ ...c, providers: c.providers.map(p => p.id === id ? { ...p, ...fields } : p) }));
  const submit = async (testProvider?: string) => {
    if (!config) return;
    setBusy(testProvider || 'save'); setNotice(undefined);
    try {
      const response = await request<API.Response<Configuration>>(`/api/v1/settings/model${testProvider ? '/test' : ''}`, {
        method: testProvider ? 'POST' : 'PUT',
        data: { enabled: config.providers.length > 0, output_language: config.output_language || 'zh', default_provider: config.default_provider, test_provider: testProvider,
          providers: config.providers.map(p => ({ id: p.id, type: p.type, base_url: p.base_url, models: p.models, model: p.model, api_key: p.api_key || '', clear_key: !!p.clear_key })) },
      });
      if (response.code !== 200) throw new Error();
      if (!testProvider && response.data) setConfig(response.data);
      setNotice({ tone: 'success', text: testProvider ? tr('模型连接成功', 'Model connection successful') : tr('配置已保存，即时生效', 'Settings saved and applied') });
    } catch {
      setNotice({ tone: 'danger', text: tr('操作失败，请检查地址、模型及密钥。更改地址时需重新填写或清除密钥。', 'Request failed. Check endpoint, model and key. Changing an endpoint requires replacing or clearing its saved key.') });
    } finally { setBusy(''); }
  };
  const addProvider = () => {
    if (!config) return;
    const preset = presets.find(p => p.id === addType)!;
    const id = addType === 'custom' ? `custom-${Date.now()}` : addType;
    if (config.providers.some(p => p.id === id)) { setExpanded(id); return; }
    setConfig({ ...config, default_provider: config.default_provider || id, providers: [...config.providers, { id, type: addType, base_url: preset.url, model: preset.model, models: preset.model ? [preset.model] : [] }] });
    setExpanded(id);
  };
  const addModel = (p: Provider) => {
    const name = newModel[p.id]?.trim();
    if (!name || p.models.includes(name)) return;
    patch(p.id, { models: [...p.models, name], model: p.model || name });
    setNewModel({ ...newModel, [p.id]: '' });
  };
  return <section className="settings-section settings-models">
    <header className="settings-section-heading"><h2>{tr('模型配置', 'Models')}</h2><p>{tr('管理模型提供方，选择 Agent 对话与终端提示使用的默认模型。', 'Manage providers and choose the default model for Agent conversations and terminal suggestions.')}</p></header>
    {notice && <Notice tone={notice.tone}>{notice.text}</Notice>}
    {config && <fieldset disabled={!!busy || !canWrite} className="model-provider-form">
      <div className="model-providers-heading"><h3>{tr('AI 输出语言', 'AI output language')}</h3><span>{tr('统一所有 AI 对话与分析的语言，不跟随界面语言。', 'Use one language for all AI conversations and analysis, independently of the interface.')}</span></div>
      <div className="model-general-card">
        <div className="model-language-field"><Field label={tr('输出语言', 'Output language')}>
        <Select aria-label={tr('AI 输出语言', 'AI output language')} value={config.output_language || 'zh'} onChange={e=>setConfig({...config,output_language:e.target.value as 'zh'|'en'})}>
          <option value="zh">简体中文</option><option value="en">English</option>
        </Select>
        </Field></div>
        <small>{tr('命令、SQL 和原始输出保持原样。保存后对下一次请求生效。', 'Commands, SQL and raw output stay unchanged. Applies to the next request after saving.')}</small>
      </div>
      <div className="model-providers-heading"><h3>{tr('模型提供方', 'Model providers')}</h3><span>{tr('保存有效的模型配置后即可使用 AI，使用范围由用户权限控制。', 'Save a valid model configuration to use AI. User permissions control access.')}</span></div>
      <div className="model-provider-list">{config.providers.map(p => <article className="model-provider-card" key={p.id}>
        <div className="model-provider-header">
          <button className="model-provider-title" onClick={() => setExpanded(expanded === p.id ? '' : p.id)}><ProviderIcon provider={p.type} size={20} /><span>{presets.find(meta => meta.id === p.type)?.label || p.type}<small>{p.model || tr('尚未选择模型', 'No model selected')}</small></span><ChevronDown size={16} /></button>
          <Button disabled={!!busy} onClick={() => setConfig({ ...config, default_provider: p.id })}><Star size={14} fill={config.default_provider === p.id ? 'currentColor' : 'none'} />{config.default_provider === p.id ? tr('默认', 'Default') : tr('设为默认', 'Set default')}</Button>
        </div>
        {expanded === p.id && <div className="model-provider-body">
          <Field label="Base URL" required><Input value={p.base_url} onChange={e => patch(p.id, { base_url: e.target.value })} placeholder="https://your-service/v1" /></Field>
          <Field label="API Key" hint={tr('加密保存；留空保留原密钥，修改地址需替换或清除密钥。', 'Stored encrypted. Leave blank to keep the saved key; replace or clear it when changing endpoints.')}><Input type="password" autoComplete="new-password" value={p.api_key || ''} onChange={e => patch(p.id, { api_key: e.target.value, clear_key: false })} placeholder={p.has_api_key && !p.clear_key ? tr('已保存 · 输入以替换', 'Saved · enter to replace') : tr('输入密钥（本地服务可留空）', 'API key (optional for local services)')} /></Field>
          {p.has_api_key && <label className="model-enable"><input type="checkbox" checked={!!p.clear_key} onChange={e => patch(p.id, { clear_key: e.target.checked, api_key: '' })} />{tr('清除已保存密钥', 'Clear saved key')}</label>}
          <Field label={tr('模型列表', 'Models')} hint={tr('点击星标选择此提供方的默认模型。', 'Use the star to select this provider’s default model.')}><div className="model-chip-list">{p.models.map(name => <span className="model-chip" key={name}><button onClick={() => patch(p.id, { model: name })} title={tr('设为默认模型', 'Set default model')}><Star size={13} fill={p.model === name ? 'currentColor' : 'none'} /><ModelIcon model={name} provider={p.type}/>{name}</button><button aria-label={`${tr('移除', 'Remove')} ${name}`} onClick={() => { const models = p.models.filter(m => m !== name); patch(p.id, { models, model: p.model === name ? models[0] || '' : p.model }); }}>×</button></span>)}</div><div className="model-add-row"><Input value={newModel[p.id] || ''} placeholder={tr('输入模型 ID', 'Enter model ID')} onChange={e => setNewModel({ ...newModel, [p.id]: e.target.value })} onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addModel(p); } }} /><Button onClick={() => addModel(p)}><Plus size={15} />{tr('添加', 'Add')}</Button></div></Field>
          <div className="model-settings-actions"><Button disabled={!!busy} onClick={() => void submit(p.id)}>{busy === p.id ? tr('测试中…', 'Testing…') : tr('测试连接', 'Test connection')}</Button><Button disabled={!!busy} onClick={() => { const providers = config.providers.filter(item => item.id !== p.id); setConfig({ ...config, providers, default_provider: config.default_provider === p.id ? providers[0]?.id || '' : config.default_provider }); }}><Trash2 size={14} />{tr('移除提供方', 'Remove provider')}</Button></div>
        </div>}
      </article>)}</div>
      <div className="model-add-provider"><BrandSelect label={tr('选择模型提供方', 'Select model provider')} value={addType} onChange={setAddType} disabled={!!busy||!canWrite} options={presets.map(p=>({value:p.id,label:p.label,icon:<ProviderIcon provider={p.id}/>}))}/><Button onClick={addProvider}><Plus size={15} />{tr('添加提供方', 'Add provider')}</Button></div>
      <div className="model-save-bar"><Button variant="primary" disabled={!!busy} onClick={() => void submit()}>{busy === 'save' ? tr('保存中…', 'Saving…') : tr('保存配置', 'Save settings')}</Button><span>{tr('保存后生效，无需重启。', 'Changes apply after saving. No restart needed.')}</span></div>
    </fieldset>}
  </section>;
}
