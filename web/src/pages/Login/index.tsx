import { LiaisonLogo } from '@/components/LiaisonLogo';
import { GITHUB_URL } from '@/constants';
import { useI18n } from '@/i18n';
import { history } from '@/lib/runtime';
import { login } from '@/services/api';
import { useSession } from '@/store/session';
import { GithubOutlined, LockOutlined, UserOutlined } from '@ant-design/icons';
import { App, Button, Form, Input } from 'antd';
import { Activity, Cable, ShieldCheck } from 'lucide-react';
import './index.less';

type LoginValues = { email: string; password: string };

const Login: React.FC = () => {
  const { message } = App.useApp();
  const { tr } = useI18n();
  const setToken = useSession((state) => state.setToken);
  const setInitialState = useSession((state) => state.setInitialState);

  const handleSubmit = async (values: LoginValues) => {
    try {
      const result = await login(values);
      if (result.code !== 200 || !result.data?.token) {
        message.error(result.message || tr('登录失败', 'Login failed'));
        return;
      }

      setToken(result.data.token);
      await setInitialState((state) => ({
        ...state,
        currentUser: result.data?.user,
      }));
      message.success(tr('登录成功', 'Signed in'));
      const redirect = new URL(window.location.href).searchParams.get(
        'redirect',
      );
      history.replace(redirect || '/dashboard');
    } catch (error: any) {
      const backendMessage = error?.response?.data?.message;
      message.error(
        backendMessage ||
          error?.message ||
          tr('登录失败，请重试', 'Login failed, please retry'),
      );
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
          <Form<LoginValues>
            layout="vertical"
            requiredMark={false}
            onFinish={handleSubmit}
            autoComplete="on"
          >
            <Form.Item
              name="email"
              label={tr('邮箱', 'Email')}
              rules={[
                {
                  required: true,
                  message: tr('请输入邮箱', 'Enter your email'),
                },
                {
                  type: 'email',
                  message: tr('请输入有效邮箱', 'Enter a valid email'),
                },
              ]}
            >
              <Input
                size="large"
                prefix={<UserOutlined />}
                placeholder="name@example.com"
                autoComplete="username"
              />
            </Form.Item>
            <Form.Item
              name="password"
              label={tr('密码', 'Password')}
              rules={[
                {
                  required: true,
                  message: tr('请输入密码', 'Enter your password'),
                },
              ]}
            >
              <Input.Password
                size="large"
                prefix={<LockOutlined />}
                placeholder={tr('输入登录密码', 'Enter password')}
                autoComplete="current-password"
              />
            </Form.Item>
            <Form.Item className="login-submit-row">
              <Button type="primary" htmlType="submit" size="large" block>
                {tr('登录', 'Sign in')}
              </Button>
            </Form.Item>
          </Form>
          <div className="login-form-footer">
            <span>© 2026 Liaison</span>
            <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">
              <GithubOutlined /> GitHub
            </a>
          </div>
        </div>
      </section>
    </main>
  );
};

export default Login;
