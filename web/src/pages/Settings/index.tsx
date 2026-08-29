import { Button, Field, Input, Modal, Notice, Segmented } from '@/components/ui';
import { APP_NAME } from '@/constants';
import { useI18n } from '@/i18n';
import { createAPIToken, listAPITokens, revokeAPIToken } from '@/services/api';
import { useThemeMode } from '@/store/theme';
import { Copy, Github, Globe2, Info, KeyRound, Palette, Plus } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useState } from 'react';
import './index.less';

const GITHUB_URL = 'https://github.com/liaisonio/liaison';

const SettingsPage: React.FC = () => {
  const { tr, locale, setLocale } = useI18n();
  const { preference, setPreference } = useThemeMode();
  const [active, setActive] = useState<'preferences' | 'tokens' | 'about'>('preferences');
  const [tokens, setTokens] = useState<API.APIToken[]>([]);
  const [tokensLoading, setTokensLoading] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [tokenName, setTokenName] = useState('');
  const [expires, setExpires] = useState('0');
  const [revealed, setRevealed] = useState('');
  const [revokeTarget, setRevokeTarget] = useState<API.APIToken>();
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();

  const showNotice = (tone: 'danger' | 'success', text: string) => {
    setNotice({ tone, text });
    window.setTimeout(() => setNotice(undefined), 3200);
  };

  const fetchTokens = useCallback(async () => {
    setTokensLoading(true);
    try {
      const response = await listAPITokens();
      if (response.code === 200) setTokens(response.data?.tokens || []);
      else throw new Error(response.message);
    } catch (error: any) {
      showNotice('danger', error?.message || tr('加载 Token 失败', 'Failed to load tokens'));
    } finally {
      setTokensLoading(false);
    }
  }, [tr]);

  useEffect(() => { void fetchTokens(); }, [fetchTokens]);

  const handleCreateToken = async (event: FormEvent) => {
    event.preventDefault();
    const days = Number(expires || 0);
    if (!tokenName.trim()) {
      showNotice('danger', tr('请填写名称', 'Please enter a name'));
      return;
    }
    if (!Number.isInteger(days) || days < 0 || days > 3650) {
      showNotice('danger', tr('过期天数应为 0-3650 的整数', 'Expiry must be an integer from 0 to 3650'));
      return;
    }
    setCreateLoading(true);
    try {
      const response = await createAPIToken({ name: tokenName.trim(), expires_in_days: days });
      if (response.code !== 200 || !response.data?.token) throw new Error(response.message);
      setRevealed(response.data.token);
      setCreateOpen(false);
      setTokenName('');
      setExpires('0');
      await fetchTokens();
    } catch (error: any) {
      showNotice('danger', error?.message || tr('创建失败', 'Failed to create'));
    } finally {
      setCreateLoading(false);
    }
  };

  const handleRevoke = async () => {
    if (!revokeTarget) return;
    try {
      const response = await revokeAPIToken(revokeTarget.id);
      if (response.code !== 200) throw new Error(response.message);
      setRevokeTarget(undefined);
      showNotice('success', tr('Token 已撤销', 'Token revoked'));
      await fetchTokens();
    } catch (error: any) {
      showNotice('danger', error?.message || tr('撤销失败', 'Failed to revoke'));
    }
  };

  return (
    <div className="settings-page native-settings-page">
      {notice ? <div className="settings-floating-notice"><Notice tone={notice.tone}>{notice.text}</Notice></div> : null}
      <div className="settings-shell native-settings-shell">
        <aside className="native-settings-tabs">
          <button className={active === 'preferences' ? 'is-active' : ''} onClick={() => setActive('preferences')}><Palette size={15} />{tr('界面偏好', 'Appearance')}</button>
          <button className={active === 'tokens' ? 'is-active' : ''} onClick={() => setActive('tokens')}><KeyRound size={15} />{tr('API Token', 'API Tokens')}</button>
          <button className={active === 'about' ? 'is-active' : ''} onClick={() => setActive('about')}><Info size={15} />{tr('关于', 'About')}</button>
        </aside>
        <main className="native-settings-content">
          {active === 'preferences' ? (
            <section className="settings-section settings-preferences">
              <div className="settings-preference-row"><div className="settings-preference-copy"><Palette size={18} /><div><strong>{tr('主题', 'Theme')}</strong><span>{tr('选择界面外观，系统模式会跟随设备设置。', 'Choose an appearance. System follows your device setting.')}</span></div></div><Segmented value={preference} onChange={setPreference} options={[{ label: tr('跟随系统', 'System'), value: 'system' }, { label: tr('浅色', 'Light'), value: 'light' }, { label: tr('深色', 'Dark'), value: 'dark' }]} /></div>
              <div className="settings-preference-row"><div className="settings-preference-copy"><Globe2 size={18} /><div><strong>{tr('语言', 'Language')}</strong><span>{tr('切换控制台的显示语言。', 'Switch the language used by the console.')}</span></div></div><Segmented value={locale} onChange={setLocale} options={[{ label: '中文', value: 'zh-CN' }, { label: 'English', value: 'en-US' }]} /></div>
            </section>
          ) : null}

          {active === 'tokens' ? (
            <section className="settings-section">
              <div className="password-tips"><KeyRound size={20} /><div><strong>{tr('个人访问令牌 (PAT)', 'Personal Access Tokens')}</strong><span>{tr('用于 CLI / 脚本调用 API。每个 Token 只会明文显示一次。', 'For CLI and script API access. Each token is shown once.')}</span></div></div>
              <div className="native-token-toolbar"><Button variant="primary" onClick={() => setCreateOpen(true)}><Plus size={14} />{tr('新建 Token', 'Create token')}</Button></div>
              <div className="native-table-wrap">
                <table className="native-table">
                  <thead><tr><th>{tr('名称', 'Name')}</th><th>{tr('前缀', 'Prefix')}</th><th>{tr('创建时间', 'Created')}</th><th>{tr('最后使用', 'Last used')}</th><th>{tr('过期时间', 'Expires')}</th><th>{tr('操作', 'Actions')}</th></tr></thead>
                  <tbody>
                    {tokens.map((token) => <tr key={token.id}><td>{token.name}</td><td><code>{token.token_prefix}…</code></td><td>{token.created_at}</td><td>{token.last_used_at || '-'}{token.last_used_ip ? ` (${token.last_used_ip})` : ''}</td><td>{token.expires_at || tr('永不过期', 'Never')}</td><td><Button variant="danger" onClick={() => setRevokeTarget(token)}>{tr('撤销', 'Revoke')}</Button></td></tr>)}
                    {!tokensLoading && tokens.length === 0 ? <tr><td colSpan={6} className="native-table-empty">{tr('暂无 Token', 'No tokens')}</td></tr> : null}
                    {tokensLoading ? <tr><td colSpan={6} className="native-table-empty">{tr('加载中…', 'Loading…')}</td></tr> : null}
                  </tbody>
                </table>
              </div>
            </section>
          ) : null}

          {active === 'about' ? (
            <section className="settings-section native-about"><h3>{tr('关于', 'About')} {APP_NAME}</h3><dl><div><dt>{tr('产品名称', 'Product')}</dt><dd>{APP_NAME}</dd></div><div><dt>GitHub</dt><dd><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><Github size={14} />{GITHUB_URL}</a></dd></div><div><dt>{tr('许可证', 'License')}</dt><dd>Apache License 2.0</dd></div></dl></section>
          ) : null}
        </main>
      </div>

      <Modal open={createOpen} title={tr('新建 API Token', 'Create API Token')} onClose={() => setCreateOpen(false)} footer={<><Button onClick={() => setCreateOpen(false)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="create-token-form" disabled={createLoading}>{createLoading ? tr('创建中…', 'Creating…') : tr('创建', 'Create')}</Button></>}>
        <form id="create-token-form" className="native-modal-form" onSubmit={handleCreateToken}>
          <Field label={tr('名称', 'Name')} required><Input value={tokenName} onChange={(event) => setTokenName(event.target.value)} maxLength={64} placeholder={tr('例如：laptop-cli', 'e.g. laptop-cli')} /></Field>
          <Field label={tr('过期天数', 'Expires in days')} hint={tr('0 表示永不过期', '0 means never')}><Input type="number" min={0} max={3650} step={1} value={expires} onChange={(event) => setExpires(event.target.value)} /></Field>
        </form>
      </Modal>

      <Modal open={!!revealed} title={tr('保管好你的 Token', 'Save this token now')} onClose={() => setRevealed('')} closeOnMask={false} footer={<Button variant="primary" onClick={() => setRevealed('')}>{tr('我已保存', 'I have saved it')}</Button>}>
        <Notice tone="warning">{tr('此 Token 明文仅显示一次，关闭后无法再次查看。', 'This plaintext token is shown only once.')}</Notice>
        <div className="settings-token-reveal">{revealed}</div>
        <div className="native-copy-row"><Button onClick={async () => { await navigator.clipboard.writeText(revealed); showNotice('success', tr('已复制', 'Copied')); }}><Copy size={14} />{tr('复制', 'Copy')}</Button></div>
      </Modal>

      <Modal open={!!revokeTarget} title={tr('撤销 Token', 'Revoke token')} onClose={() => setRevokeTarget(undefined)} width={440} footer={<><Button onClick={() => setRevokeTarget(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" onClick={handleRevoke}>{tr('撤销', 'Revoke')}</Button></>}>
        <p className="native-confirm-copy">{tr('撤销后使用此 Token 的客户端将立即失效。', 'Clients using this token will stop working immediately.')}</p>
      </Modal>
    </div>
  );
};

export default SettingsPage;
