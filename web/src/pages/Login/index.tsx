import { LiaisonLogo } from '@/components/LiaisonLogo';
import { GITHUB_URL } from '@/constants';
import { useI18n } from '@/i18n';
import { history } from '@/lib/runtime';
import { login } from '@/services/api';
import { useSession } from '@/store/session';
import { Activity, Cable, Github, Lock, ShieldCheck, User } from 'lucide-react';
import { FormEvent, useState } from 'react';
import './index.less';

type LoginValues = { email: string; password: string };

const Login: React.FC = () => {
  const { tr } = useI18n();
  const setToken = useSession((state) => state.setToken);
  const setInitialState = useSession((state) => state.setInitialState);

  const [values, setValues] = useState<LoginValues>({ email: '', password: '' });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!values.email.trim() || !values.password) {
      setError(tr('请输入邮箱和密码', 'Enter your email and password'));
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      const result = await login(values);
      if (result.code !== 200 || !result.data?.token) {
        setError(result.message || tr('登录失败', 'Login failed'));
        return;
      }

      setToken(result.data.token);
      await setInitialState((state) => ({
        ...state,
        currentUser: result.data?.user,
      }));
      const redirect = new URL(window.location.href).searchParams.get(
        'redirect',
      );
      history.replace(redirect || '/dashboard');
    } catch (error: any) {
      const backendMessage = error?.response?.data?.message;
      setError(
        backendMessage ||
          error?.message ||
          tr('登录失败，请重试', 'Login failed, please retry'),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="login-shell">
      <section className="login-brand-panel">
        <LiaisonLogo size={440} className="login-brand-watermark" decorative />
        <div className="login-brand-topline">
          <LiaisonLogo size={42} className="login-brand-logo" />
          <span className="login-brand-wordmark">Liaison</span>
        </div>
        <div className="login-brand-message">
          <span className="login-kicker">ZERO TRUST NETWORK ACCESS</span>
          <h1>
            {tr(
              '让每一个私有应用，都以零信任方式接入',
              'Zero-trust access for every private application',
            )}
          </h1>
          <p>
            {tr(
              '通过连接器统一接入分布在家庭、办公室与数据中心的设备和应用，无需暴露内网端口。',
              'Connect devices and applications across home, office, and data center environments without exposing private network ports.',
            )}
          </p>
        </div>
        <div className="login-capabilities">
          <div>
            <Cable size={16} />
            <span>{tr('连接器隧道', 'Connector tunnels')}</span>
          </div>
          <div>
            <ShieldCheck size={16} />
            <span>{tr('策略边界', 'Policy boundaries')}</span>
          </div>
          <div>
            <Activity size={16} />
            <span>{tr('访问审计', 'Access audit')}</span>
          </div>
        </div>
      </section>

      <section className="login-form-panel">
        <div className="login-form-card">
          <div className="login-form-heading">
            <div className="login-form-title">
              <LiaisonLogo size={36} />
              <h2>Liaison</h2>
            </div>
          </div>
          <form className="login-native-form" onSubmit={handleSubmit} autoComplete="on">
            <label className="login-field">
              <span>{tr('邮箱', 'Email')}</span>
              <span className="login-control">
                <User size={17} strokeWidth={1.7} />
                <input
                  type="email"
                  value={values.email}
                  onChange={(event) => setValues((current) => ({ ...current, email: event.target.value }))}
                  placeholder="name@example.com"
                  autoComplete="username"
                />
              </span>
            </label>
            <label className="login-field">
              <span>{tr('密码', 'Password')}</span>
              <span className="login-control">
                <Lock size={17} strokeWidth={1.7} />
                <input
                  type="password"
                  value={values.password}
                  onChange={(event) => setValues((current) => ({ ...current, password: event.target.value }))}
                  placeholder={tr('输入登录密码', 'Enter password')}
                  autoComplete="current-password"
                />
              </span>
            </label>
            {error ? <p className="login-error" role="alert">{error}</p> : null}
            <button className="login-submit" type="submit" disabled={submitting}>
              {submitting ? tr('登录中…', 'Signing in…') : tr('登录', 'Sign in')}
            </button>
          </form>
          <div className="login-form-footer">
            <span>© 2026 Liaison</span>
            <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">
              <Github size={14} /> GitHub
            </a>
          </div>
        </div>
      </section>
    </main>
  );
};

export default Login;
