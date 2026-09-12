import { request } from '@/api/client';
import { MessageContent } from '@/components/AgentWorkspace/MessageContent';
import { Button, DangerConfirm, Field, Input, Modal, Notice, Select } from '@/components/ui';
import { useI18n } from '@/i18n';
import { getToken } from '@/store/session';
import { ArrowLeft, Copy, Plus, Trash2 } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import './index.less';

type AppConfig = {
  protocol: string;
  base_path: string;
  tls: boolean;
  has_api_key: boolean;
  api_key?: string;
  clear_key?: boolean;
};
type AccessConfig = {
  enabled: boolean;
  models: Record<string, string>;
  external_protocol: string;
};
type Workspace = { name: string; enabled: boolean; models: string[]; can_manage: boolean };
type WorkspaceTab = 'overview' | 'playground' | 'keys' | 'requests' | 'configuration';
class GatewayError extends Error {}
type Key = {
  id: number;
  name: string;
  models: string[];
  expires_at: string;
  secret?: string;
  token_limit: number | null;
  used_tokens: number;
  remaining_tokens: number | null;
  unknown_requests: number;
};
type RecordItem = {
  request_id: string;
  key_id: number;
  model: string;
  status: number;
  duration_ms: number;
  input_tokens?: number;
  output_tokens?: number;
  complete: boolean;
};

