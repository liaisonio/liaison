import { LockOutlined, UserOutlined, GithubOutlined } from '@ant-design/icons';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { history, useModel } from '@umijs/max';
import { App } from 'antd';
import { login } from '@/services/api';
import { APP_NAME } from '@/constants';
import { useI18n } from '@/i18n';
import './index.less';

const GITHUB_URL = 'https://github.com/singchia/liaison';

const Login: React.FC = () => {
  const { message } = App.useApp();
  const { setInitialState } = useModel('@@initialState');
  const { tr } = useI18n();

  const handleSubmit = async (values: { email: string; password: string }) => {
    try {
      const result = await login(values);
      if (result.code === 200 && result.data?.token) {
        localStorage.setItem('token', result.data.token);
        message.success(tr('登录成功！', 'Login successful'));

        setInitialState((s) => ({
          ...s,
          currentUser: result.data?.user,
        }));

        const urlParams = new URL(window.location.href).searchParams;
        history.push(urlParams.get('redirect') || '/');
        return;
      }
      message.error(result.message || tr('登录失败', 'Login failed'));
    } catch (error: any) {
      message.error(error?.message || tr('登录失败，请重试！', 'Login failed, please retry'));
    }
  };

  return (
    <div className="login-container">
      <div className="login-content">
        <div className="login-header">
          <img 
            src="/liaison.png" 
            alt="Liaison" 
            className="login-logo-img"
          />
          <span className="login-title">{APP_NAME}</span>
        </div>
        
        <LoginForm
          style={{
            minWidth: 280,
            maxWidth: '75vw',
          }}
          submitter={{
            searchConfig: {
              submitText: tr('登录', 'Login'),
            },
          }}
          onFinish={handleSubmit}
        >
          <ProFormText
            name="email"
            fieldProps={{
              size: 'large',
              prefix: <UserOutlined className="prefixIcon" />,
            }}
            placeholder={tr('邮箱', 'Email')}
            initialValue=""
            rules={[
              {
                required: true,
                message: tr('请输入邮箱!', 'Please input email'),
              },
              {
                type: 'email',
                message: tr('请输入有效的邮箱地址!', 'Please input a valid email'),
              },
            ]}
          />
          <ProFormText.Password
            name="password"
            fieldProps={{
              size: 'large',
              prefix: <LockOutlined className="prefixIcon" />,
            }}
            placeholder={tr('密码', 'Password')}
            initialValue=""
            rules={[
              {
                required: true,
                message: tr('请输入密码！', 'Please input password'),
              },
            ]}
          />
        </LoginForm>
        
        <div className="login-footer">
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <span>© 2026 {APP_NAME}. All rights reserved.</span>
              <a
                href={GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                style={{ 
                  color: '#1677ff',
                  textDecoration: 'none',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 4,
                  transition: 'opacity 0.3s'
                }}
                onMouseEnter={(e) => e.currentTarget.style.opacity = '0.8'}
                onMouseLeave={(e) => e.currentTarget.style.opacity = '1'}
              >
                <GithubOutlined />
                <span>GitHub</span>
              </a>
            </div>
            <div>备案号：浙ICP备2026007675号-1</div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Login;
