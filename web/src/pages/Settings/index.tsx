import { PageContainer } from '@ant-design/pro-components';
import {
  Card,
  Tabs,
  Form,
  Input,
  Button,
  App,
  Descriptions,
  Avatar,
  Typography,
  Divider,
  Upload,
} from 'antd';
import {
  UserOutlined,
  LockOutlined,
  SafetyOutlined,
  GithubOutlined,
  InfoCircleOutlined,
  CameraOutlined,
} from '@ant-design/icons';
import { useState } from 'react';
import { useModel } from '@umijs/max';
import { changePassword, uploadAvatar } from '@/services/api';
import { executeAction } from '@/utils/request';
import { APP_NAME, getAvatarUrl } from '@/constants';
import { useI18n } from '@/i18n';
import './index.less';

const { Title, Text, Link } = Typography;
const GITHUB_URL = 'https://github.com/singchia/liaison';

const SettingsPage: React.FC = () => {
  const { message } = App.useApp();
  const { initialState, setInitialState } = useModel('@@initialState');
  const { tr } = useI18n();
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [avatarLoading, setAvatarLoading] = useState(false);
  const [passwordForm] = Form.useForm();

  const handleChangePassword = async (values: {
    oldPassword: string;
    newPassword: string;
    confirmPassword: string;
  }) => {
    if (values.newPassword !== values.confirmPassword) {
      message.error(tr('两次输入的新密码不一致', 'New passwords do not match'));
      return;
    }

    setPasswordLoading(true);
    await executeAction(
      () =>
        changePassword({
          old_password: values.oldPassword,
          new_password: values.newPassword,
        }),
      {
        successMessage: tr('密码修改成功', 'Password changed successfully'),
        errorMessage: tr('密码修改失败', 'Failed to change password'),
        onSuccess: () => passwordForm.resetFields(),
      },
    );
    setPasswordLoading(false);
  };

  const handleUploadAvatar = async (file: File) => {
    const allowed = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
    if (!allowed.includes(file.type)) {
      message.error(tr('请选择 JPG、PNG、GIF 或 WebP 图片', 'Please select JPG, PNG, GIF or WebP image'));
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      message.error(tr('图片大小不能超过 5MB', 'Image size must be under 5MB'));
      return;
    }
    setAvatarLoading(true);
    try {
      const res = await uploadAvatar(file);
      // 兼容 dataField 解包：返回可能是 { code, data } 或直接是用户对象
      const user = (res as any)?.data !== undefined ? (res as any).data : res;
      const ok = (res as any)?.code === 200 || (user && typeof user === 'object');
      if (ok && user && typeof user === 'object') {
        message.success(tr('头像已更新', 'Avatar updated'));
        const rawAvatar = (user as any).avatar ?? (user as any).Avatar ?? '';
        const avatarUrl = rawAvatar
          ? `${rawAvatar}${rawAvatar.includes('?') ? '&' : '?'}t=${Date.now()}`
          : rawAvatar;
        setInitialState((s) => ({
          ...s,
          currentUser: { ...user, avatar: avatarUrl },
        }));
      } else {
        message.error((res as any)?.message || tr('更新失败', 'Update failed'));
      }
    } catch (e: any) {
      message.error(e?.message || tr('更新头像失败', 'Failed to update avatar'));
    } finally {
      setAvatarLoading(false);
    }
  };

  const items = [
    {
      key: 'account',
      label: (
        <span>
          <UserOutlined />
          {tr('账户信息', 'Account')}
        </span>
      ),
      children: (
        <div className="settings-section">
          <Card variant="borderless">
            <div className="user-profile">
              <Avatar
                size={80}
                icon={<UserOutlined />}
                src={getAvatarUrl(initialState?.currentUser?.avatar)}
              />
              <div className="user-info">
                <Title level={4}>
                  {initialState?.currentUser?.name || 'Admin'}
                </Title>
                <Text type="secondary">
                  {initialState?.currentUser?.email || 'default@liaison.local'}
                </Text>
              </div>
            </div>
            <div style={{ marginTop: 16 }}>
              <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
                {tr('更新头像', 'Update avatar')}（{tr('从本机选择图片', 'Select image from device')}，JPG/PNG/GIF/WebP，≤5MB）
              </Text>
              <Upload
                accept="image/jpeg,image/png,image/gif,image/webp"
                showUploadList={false}
                beforeUpload={(file) => {
                  handleUploadAvatar(file);
                  return false;
                }}
              >
                <Button icon={<CameraOutlined />} loading={avatarLoading}>
                  {tr('选择图片', 'Choose image')}
                </Button>
              </Upload>
            </div>
            
            <Divider />
            
            <Descriptions
              column={{ xs: 1, sm: 1, md: 2 }}
              styles={{ label: { fontWeight: 500 } }}
            >
              <Descriptions.Item label={tr('用户名', 'Username')}>
                {initialState?.currentUser?.name || 'Admin'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('邮箱', 'Email')}>
                {initialState?.currentUser?.email || 'default@liaison.local'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('角色', 'Role')}>
                {initialState?.currentUser?.role || tr('管理员', 'Administrator')}
              </Descriptions.Item>
              <Descriptions.Item label={tr('注册时间', 'Created At')}>
                {initialState?.currentUser?.created_at || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('最后登录', 'Last Login')}>
                {initialState?.currentUser?.last_login || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('登录IP', 'Login IP')}>
                {initialState?.currentUser?.login_ip || '-'}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </div>
      ),
    },
    {
      key: 'password',
      label: (
        <span>
          <LockOutlined />
          {tr('修改密码', 'Password')}
        </span>
      ),
      children: (
        <div className="settings-section">
          <Card variant="borderless">
            <div className="password-tips">
              <SafetyOutlined className="text-blue-500 text-xl mr-2" />
              <div>
                <Text strong>{tr('密码安全提示', 'Password Security Tips')}</Text>
                <br />
                <Text type="secondary">
                  {tr('建议定期修改密码，密码长度至少8位，包含字母和数字', 'Use at least 8 characters and include letters and numbers')}
                </Text>
              </div>
            </div>
            
            <Divider />
            
            <Form
              form={passwordForm}
              layout="vertical"
              onFinish={handleChangePassword}
              className="password-form"
              requiredMark={false}
            >
              <Form.Item
                name="oldPassword"
                label={tr('当前密码', 'Current Password')}
                rules={[{ required: true, message: tr('请输入当前密码', 'Please input current password') }]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请输入当前密码', 'Please input current password')}
                />
              </Form.Item>

              <Form.Item
                name="newPassword"
                label={tr('新密码', 'New Password')}
                rules={[
                  { required: true, message: tr('请输入新密码', 'Please input new password') },
                  { min: 8, message: tr('密码长度至少8位', 'Password must be at least 8 characters') },
                  {
                    pattern: /^(?=.*[A-Za-z])(?=.*\d)/,
                    message: tr('密码必须包含字母和数字', 'Password must include letters and numbers'),
                  },
                ]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请输入新密码', 'Please input new password')}
                />
              </Form.Item>

              <Form.Item
                name="confirmPassword"
                label={tr('确认新密码', 'Confirm New Password')}
                dependencies={['newPassword']}
                rules={[
                  { required: true, message: tr('请确认新密码', 'Please confirm new password') },
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (!value || getFieldValue('newPassword') === value) {
                        return Promise.resolve();
                      }
                      return Promise.reject(new Error(tr('两次输入的密码不一致', 'Passwords do not match')));
                    },
                  }),
                ]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请再次输入新密码', 'Please input password again')}
                />
              </Form.Item>

              <Form.Item>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={passwordLoading}
                >
                  {tr('修改密码', 'Change Password')}
                </Button>
              </Form.Item>
            </Form>
          </Card>
        </div>
      ),
    },
    {
      key: 'about',
      label: (
        <span>
          <InfoCircleOutlined />
          {tr('关于', 'About')}
        </span>
      ),
      children: (
        <div className="settings-section">
          <Card variant="borderless">
            <Title level={4}>{tr('关于', 'About')} {APP_NAME}</Title>
            <Divider />
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
                <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>{tr('产品名称:', 'Product:')}</span>
                <span>{APP_NAME}</span>
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
                <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>GitHub:</span>
                <Link 
                  href={GITHUB_URL} 
                  target="_blank" 
                  rel="noopener noreferrer"
                  style={{ 
                    display: 'inline-flex',
                    alignItems: 'center',
                    wordBreak: 'break-all',
                    flex: 1
                  }}
                >
                  <GithubOutlined style={{ marginRight: 8, flexShrink: 0 }} />
                  <span>{GITHUB_URL}</span>
                </Link>
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
                <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>{tr('许可证:', 'License:')}</span>
                <span>Apache License 2.0</span>
              </div>
            </div>
          </Card>
        </div>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card variant="borderless">
        <Tabs
          items={items}
          tabPosition="left"
          className="settings-tabs"
        />
      </Card>
    </PageContainer>
  );
};

export default SettingsPage;
