import { createAPIToken } from '@/services/api';
import { APP_NAME } from '@/constants';
import { useI18n } from '@/i18n';
import { history, useModel } from '@umijs/max';
import { App, Button, Spin } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import './index.less';

const CALLBACK_RE = /^http:\/\/(127\.0\.0\.1|localhost):\d+\/callback$/;

interface CliParams {
  callback: string;
  state: string;
  name: string;
  manual: boolean; // true when mode=manual (headless, no callback)
}

function parseQuery(): CliParams | { error: string } {
  const q = new URLSearchParams(window.location.search);
  const mode = q.get('mode') || '';
  const name = q.get('name') || '';

  // Manual: headless CLI — just show the token on screen to copy.
  if (mode === 'manual') {
    if (!name || name.length > 64) {
      return { error: 'token 名称缺失或过长' };
    }
    return { callback: '', state: '', name, manual: true };
  }

  // Callback: browser on same machine, redirect back to localhost.
  const callback = q.get('callback') || '';
  const state = q.get('state') || '';
  if (!callback || !CALLBACK_RE.test(callback)) {
    return { error: 'callback URL 必须指向本机回环地址' };
  }
  if (!state || state.length < 16) {
    return { error: 'state 参数缺失或过短' };
  }
  if (!name || name.length > 64) {
    return { error: 'token 名称缺失或过长' };
  }
  return { callback, state, name, manual: false };
}

