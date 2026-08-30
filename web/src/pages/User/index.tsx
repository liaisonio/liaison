import { Button, Field, Input, Notice } from '@/components/ui';
import { useI18n } from '@/i18n';
import { useModel } from '@/lib/runtime';
import { changePassword } from '@/services/api';
import { getAccountLabel, getAvatarIdentity, useAvatarStore } from '@/store/avatar';
import { Camera, Lock, ShieldCheck, Trash2, User } from 'lucide-react';
import { ChangeEvent, FormEvent, useRef, useState } from 'react';
import '../Settings/index.less';

type PasswordValues = { oldPassword: string; newPassword: string; confirmPassword: string };
const emptyPasswords: PasswordValues = { oldPassword: '', newPassword: '', confirmPassword: '' };

const UserPage: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const { tr } = useI18n();
  const [active, setActive] = useState<'account' | 'password'>('account');
  const [passwords, setPasswords] = useState(emptyPasswords);
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const currentUser = initialState?.currentUser;
  const identity = getAvatarIdentity(currentUser);
  const localAvatar = useAvatarStore((state) => state.avatars[identity]);
  const setAvatar = useAvatarStore((state) => state.setAvatar);
  const removeAvatar = useAvatarStore((state) => state.removeAvatar);
  const avatar = localAvatar || currentUser?.avatar;
  const accountLabel = getAccountLabel(currentUser) || tr('用户', 'User');

  const showNotice = (tone: 'danger' | 'success', text: string) => {
    setNotice({ tone, text });
    window.setTimeout(() => setNotice(undefined), 3200);
  };

  const handleAvatarChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      showNotice('danger', tr('请选择 JPG、PNG 或 WebP 图片', 'Choose a JPG, PNG or WebP image'));
      return;
    }
    if (file.size > 3 * 1024 * 1024) {
      showNotice('danger', tr('图片不能超过 3 MB', 'Image must be under 3 MB'));
      return;
    }
    const reader = new FileReader();
    reader.onerror = () => showNotice('danger', tr('无法读取图片', 'Unable to read the image'));
    reader.onload = () => {
      const image = new Image();
      image.onerror = () => showNotice('danger', tr('无法处理图片', 'Unable to process the image'));
      image.onload = () => {
        const size = Math.min(image.naturalWidth, image.naturalHeight);
        const canvas = document.createElement('canvas');
        canvas.width = 256;
        canvas.height = 256;
        const context = canvas.getContext('2d');
        if (!context) return;
        context.drawImage(image, (image.naturalWidth - size) / 2, (image.naturalHeight - size) / 2, size, size, 0, 0, 256, 256);
        // Preserve transparency. Encoding an uploaded PNG as JPEG turns its
        // transparent edge black, which is especially visible in light mode.
        setAvatar(identity, canvas.toDataURL('image/png'));
        showNotice('success', tr('头像已更新', 'Avatar updated'));
      };
      image.src = String(reader.result);
    };
    reader.readAsDataURL(file);
  };

  const handleChangePassword = async (event: FormEvent) => {
    event.preventDefault();
    if (!passwords.oldPassword || !passwords.newPassword || !passwords.confirmPassword) {
      showNotice('danger', tr('请填写全部密码字段', 'Complete all password fields'));
      return;
    }
    if (passwords.newPassword.length < 8 || !/[A-Za-z]/.test(passwords.newPassword) || !/\d/.test(passwords.newPassword)) {
      showNotice('danger', tr('新密码至少 8 位，并包含字母和数字', 'Use 8+ characters with letters and numbers'));
      return;
    }
    if (passwords.newPassword !== passwords.confirmPassword) {
      showNotice('danger', tr('两次输入的新密码不一致', 'New passwords do not match'));
      return;
    }
    setPasswordLoading(true);
    try {
      const response = await changePassword({ old_password: passwords.oldPassword, new_password: passwords.newPassword });
      if (response.code !== 200) throw new Error(response.message);
      setPasswords(emptyPasswords);
      showNotice('success', tr('密码修改成功', 'Password changed successfully'));
    } catch (error: any) {
      showNotice('danger', error?.message || tr('密码修改失败', 'Failed to change password'));
    } finally {
      setPasswordLoading(false);
    }
  };

  const roleLabel = currentUser?.role || tr('用户', 'User');
  const details = [
    [tr('角色', 'Role'), roleLabel],
    [tr('注册时间', 'Created At'), currentUser?.created_at || '-'],
    [tr('最后登录', 'Last Login'), currentUser?.last_login_at || '-'],
    [tr('登录 IP', 'Login IP'), currentUser?.last_login_ip || '-'],
  ];

  return (
    <div className="settings-page native-settings-page">
      {notice ? <div className="settings-floating-notice"><Notice tone={notice.tone}>{notice.text}</Notice></div> : null}
      <div className="settings-shell native-settings-shell">
        <aside className="native-settings-tabs">
          <button className={active === 'account' ? 'is-active' : ''} onClick={() => setActive('account')}><User size={15} />{tr('账户信息', 'Account')}</button>
          <button className={active === 'password' ? 'is-active' : ''} onClick={() => setActive('password')}><Lock size={15} />{tr('修改密码', 'Password')}</button>
        </aside>
        <main className="native-settings-content">
          {active === 'account' ? (
            <section className="settings-section native-card">
              <div className="user-profile">
                <div className="user-avatar-editor">
                  <div className="native-avatar">{avatar ? <img src={avatar} alt="" /> : accountLabel.slice(0, 1).toUpperCase()}</div>
                  <button type="button" className="user-avatar-edit" aria-label={tr('更换头像', 'Change avatar')} onClick={() => avatarInputRef.current?.click()}><Camera size={13} /></button>
                  <input ref={avatarInputRef} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={handleAvatarChange} />
                </div>
                <div className="user-info">
                  <div className="user-name-row"><h3>{accountLabel}</h3><span>{roleLabel}</span></div>
                  <p>{currentUser?.email || '-'}</p>
                  <small>{tr('用于登录 Liaison 与识别操作记录', 'Used to sign in and identify account activity')}</small>
                </div>
                <div className="user-avatar-actions">
                  <Button variant="ghost" onClick={() => avatarInputRef.current?.click()}><Camera size={14} />{tr('更换头像', 'Change avatar')}</Button>
                  {avatar ? <Button variant="ghost" onClick={() => removeAvatar(identity)}><Trash2 size={14} />{tr('移除', 'Remove')}</Button> : null}
                </div>
              </div>
              <dl className="native-description-grid">{details.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
            </section>
          ) : (
            <section className="settings-section native-card">
              <div className="password-tips"><ShieldCheck size={20} /><div><strong>{tr('密码安全', 'Password security')}</strong><span>{tr('密码至少 8 位，并同时包含字母和数字。', 'Use at least 8 characters with both letters and numbers.')}</span></div></div>
              <form className="native-password-form" onSubmit={handleChangePassword}>
                <Field label={tr('当前密码', 'Current password')} required><Input type="password" value={passwords.oldPassword} onChange={(event) => setPasswords((value) => ({ ...value, oldPassword: event.target.value }))} /></Field>
                <Field label={tr('新密码', 'New password')} required><Input type="password" value={passwords.newPassword} onChange={(event) => setPasswords((value) => ({ ...value, newPassword: event.target.value }))} /></Field>
                <Field label={tr('确认新密码', 'Confirm new password')} required><Input type="password" value={passwords.confirmPassword} onChange={(event) => setPasswords((value) => ({ ...value, confirmPassword: event.target.value }))} /></Field>
                <Button type="submit" variant="primary" disabled={passwordLoading}>{passwordLoading ? tr('提交中…', 'Saving…') : tr('修改密码', 'Change password')}</Button>
              </form>
            </section>
          )}
        </main>
      </div>
    </div>
  );
};

export default UserPage;
