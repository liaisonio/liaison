import { useI18n } from '@/i18n';
import { useModel } from '@/lib/runtime';
import { changePassword } from '@/services/api';
import {
  getAccountLabel,
  getAvatarIdentity,
  useAvatarStore,
} from '@/store/avatar';
import { executeAction } from '@/utils/request';
import {
  CameraOutlined,
  DeleteOutlined,
  LockOutlined,
  SafetyOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import {
  App,
  Avatar,
  Button,
  Card,
  Descriptions,
  Divider,
  Form,
  Input,
  Tabs,
  Typography,
} from 'antd';
import { ChangeEvent, useRef, useState } from 'react';
import '../Settings/index.less';

const { Title, Text } = Typography;

const UserPage: React.FC = () => {
  const { message } = App.useApp();
  const { initialState } = useModel('@@initialState');
  const { tr } = useI18n();
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [passwordForm] = Form.useForm();
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const currentUser = initialState?.currentUser;
  const identity = getAvatarIdentity(currentUser);
  const localAvatar = useAvatarStore((state) => state.avatars[identity]);
  const setAvatar = useAvatarStore((state) => state.setAvatar);
  const removeAvatar = useAvatarStore((state) => state.removeAvatar);
  const avatar = localAvatar || currentUser?.avatar;
  const accountLabel = getAccountLabel(currentUser) || tr('用户', 'User');

  const handleAvatarChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      message.error(
        tr('请选择 JPG、PNG 或 WebP 图片', 'Choose a JPG, PNG or WebP image'),
      );
      return;
    }
    if (file.size > 3 * 1024 * 1024) {
      message.error(tr('图片不能超过 3 MB', 'Image must be under 3 MB'));
      return;
    }

    const reader = new FileReader();
    reader.onerror = () =>
      message.error(tr('无法读取图片', 'Unable to read the image'));
    reader.onload = () => {
      const image = new Image();
      image.onerror = () =>
        message.error(tr('无法处理图片', 'Unable to process the image'));
      image.onload = () => {
        const size = Math.min(image.naturalWidth, image.naturalHeight);
        const canvas = document.createElement('canvas');
        canvas.width = 256;
        canvas.height = 256;
        const context = canvas.getContext('2d');
        if (!context) return;
        context.drawImage(
          image,
          (image.naturalWidth - size) / 2,
          (image.naturalHeight - size) / 2,
          size,
          size,
          0,
          0,
          256,
          256,
        );
        setAvatar(identity, canvas.toDataURL('image/jpeg', 0.9));
        message.success(tr('头像已更新', 'Avatar updated'));
      };
      image.src = String(reader.result);
    };
    reader.readAsDataURL(file);
  };

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
              <div className="user-avatar-editor">
                <Avatar size={56} icon={<UserOutlined />} src={avatar}>
                  {accountLabel.slice(0, 1).toUpperCase()}
                </Avatar>
                <button
                  type="button"
                  className="user-avatar-edit"
                  aria-label={tr('更换头像', 'Change avatar')}
                  title={tr('更换头像', 'Change avatar')}
                  onClick={() => avatarInputRef.current?.click()}
                >
                  <CameraOutlined />
                </button>
                <input
                  ref={avatarInputRef}
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  hidden
                  onChange={handleAvatarChange}
                />
              </div>
              <div className="user-info">
                <Title level={4}>{accountLabel}</Title>
                <Text type="secondary">{currentUser?.email || '-'}</Text>
                <div className="user-avatar-actions">
                  <Button
                    type="link"
                    size="small"
                    icon={<CameraOutlined />}
                    onClick={() => avatarInputRef.current?.click()}
                  >
                    {tr('更换头像', 'Change avatar')}
                  </Button>
                  {avatar && (
                    <Button
                      type="link"
                      size="small"
                      danger
                      icon={<DeleteOutlined />}
                      onClick={() => removeAvatar(identity)}
                    >
                      {tr('移除', 'Remove')}
                    </Button>
                  )}
                </div>
              </div>
            </div>

            <Divider />

            <Descriptions
              column={{ xs: 1, sm: 1, md: 2 }}
              styles={{ label: { fontWeight: 500 } }}
            >
              <Descriptions.Item label={tr('用户名', 'Username')}>
                {accountLabel}
              </Descriptions.Item>
              <Descriptions.Item label={tr('邮箱', 'Email')}>
                {currentUser?.email || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('角色', 'Role')}>
                {initialState?.currentUser?.role || tr('用户', 'User')}
              </Descriptions.Item>
              <Descriptions.Item label={tr('注册时间', 'Created At')}>
                {initialState?.currentUser?.created_at || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('最后登录', 'Last Login')}>
                {initialState?.currentUser?.last_login_at || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('登录 IP', 'Login IP')}>
                {initialState?.currentUser?.last_login_ip || '-'}
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
              <SafetyOutlined />
              <div>
                <Text strong>{tr('密码安全', 'Password security')}</Text>
                <br />
                <Text type="secondary">
                  {tr(
                    '密码至少 8 位，并同时包含字母和数字。',
                    'Use at least 8 characters with both letters and numbers.',
                  )}
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
                label={tr('当前密码', 'Current password')}
                rules={[
                  {
                    required: true,
                    message: tr('请输入当前密码', 'Enter the current password'),
                  },
                ]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请输入当前密码', 'Current password')}
                />
              </Form.Item>
              <Form.Item
                name="newPassword"
                label={tr('新密码', 'New password')}
                rules={[
                  {
                    required: true,
                    message: tr('请输入新密码', 'Enter a new password'),
                  },
                  {
                    min: 8,
                    message: tr(
                      '密码长度至少 8 位',
                      'Use at least 8 characters',
                    ),
                  },
                  {
                    pattern: /^(?=.*[A-Za-z])(?=.*\d)/,
                    message: tr(
                      '密码必须包含字母和数字',
                      'Include both letters and numbers',
                    ),
                  },
                ]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请输入新密码', 'New password')}
                />
              </Form.Item>
              <Form.Item
                name="confirmPassword"
                label={tr('确认新密码', 'Confirm new password')}
                dependencies={['newPassword']}
                rules={[
                  {
                    required: true,
                    message: tr('请确认新密码', 'Confirm the new password'),
                  },
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (!value || getFieldValue('newPassword') === value) {
                        return Promise.resolve();
                      }
                      return Promise.reject(
                        new Error(
                          tr('两次输入的密码不一致', 'Passwords do not match'),
                        ),
                      );
                    },
                  }),
                ]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请再次输入新密码', 'Confirm new password')}
                />
              </Form.Item>
              <Form.Item>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={passwordLoading}
                >
                  {tr('修改密码', 'Change password')}
                </Button>
              </Form.Item>
            </Form>
          </Card>
        </div>
      ),
    },
  ];

  return (
    <PageContainer className="settings-page">
      <Card variant="borderless" className="settings-shell">
        <Tabs items={items} tabPosition="left" className="settings-tabs" />
      </Card>
    </PageContainer>
  );
};

export default UserPage;