const CliAuthPage: React.FC = () => {
  const { tr } = useI18n();
  const { message } = App.useApp();
  const { initialState, setInitialState } = useModel('@@initialState');
  const [submitting, setSubmitting] = useState(false);
  const [phase, setPhase] = useState<'idle' | 'approved' | 'denied'>('idle');
  const [manualToken, setManualToken] = useState('');
  // 'checking' until we verify auth state — without this the first render
  // after Login.history.push lands here while the model's setInitialState
  // from Login is still in flight, and we would bounce straight back to
  // /login.
  const [authState, setAuthState] = useState<'checking' | 'authed' | 'noauth'>('checking');

  const parsed = useMemo(() => parseQuery(), []);
  const isError = 'error' in parsed;

  useEffect(() => {
    if (isError) return;
    let cancelled = false;

    (async () => {
      if (initialState?.currentUser) {
        setAuthState('authed');
        return;
      }

      // Slow path: authoritative check. Login.persistLogin writes
      // localStorage.token synchronously before history.push, so by the time
      // we mount the token is present.
      const fetchFn = initialState?.fetchUserInfo;
      const user = fetchFn ? await fetchFn() : undefined;
      if (cancelled) return;

      if (user) {
        await setInitialState((s) => ({ ...s, currentUser: user }));
        setAuthState('authed');
        return;
      }

      // Definitely not logged in — go through /login and come back.
      const target = `/cli-auth${window.location.search}`;
      history.push(`/login?redirect=${encodeURIComponent(target)}`);
    })();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (isError) {
    return (
      <div className="cli-auth-container">
        <div className="cli-auth-panel">
          <div className="cli-auth-header">
            <img src="/liaison.png" alt="Liaison" className="cli-auth-logo" />
            <span className="cli-auth-title">{APP_NAME}</span>
          </div>
          <div className="cli-auth-state cli-auth-state--error">
            <h2>{tr('请求无效', 'Invalid request')}</h2>
            <p>{(parsed as { error: string }).error}</p>
            <p className="cli-auth-hint">
              {tr('请回到 CLI 重新执行 ', 'Return to your CLI and re-run ')}
              <code>liaison login</code>
            </p>
          </div>
        </div>
      </div>
    );
  }

  const params = parsed as CliParams;

  const handleApprove = async () => {
    setSubmitting(true);
    try {
      const res = await createAPIToken({ name: params.name, expires_in_days: 0 });
      if (res.code === 200 && res.data?.token) {
        if (params.manual) {
          setManualToken(res.data.token);
          setPhase('approved');
          return;
        }
        const url = `${params.callback}?state=${encodeURIComponent(params.state)}&token=${encodeURIComponent(res.data.token)}`;
        setPhase('approved');
        setTimeout(() => {
          window.location.href = url;
        }, 600);
        return;
      }
      message.error(res.message || tr('创建 token 失败', 'Failed to create token'));
    } catch (err: any) {
      message.error(err?.message || tr('创建 token 失败', 'Failed to create token'));
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeny = () => {
    setPhase('denied');
    if (!params.manual) {
      const url = `${params.callback}?state=${encodeURIComponent(params.state)}&error=denied`;
      setTimeout(() => {
        window.location.href = url;
      }, 400);
    }
  };

  if (authState !== 'authed' || !initialState?.currentUser) {
    return (
      <div className="cli-auth-container">
        <Spin />
      </div>
    );
  }

  if (phase === 'approved') {
    return (
      <div className="cli-auth-container">
        <div className="cli-auth-panel">
          <div className="cli-auth-header">
            <img src="/liaison.png" alt="Liaison" className="cli-auth-logo" />
            <span className="cli-auth-title">{APP_NAME}</span>
          </div>
          <div className="cli-auth-state cli-auth-state--success">
            <h2>{tr('已授权', 'Authorized')}</h2>
            {manualToken ? (
              <>
                <p>{tr('复制下面的 Token，粘贴回 CLI 终端：', 'Copy the token below and paste it into your CLI:')}</p>
                <div className="cli-auth-token-display">
                  <code className="cli-auth-token-value">{manualToken}</code>
                  <Button
                    size="small"
                    onClick={() => {
                      navigator.clipboard.writeText(manualToken);
                      message.success(tr('已复制', 'Copied'));
                    }}
                  >
                    {tr('复制', 'Copy')}
                  </Button>
                </div>
                <p className="cli-auth-hint">
                  {tr('Token 仅显示一次，关闭此页面后无法再次查看。', 'This token is shown only once. It cannot be viewed again after closing this page.')}
                </p>
              </>
            ) : (
              <p>{tr('正在跳回 CLI，可以关闭此页面', 'Returning to your CLI — you can close this tab')}</p>
            )}
          </div>
        </div>
      </div>
    );
  }

  if (phase === 'denied') {
    return (
      <div className="cli-auth-container">
        <div className="cli-auth-panel">
          <div className="cli-auth-header">
            <img src="/liaison.png" alt="Liaison" className="cli-auth-logo" />
            <span className="cli-auth-title">{APP_NAME}</span>
          </div>
          <div className="cli-auth-state cli-auth-state--denied">
            <h2>{tr('已拒绝', 'Denied')}</h2>
            <p>{tr('CLI 不会获得任何 token', 'The CLI did not receive a token')}</p>
          </div>
        </div>
      </div>
    );
  }

  const account = initialState.currentUser.email || initialState.currentUser.name;

  return (
    <div className="cli-auth-container">
      <div className="cli-auth-panel">
        <div className="cli-auth-header">
          <img src="/liaison.png" alt="Liaison" className="cli-auth-logo" />
          <span className="cli-auth-title">{APP_NAME}</span>
        </div>

        <h2 className="cli-auth-heading">{tr('登录命令行', 'Sign in to the CLI')}</h2>
        <p className="cli-auth-subheading">
          {tr(
            '为命令行创建一个长期访问令牌，可以随时在设置里撤销。',
            'Create a long-lived access token for the CLI. You can revoke it any time in Settings.',
          )}
        </p>

        <dl className="cli-auth-meta">
          <div>
            <dt>{tr('账号', 'Account')}</dt>
            <dd>{account}</dd>
          </div>
          <div>
            <dt>{tr('令牌名称', 'Token')}</dt>
            <dd><code>{params.name}</code></dd>
          </div>
          {!params.manual && (
            <div>
              <dt>{tr('回调地址', 'Callback')}</dt>
              <dd><code>{params.callback}</code></dd>
            </div>
          )}
        </dl>

        <div className="cli-auth-actions">
          <Button size="large" onClick={handleDeny}>
            {tr('取消', 'Cancel')}
          </Button>
          <Button
            size="large"
            type="primary"
            onClick={handleApprove}
            loading={submitting}
            className="cli-auth-primary-btn"
          >
            {tr('授权登录', 'Authorize')}
          </Button>
        </div>
      </div>
    </div>
  );
};

export default CliAuthPage;