export default function AIGateway() {
  const { tr } = useI18n();
  const { proxyId, applicationId } = useParams();
  const [appId, setAppId] = useState(applicationId || '');
  const [name, setName] = useState('LLM');
  const [config, setConfig] = useState<AppConfig>();
  const [access, setAccess] = useState<AccessConfig>();
  const [workspace, setWorkspace] = useState<Workspace>();
  const [tab, setTab] = useState<WorkspaceTab>('overview');
  const [mapping, setMapping] = useState<[string, string][]>([]);
  const [keys, setKeys] = useState<Key[]>([]);
  const [records, setRecords] = useState<RecordItem[]>([]);
  const [secret, setSecret] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [createdKey, setCreatedKey] = useState<Key>();
  const [revokeTarget, setRevokeTarget] = useState<Key>();
  const [keyName, setKeyName] = useState('');
  const [days, setDays] = useState('30');
  const [tokenLimit, setTokenLimit] = useState('');
  const [quotaTarget, setQuotaTarget] = useState<Key>();
  const validLimit = tokenLimit.trim() === '' || (/^\d+$/.test(tokenLimit) && Number.isSafeInteger(Number(tokenLimit)) && Number(tokenLimit) <= 1_000_000_000_000);
  const limitValue = tokenLimit.trim() === '' ? null : Number(tokenLimit);
  const quotaHelp = tr('输入与输出合计，跨模型累计，不自动重置。留空不限额，0 禁止调用。', 'Lifetime input + output across models. Blank means unlimited; 0 blocks calls.');
  const quotaBoundary = tr('达到额度后拒绝新请求；已开始的请求（含并发请求）可能超额。用量未确认时，限额密钥暂停调用。', 'New requests are blocked at the limit. In-flight requests, including concurrent requests, may exceed it. Limited keys pause when usage is unconfirmed.');
  const keyStatus = (k: Key) => new Date(k.expires_at).getTime() <= Date.now() ? tr('已过期', 'Expired')
    : k.token_limit != null && k.used_tokens >= k.token_limit ? tr('额度耗尽', 'Quota exhausted')
    : k.token_limit != null && k.unknown_requests > 0 ? tr('用量未确认', 'Usage unconfirmed') : tr('有效', 'Active');
  const [scope, setScope] = useState<string[]>([]);
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState<{
    tone: 'danger' | 'success';
    text: string;
  }>();
  const [probe, setProbe] = useState<{ state: string; models?: string[] }>();
  const [model, setModel] = useState('');
  const [prompt, setPrompt] = useState('');
  const [answer, setAnswer] = useState('');
  const [history, setHistory] = useState<{role: 'user' | 'assistant'; content: string}[]>([]);
  const [requestId, setRequestId] = useState('');
  const generation = useRef(0);
  const abort = useRef<AbortController>();
  const base = `/api/v1/ai/accesses/${proxyId}`;
  const api = async <T,>(
    path: string,
    method: 'GET' | 'PUT' | 'POST' | 'DELETE' = 'GET',
    data?: unknown,
  ) => {
    const res = await request<API.Response<T>>(path, { method, data });
    if (res.code !== 200) throw new Error(res.message);
    return res.data as T;
  };
  const refresh = async () => {
    if (!proxyId) return;
    const current = generation.current;
    const [a, k, r] = await Promise.all([
      api<Workspace>(`${base}/workspace`),
      api<Key[]>(`${base}/keys`),
      api<RecordItem[]>(`${base}/requests`),
    ]);
    const settings = a.can_manage ? await api<AccessConfig>(base) : undefined;
    if (current !== generation.current) return;
    setWorkspace(a);
    setName(a.name);
    if (a.can_manage) {
      setAccess(settings);
      setMapping(Object.entries(settings?.models || {}));
    } else {
      setAccess(undefined);
    }
    setKeys(k || []);
    setRecords(r || []);
    setModel((v) => a.models.includes(v) ? v : a.models[0] || '');
  };
  useEffect(() => {
    let alive = true;
    setConfig(undefined);
    setAccess(undefined);
    setWorkspace(undefined);
    setTab('overview');
    setNotice(undefined);
    setSecret('');
    setCreateOpen(false);
    setCreatedKey(undefined);
    setRevokeTarget(undefined);
    setQuotaTarget(undefined);
    setHistory([]);
    setAnswer('');
    setRequestId('');
    setAppId(applicationId || '');
    (async () => {
      try {
        if (proxyId) {
          await refresh();
          return;
        }
        const id = applicationId;
        if (!id) throw new Error();
        const c = await api<AppConfig>(`/api/v1/ai/applications/${id}`);
        if (alive) {
          setAppId(id);
          setConfig(c);
          await refresh();
        }
      } catch {
        if (alive)
          setNotice({
            tone: 'danger',
            text: tr(
              '无法读取配置，请检查访问权限。',
              'Unable to load configuration. Check your permissions.',
            ),
          });
      }
    })();
    return () => {
      alive = false;
      generation.current++;
      abort.current?.abort();
    };
  }, [proxyId, applicationId, tr]);
  const act = async (id: string, fn: () => Promise<void>) => {
    const current = generation.current;
    setBusy(id);
    setNotice(undefined);
    try {
      await fn();
    } catch (error) {
      if (current !== generation.current) return;
      setNotice({
        tone: 'danger',
        text: error instanceof GatewayError ? error.message : tr(
          '操作失败，请检查配置、权限及连接器状态。更改目标后需要重新填写或清除密钥。',
          'Operation failed. Check configuration, permissions and connector status. Replace or clear the key after changing the target.',
        ),
      });
    } finally {
      if (current === generation.current) setBusy('');
    }
  };
  const saveApp = async () => {
    const c = await api<AppConfig>(
      `/api/v1/ai/applications/${appId}`,
      'PUT',
      config,
    );
    setConfig(c);
    return c;
  };
  const runTest = async () => {
    const current = generation.current;
    setAnswer('');
    setRequestId('');
    const messages = [...history, {role: 'user' as const, content: prompt}];
    let output = '';
    const controller = new AbortController();
    abort.current = controller;
    try {
      const response = await fetch(`${base}/test`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${getToken()}`,
        },
        body: JSON.stringify({
          model,
          messages,
          max_tokens: 1024,
          stream: true,
        }),
        signal: controller.signal,
      });
      if (current !== generation.current) return;
      setRequestId(response.headers.get('X-Request-ID') || '');
      if (!response.ok) {
        const payload = await response.json().catch(() => ({}));
        const code = payload.error?.code || `HTTP_${response.status}`;
        const reason = code === 'SERVICE_UNAVAILABLE'
          ? tr('模型服务暂不可用，请检查访问是否启用、连接器是否在线，以及内网模型服务是否运行。', 'Model service unavailable. Check that access is enabled, the connector is online and the private model service is running.')
          : tr('请求失败', 'Request failed');
        throw new GatewayError(`${reason} · ${code}`);
      }
      if (!response.body) throw new Error();
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let pending = '';
      let done = false;
      while (true) {
        const next = await reader.read();
        if (current !== generation.current) return;
        if (next.done) break;
        pending += decoder.decode(next.value, { stream: true });
        let end;
        while ((end = pending.indexOf('\n\n')) >= 0) {
          const block = pending.slice(0, end);
          pending = pending.slice(end + 2);
          for (const line of block.split('\n')) {
            if (!line.startsWith('data:')) continue;
            const data = line.slice(5).trim();
            if (data === '[DONE]') {
              done = true;
              continue;
            }
            const parsed = JSON.parse(data);
            if (parsed.error) throw new Error();
            const delta = parsed.choices?.[0]?.delta?.content;
            if (delta) { output += delta; setAnswer(output); }
          }
        }
      }
      if (!done) throw new Error();
      setHistory([...messages, {role: 'assistant', content: output}]);
      setAnswer('');
      setPrompt('');
    } catch (e) {
      if ((e as Error).name !== 'AbortError') throw e;
    } finally {
      if (current === generation.current) {
        abort.current = undefined;
        await refresh();
      }
    }
  };
  const probeState = (state: string) =>
    ({
      compatible: tr(
        '协议兼容 · 元数据探测成功',
        'Compatible protocol · metadata probe succeeded',
      ),
      auth_required: tr(
        '需要认证，请检查上游密钥',
        'Authentication required; check upstream key',
      ),
      unknown: tr(
        '无法确认该协议，可更换协议后重试',
        'Protocol not confirmed; select another protocol and retry',
      ),
      unreachable: tr(
        '无法连接内网服务',
        'Unable to reach the internal service',
      ),
    }[state] || state);
  return (
    <main className="ai-api-page">
      <header>
        {!proxyId && <Link to="/resource/app">
          <ArrowLeft size={18} />
          {tr('返回', 'Back')}
        </Link>}
        {proxyId && <nav className="ai-api-breadcrumb" aria-label={tr('页面层级', 'Breadcrumb')}><span>{tr('访问', 'Access')}</span><i>/</i><span>{tr('LLM 协议', 'LLM protocol')}</span><i>/</i><span>{name}</span></nav>}
        <h1>{name}</h1>
        <p>
          {tr(
            '通过连接器安全调用内网模型。',
            'Access internal models securely through a connector.',
          )}
        </p>
      </header>
      {notice && <Notice tone={notice.tone}>{notice.text}</Notice>}
      {!config && !workspace && !notice && <p role="status">{tr('正在加载…', 'Loading…')}</p>}
      {config && (
        <section className="ai-api-card">
          <h2>{tr('内网模型服务', 'Internal model service')}</h2>
          <p>
            {tr(
              '配置属于应用；同一应用的访问共用上游协议和凭据。',
              'Application-level configuration shared by its accesses.',
            )}
          </p>
          <div className="ai-api-grid">
            <Field label={tr('上游协议', 'Upstream protocol')}>
              <Select
                value={config.protocol}
                onChange={(e) =>
                  setConfig({ ...config, protocol: e.target.value })
                }
              >
                <option value="openai-compatible">OpenAI-compatible</option>
                <option value="anthropic">Anthropic Messages</option>
              </Select>
            </Field>
            <Field label={tr('API 路径', 'API base path')}>
              <Input
                value={config.base_path}
                onChange={(e) =>
                  setConfig({ ...config, base_path: e.target.value })
                }
                placeholder="/v1"
              />
            </Field>
            <Field label={tr('传输加密', 'Transport encryption')}>
              <Select
                value={config.tls ? 'https' : 'http'}
                onChange={(e) =>
                  setConfig({ ...config, tls: e.target.value === 'https' })
                }
              >
                <option value="http">HTTP</option>
                <option value="https">HTTPS · TLS</option>
              </Select>
            </Field>
            <Field label={tr('上游密钥', 'Upstream API key')}>
              <Input
                type="password"
                autoComplete="new-password"
                value={config.api_key || ''}
                placeholder={
                  config.has_api_key
                    ? '********'
                    : tr(
                        '可选 · 无认证服务留空',
                        'Optional for unauthenticated services',
                      )
                }
                onChange={(e) =>
                  setConfig({ ...config, api_key: e.target.value })
                }
              />
            </Field>
          </div>
          <label className="ai-api-check">
            <input
              type="checkbox"
              checked={!!config.clear_key}
              onChange={(e) =>
                setConfig({ ...config, clear_key: e.target.checked })
              }
            />
            {tr('清除已保存的密钥', 'Clear saved key')}
          </label>
          <footer>
            <Button
              disabled={!!busy}
              onClick={() =>
                void act('save-app', async () => {
                  await saveApp();
                  setNotice({
                    tone: 'success',
                    text: tr('配置已保存', 'Configuration saved'),
                  });
                })
              }
            >
              {tr('保存', 'Save')}
            </Button>
            <Button
              variant="primary"
              disabled={!!busy}
              loading={busy === 'probe'}
              onClick={() =>
                void act('probe', async () => {
                  await saveApp();
                  setProbe(
                    await api(`/api/v1/ai/applications/${appId}/probe`, 'POST'),
                  );
                })
              }
            >
              {tr('保存并探测', 'Save & probe')}
            </Button>
          </footer>
          {probe && (
            <div role="status" className="ai-api-probe">
              <strong>{probeState(probe.state)}</strong>
              <p>
                {tr(
                  '仅查询模型元数据，不触发推理；协议兼容不代表厂商品牌。',
                  'Metadata only, no inference. Compatibility does not identify the vendor.',
                )}
              </p>
              <div className="ai-api-tags">
                {probe.models?.map((id) => (
                  <code key={id}>{id}</code>
                ))}
              </div>
            </div>
          )}
        </section>
      )}
      {workspace && (
        <>
          <nav className="ai-api-tabs" aria-label={tr('LLM 协议工作区', 'LLM protocol workspace')}>
            {([
              ['overview', tr('概览', 'Overview')],
              ['playground', tr('在线体验', 'Playground')],
              ['keys', tr('API 密钥', 'API keys')],
              ['requests', tr('请求记录', 'Request records')],
              ...(workspace.can_manage ? [['configuration', tr('访问配置', 'Access configuration')]] : []),
            ] as [WorkspaceTab, string][]).map(([id, label]) => (
              <button key={id} type="button" aria-current={tab === id ? 'page' : undefined} onClick={() => setTab(id)}>{label}</button>
            ))}
          </nav>
          {tab === 'overview' && <section className="ai-api-card">
            <h2>{tr('调用信息', 'Connection details')}</h2>
            <p>{workspace.enabled ? tr('服务已启用。创建自己的 API 密钥，或在在线体验中发起请求。', 'Service enabled. Create your API key or try a request in the playground.') : tr('服务暂未启用，请联系管理员检查访问配置和连接状态。', 'Service unavailable. Ask an administrator to check access configuration and connectivity.')}</p>
            <Field label="Base URL"><div className="ai-api-endpoint"><code>{window.location.origin}{base}/v1</code><Button aria-label={tr('复制地址', 'Copy URL')} onClick={() => void act('copy', async () => { await navigator.clipboard.writeText(`${window.location.origin}${base}/v1`); })}><Copy size={15} /></Button></div></Field>
            <div className="ai-api-tags">{workspace.models.map(alias => <code key={alias}>{alias}</code>)}</div>
            <h2>{tr('调用示例', 'Request example')}</h2>
            <p>{tr('LIAISON_API_KEY 使用本页「API 密钥」中创建的 Liaison 调用密钥，不是上游模型密钥。外部调用始终需要认证，请勿将密钥放入前端代码。', 'Set LIAISON_API_KEY to a Liaison key created under API keys, not an upstream model key. External requests always require authentication. Never embed keys in frontend code.')}</p>
            {!keys.length && <p role="status">{tr('你尚未创建调用密钥。请先在「API 密钥」中创建，否则外部调用将返回 401。在线体验使用当前登录身份，无需先创建密钥。', 'You have not created an API key. Create one under API keys before calling externally, otherwise requests return 401. The playground uses your signed-in identity and needs no separate key.')}</p>}
            <pre className="ai-api-example">{`curl '${window.location.origin}${base}/v1/chat/completions' \\\n  -H "Authorization: Bearer $LIAISON_API_KEY" \\\n  -H 'Content-Type: application/json' \\\n  -d '${JSON.stringify({model: workspace.models[0] || 'MODEL_ALIAS', messages: [{role: 'user', content: 'Hello'}], stream: true})}'`}</pre>
            <footer><Button onClick={() => setTab('keys')}>{tr('管理密钥', 'Manage keys')}</Button><Button variant="primary" onClick={() => setTab('playground')}>{tr('在线体验', 'Open playground')}</Button></footer>
          </section>}
          {tab === 'configuration' && access && workspace.can_manage && <section className="ai-api-card">
            <h2>{tr('模型映射', 'Model mappings')}</h2>
            <p>
              {tr(
                '客户端使用左侧别名；右侧为内网模型 ID。未映射的模型无法调用。',
                'Clients use the public alias. Unmapped models cannot be called.',
              )}
            </p>
            <div className="ai-api-mapping-head">
              <span>{tr('公开别名', 'Public alias')}</span>
              <span>{tr('上游模型', 'Upstream model')}</span>
            </div>
            {mapping.map(([alias, target], i) => (
              <div className="ai-api-mapping" key={i}>
                <Input
                  aria-label={`${tr('公开别名', 'Public alias')} ${i + 1}`}
                  value={alias}
                  onChange={(e) =>
                    setMapping((v) =>
                      v.map((pair, n) =>
                        n === i ? [e.target.value, pair[1]] : pair,
                      ),
                    )
                  }
                />
                <Input
                  aria-label={`${tr('上游模型', 'Upstream model')} ${i + 1}`}
                  list="ai-upstream-models"
                  value={target}
                  onChange={(e) =>
                    setMapping((v) =>
                      v.map((pair, n) =>
                        n === i ? [pair[0], e.target.value] : pair,
                      ),
                    )
                  }
                />
                <Button
                  aria-label={tr('移除映射', 'Remove mapping')}
                  onClick={() => setMapping((v) => v.filter((_, n) => n !== i))}
                >
                  <Trash2 size={15} />
                </Button>
              </div>
            ))}
            <datalist id="ai-upstream-models">
              {probe?.models?.map((m) => (
                <option key={m} value={m} />
              ))}
            </datalist>
            <Button onClick={() => setMapping((v) => [...v, ['', '']])}>
              <Plus size={15} />
              {tr('添加映射', 'Add mapping')}
            </Button>
            <footer>
              <label className="ai-api-check">
                <input
                  type="checkbox"
                  checked={access.enabled}
                  onChange={(e) =>
                    setAccess({ ...access, enabled: e.target.checked })
                  }
                />
                {tr('允许 API 调用', 'Enable API calls')}
              </label>
              <Button
                variant="primary"
                disabled={!!busy}
                onClick={() =>
                  void act('save-access', async () => {
                    if (
                      mapping.some(([a, b]) => !a.trim() || !b.trim()) ||
                      new Set(mapping.map(([a]) => a)).size !== mapping.length
                    )
                      throw new Error();
                    await api(base, 'PUT', {
                      ...access,
                      models: Object.fromEntries(mapping),
                    });
                    await refresh();
                    setNotice({
                      tone: 'success',
                      text: tr('模型权限已更新', 'Model permissions updated'),
                    });
                  })
                }
              >
                {tr('保存映射', 'Save mappings')}
              </Button>
            </footer>
          </section>}
          {tab === 'keys' && <section className="ai-api-card">
            <div className="ai-api-key-toolbar">
              <div><h2>{tr('API 密钥', 'API keys')}</h2><p>{tr('管理你创建的 Liaison 调用密钥，供 curl 或 SDK 认证使用。不是上游模型密钥。', 'Manage your Liaison keys for curl or SDK authentication, not upstream model keys.')}</p></div>
              <div className="ai-api-key-actions"><Button disabled={!!busy} onClick={() => void act('refresh-keys', refresh)}>{tr('刷新', 'Refresh')}</Button>{workspace.can_manage && <Button variant="primary" onClick={() => { setCreateOpen(true); setSecret(''); setCreatedKey(undefined); setKeyName(''); setDays('30'); setTokenLimit(''); setScope([]); setNotice(undefined); }}><Plus size={15} />{tr('创建密钥', 'Create key')}</Button>}</div>
            </div>
            {!keys.length ? <p role="status">{tr('暂无调用密钥。外部 API 仍需认证，点击「创建密钥」开始。', 'No API keys yet. External requests still require authentication. Select Create key to begin.')}</p> :
            <div className="ai-api-table"><table>
              <thead><tr><th>{tr('名称', 'Name')}</th><th>{tr('授权模型', 'Allowed models')}</th><th>{tr('Token 已用 / 总额度', 'Tokens used / limit')}</th><th>{tr('剩余 Token', 'Remaining tokens')}</th><th>{tr('到期时间', 'Expires')}</th><th>{tr('状态', 'Status')}</th><th>{tr('操作', 'Actions')}</th></tr></thead>
              <tbody>{keys.map(k => <tr key={k.id}><td><strong>{k.name}</strong></td><td>{k.models.join(', ')}</td><td><span className="ai-api-quota-number">{(k.used_tokens || 0).toLocaleString()} / {k.token_limit == null ? tr('不限额', 'Unlimited') : k.token_limit.toLocaleString()}</span>{k.unknown_requests > 0 && <small className="ai-api-quota-note">{tr('含未确认用量', 'Includes unconfirmed usage')}</small>}</td><td>{k.token_limit == null ? '—' : k.remaining_tokens == null ? tr('未知', 'Unknown') : k.remaining_tokens.toLocaleString()}</td><td>{new Date(k.expires_at).toLocaleString()}</td><td>{keyStatus(k)}</td><td><div className="ai-api-key-actions">{workspace.can_manage && <Button disabled={!!busy || new Date(k.expires_at).getTime() <= Date.now()} onClick={() => { setQuotaTarget(k); setTokenLimit(k.token_limit == null ? '' : String(k.token_limit)); setNotice(undefined); }}>{tr('调整额度', 'Edit quota')}</Button>}<Button disabled={!!busy} onClick={() => { setRevokeTarget(k); setNotice(undefined); }}>{tr('撤销', 'Revoke')}</Button></div></td></tr>)}</tbody>
            </table></div>}
            <p className="ai-api-quota-note">{quotaBoundary}</p>
          </section>}
          <Modal open={!!quotaTarget} className="ai-api-key-modal" title={tr('调整额度', 'Edit quota')} closeOnMask={false} onClose={() => { if (!busy) setQuotaTarget(undefined); }} footer={<>
            <Button disabled={!!busy} onClick={() => setQuotaTarget(undefined)}>{tr('取消', 'Cancel')}</Button>
            <Button variant="primary" loading={busy === 'quota'} disabled={!!busy || !validLimit} onClick={() => void act('quota', async () => { if (!quotaTarget) return; await api(`${base}/keys/${quotaTarget.id}/quota`, 'PUT', {token_limit:limitValue}); await refresh(); setQuotaTarget(undefined); setNotice({tone:'success',text:tr('额度已更新，累计用量保留', 'Quota updated; lifetime usage retained')}); })}>{tr('保存', 'Save')}</Button>
          </>}>
            <div className="ai-api-key-form"><strong>{quotaTarget?.name}</strong><Field label={tr('Token 总额度', 'Total token limit')} hint={quotaHelp}><Input autoFocus type="number" min="0" max="1000000000000" step="1" placeholder={tr('不限额', 'Unlimited')} value={tokenLimit} onChange={e=>setTokenLimit(e.target.value)} /></Field><p>{quotaBoundary}</p><p>{tr('修改总额度不会清零已用 Token。', 'Changing the limit does not reset used tokens.')}</p>{notice?.tone === 'danger' && <Notice tone="danger">{notice.text}</Notice>}</div>
          </Modal>
          <Modal open={createOpen} className="ai-api-key-modal" title={secret ? tr('密钥已创建', 'Key created') : tr('创建密钥', 'Create key')} closeOnMask={false}
            onClose={() => { if (!busy) { setCreateOpen(false); setSecret(''); setCreatedKey(undefined); } }}
            footer={secret ? <Button variant="primary" onClick={() => { setSecret(''); setCreatedKey(undefined); setCreateOpen(false); }}>{tr('我已保存，完成', 'Saved, done')}</Button> : <>
              <Button disabled={!!busy} onClick={() => setCreateOpen(false)}>{tr('取消', 'Cancel')}</Button>
              <Button variant="primary" disabled={!!busy || !validLimit || !keyName.trim() || !scope.length || !Number.isInteger(Number(days)) || Number(days)<1 || Number(days)>365}
                onClick={() => void act('create-key', async () => {
                  const k = await api<Key>(`${base}/keys`, 'POST', {name:keyName.trim(),models:scope,expires_in_days:Number(days),token_limit:limitValue});
                  setCreatedKey(k); setSecret(k.secret || '');
                  setKeys(v => [...v, {...k, secret:undefined}]);
                })}>{tr('创建', 'Create')}</Button></>}>
            {notice?.tone === 'danger' && <Notice tone="danger">{notice.text}</Notice>}
            {secret && createdKey ? <div className="ai-api-key-result">
              <strong>{tr('密钥', 'Key')}「{createdKey.name}」{tr('已创建', 'created')}</strong>
              <p>{tr('下方密钥仅属于这个名称。原文只显示这一次，关闭前请复制并安全保存。', 'The secret below belongs to this key. It is shown only once. Copy and store it securely before closing.')}</p>
              <Field label={tr('授权模型', 'Allowed models')}><span>{createdKey.models.join(', ')}</span></Field>
              <Field label={tr('Token 总额度', 'Total token limit')}><span>{createdKey.token_limit == null ? tr('不限额', 'Unlimited') : createdKey.token_limit.toLocaleString()}</span></Field>
              <Field label={tr('调用密钥', 'API key')}><Input readOnly type="password" value={secret} /></Field>
              <Button onClick={() => void act('copy-key', async () => { await navigator.clipboard.writeText(secret); setNotice({tone:'success',text:tr('密钥已复制', 'Key copied')}); })}><Copy size={15} />{tr('复制密钥', 'Copy key')}</Button>
              {notice?.tone === 'success' && <Notice tone="success">{notice.text}</Notice>}
            </div> : <div className="ai-api-key-form">
              <Field label={tr('名称', 'Name')}><Input autoFocus value={keyName} onChange={e=>setKeyName(e.target.value)} /></Field>
              <Field label={tr('有效天数', 'Validity (days)')}><Input type="number" min="1" max="365" value={days} onChange={e=>setDays(e.target.value)} /></Field>
              <Field label={tr('Token 总额度', 'Total token limit')} hint={quotaHelp}><Input type="number" min="0" max="1000000000000" step="1" placeholder={tr('不限额', 'Unlimited')} value={tokenLimit} onChange={e=>setTokenLimit(e.target.value)} /></Field>
              <p>{quotaBoundary}</p>
              <Field label={tr('授权模型', 'Allowed models')}><div className="ai-api-tags">{workspace.models.map(alias=><label className="ai-api-check" key={alias}><input type="checkbox" checked={scope.includes(alias)} onChange={e=>setScope(v=>e.target.checked ? [...v,alias] : v.filter(x=>x!==alias))}/>{alias}</label>)}</div></Field>
              <p>{tr('仅可调用勾选的模型。创建后只显示一次密钥原文。', 'Only selected models are allowed. The secret is shown once after creation.')}</p>
            </div>}
          </Modal>
          <Modal open={!!revokeTarget} title={tr('撤销密钥', 'Revoke key')} onClose={() => { if (!busy) setRevokeTarget(undefined); }} footer={<>
            <Button disabled={!!busy} onClick={()=>setRevokeTarget(undefined)}>{tr('取消', 'Cancel')}</Button>
            <Button variant="danger" disabled={!!busy} onClick={()=>void act('revoke', async()=>{ if (!revokeTarget) return; await api(`${base}/keys/${revokeTarget.id}`, 'DELETE'); setKeys(v=>v.filter(k=>k.id!==revokeTarget.id)); setRevokeTarget(undefined); })}>{tr('确认撤销', 'Confirm revocation')}</Button>
          </>}>
            <DangerConfirm title={tr('确认撤销密钥', 'Revoke key') + '「' + (revokeTarget?.name || '') + '」？'} description={tr('使用此密钥的客户端将立即失效，此操作无法撤销。', 'Clients using this key will lose access immediately. This cannot be undone.')} />
            {notice?.tone === 'danger' && <Notice tone="danger">{notice.text}</Notice>}
          </Modal>
          {tab === 'playground' && <section className="ai-api-card">
            <h2>{tr('在线体验', 'Playground')}</h2>
            <p>
              {tr(
                '使用当前用户权限发起真实推理，可能产生上游费用。',
                'Runs real inference with your permissions and may incur upstream costs.',
              )}
            </p>
            {history.map((message, i) => <div className="ai-api-answer" key={i}><small>{message.role === 'user' ? tr('你', 'You') : model}</small><MessageContent text={message.content} /></div>)}
            <Field label={tr('模型', 'Model')}>
              <Select value={model} onChange={(e) => setModel(e.target.value)}>
                <option value="">{tr('选择模型', 'Select model')}</option>
                {workspace.models.map((m) => (
                  <option key={m}>{m}</option>
                ))}
              </Select>
            </Field>
            <Field label={tr('消息', 'Message')}>
              <textarea
                className="liaison-input"
                rows={3}
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
              />
            </Field>
            <footer>
              <Button disabled={!!busy || !history.length} onClick={() => { setHistory([]); setAnswer(''); setRequestId(''); }}>{tr('新对话', 'New conversation')}</Button>
              <Button
                variant="primary"
                disabled={!!busy || !workspace.enabled || !model || !prompt.trim()}
                onClick={() => void act('test', runTest)}
              >
                {tr('发送', 'Send')}
              </Button>
              {busy === 'test' && (
                <Button onClick={() => abort.current?.abort()}>
                  {tr('停止', 'Stop')}
                </Button>
              )}
            </footer>
            {answer && <div className="ai-api-answer"><MessageContent text={answer} /></div>}
            {requestId && <p>{tr('请求 ID', 'Request ID')}: <code>{requestId}</code></p>}
          </section>}
          {tab === 'requests' && <section className="ai-api-card">
            <h2>{tr('请求记录', 'Request records')}</h2>
            <p>
              {tr(
                '仅保存状态、耗时与上游提供的 Token 用量，不记录对话内容。',
                'Status, timing and reported token usage only. Prompts and answers are not logged.',
              )}
            </p>
            <div className="ai-api-table">
              <table>
                <thead>
                  <tr>
                    <th>{tr('请求 ID', 'Request ID')}</th>
                    <th>{tr('密钥', 'Key')}</th>
                    <th>{tr('模型', 'Model')}</th>
                    <th>{tr('状态', 'Status')}</th>
                    <th>{tr('耗时 (ms)', 'Duration (ms)')}</th>
                    <th>{tr('Token 输入 / 输出', 'Tokens in / out')}</th>
                  </tr>
                </thead>
                <tbody>
                  {records.map((r) => (
                    <tr key={r.request_id}>
                      <td>
                        <code title={r.request_id}>
                          {r.request_id.slice(0, 12)}
                        </code>
                      </td>
                      <td>{r.key_id || tr('控制台', 'Console')}</td>
                      <td>{r.model}</td>
                      <td>
                        {r.status}
                        {!r.complete ? ' · ' + tr('未完成', 'Incomplete') : ''}
                      </td>
                      <td>{r.duration_ms}</td>
                      <td>
                        {r.input_tokens ?? '—'} / {r.output_tokens ?? '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!records.length && (
                <p>{tr('暂无调用记录', 'No requests yet')}</p>
              )}
            </div>
          </section>}
        </>
      )}
    </main>
  );
}
