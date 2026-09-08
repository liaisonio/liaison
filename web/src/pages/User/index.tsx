import { RequestError } from '@/api/client';
import { Button, DangerConfirm, DataTable, Field, Input, Modal, Notice, Select, Timestamp } from '@/components/ui';
import { useI18n } from '@/i18n';
import { useModel } from '@/lib/runtime';
import {
  addOrganizationMember, changePassword, createManagedUser, createOrganization,
  deleteManagedUser, deleteOrganization, getManagedUsers, getOrganizationMembers,
  getOrganizations, removeOrganizationMember, resetManagedUserPassword,
  updateManagedUser, updateOrganization, updateOrganizationMember,
} from '@/services/api';
import { getAccountLabel, getAvatarIdentity, useAvatarStore } from '@/store/avatar';
import { Building2, Camera, ChevronRight, KeyRound, Lock, Pencil, Plus, ShieldCheck, Trash2, User, UserMinus, Users } from 'lucide-react';
import { ChangeEvent, FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import '../Settings/index.less';
import { useFeature } from '@/store/permissions';
import Permissions from './Permissions';

type Tab = 'organizations' | 'users' | 'account' | 'password' | 'permissions';
type Notify = (tone: 'danger' | 'success', text: string) => void;

const UserPage: React.FC = () => {
  const organizationsAllowed = useFeature('organizations.read');
  const managePermissions = useFeature('permissions.manage');
  const { initialState } = useModel('@@initialState');
  const { tr } = useI18n();
  const [active, setActive] = useState<Tab>('organizations');
  useEffect(() => { if ((active === 'organizations' && !organizationsAllowed) || ((active === 'users' || active === 'permissions') && !managePermissions)) setActive('account'); }, [active, organizationsAllowed, managePermissions]);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  const notify: Notify = useCallback((tone, text) => {
    setNotice({ tone, text });
    window.setTimeout(() => setNotice(undefined), 3200);
  }, []);
  const tabs: Array<[Tab, React.ReactNode, string]> = [
    ['organizations', <Building2 size={15} />, tr('组织架构', 'Organizations')],
    ['users', <Users size={15} />, tr('用户', 'Users')],
    ['permissions', <ShieldCheck size={15} />, tr('权限策略', 'Permissions')],
    ['account', <User size={15} />, tr('我的账户', 'My account')],
    ['password', <Lock size={15} />, tr('修改密码', 'Password')],
  ];
  return <div className="settings-page native-settings-page user-management-page">
    {notice ? <div className="settings-floating-notice"><Notice tone={notice.tone}>{notice.text}</Notice></div> : null}
    <div className="settings-shell native-settings-shell user-management-shell">
      <aside className="native-settings-tabs">
        {tabs.filter(([key]) => key === 'organizations' ? organizationsAllowed : (key === 'users' || key === 'permissions') ? managePermissions : true).map(([key, icon, label]) => <span key={key} className={key === 'account' && (organizationsAllowed || managePermissions) ? 'user-tab-break' : undefined}><button className={active === key ? 'is-active' : ''} onClick={() => setActive(key)}>{icon}{label}</button></span>)}
      </aside>
      <main className="native-settings-content">
        {active === 'permissions' && managePermissions ? <Permissions /> : null}
        {active === 'organizations' && organizationsAllowed ? <OrganizationsPanel notify={notify} /> : null}
        {active === 'users' && managePermissions ? <UsersPanel currentUserId={initialState?.currentUser?.id} notify={notify} /> : null}
        {active === 'account' ? <AccountPanel /> : null}
        {active === 'password' ? <PasswordPanel notify={notify} /> : null}
      </main>
    </div>
  </div>;
};

function OrganizationsPanel({ notify }: { notify: Notify }) {
  const { tr } = useI18n();
  const [orgs, setOrgs] = useState<API.Organization[]>([]);
  const [selectedId, setSelectedId] = useState<number>();
  const [members, setMembers] = useState<API.OrganizationMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialog, setDialog] = useState<'create' | 'edit' | 'member' | 'delete'>();
  const loadOrganizations = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getOrganizations();
      const items = response.data?.organizations || [];
      setOrgs(items);
      setSelectedId((current) => items.some((item) => item.id === current) ? current : items[0]?.id);
    } catch (error: any) { notify('danger', error?.message || tr('组织加载失败', 'Failed to load organizations')); }
    finally { setLoading(false); }
  }, [notify, tr]);
  const loadMembers = useCallback(async (id: number) => {
    try { const response = await getOrganizationMembers(id); setMembers(response.data?.members || []); }
    catch (error: any) { setMembers([]); notify('danger', error?.message || tr('成员加载失败', 'Failed to load members')); }
  }, [notify, tr]);
  useEffect(() => { void loadOrganizations(); }, [loadOrganizations]);
  useEffect(() => { if (selectedId) void loadMembers(selectedId); }, [selectedId, loadMembers]);
  const selected = orgs.find((item) => item.id === selectedId);
  const tree = useMemo(() => flattenOrganizations(orgs), [orgs]);
  const roleLabel = (role: API.OrganizationRole) => ({ admin: tr('管理员', 'Admin'), user: tr('用户', 'User') })[role];
  return <section className="organization-manager">
    <header className="user-section-heading"><div><h2>{tr('组织架构', 'Organizations')}</h2><p>{tr('维护组织层级、成员与组织内角色', 'Manage hierarchy, members, and organization roles')}</p></div>{orgs.some((item) => item.can_manage) ? <Button variant="primary" onClick={() => setDialog('create')}><Plus size={13} />{tr('新建组织', 'New organization')}</Button> : null}</header>
    <div className="organization-layout">
      <aside className="organization-tree"><strong>{tr('组织', 'Organizations')}</strong><div className="organization-tree-list">
        {loading ? <span className="organization-empty">…</span> : tree.map(({ organization, depth }) => <button key={organization.id} className={organization.id === selectedId ? 'is-active' : ''} style={{ paddingLeft: 12 + depth * 17 }} onClick={() => setSelectedId(organization.id)}>{depth ? <ChevronRight size={11} /> : <Building2 size={13} />}<span>{organization.is_root ? tr('默认组织', 'Default organization') : organization.name}</span></button>)}
      </div></aside>
      <div className="organization-detail">{selected ? <>
        <header><div><h3>{selected.is_root ? tr('默认组织', 'Default organization') : selected.name}</h3><p>{selected.is_root ? tr('系统根组织', 'Root organization') : (selected.description || tr('暂无描述', 'No description'))}</p></div>{selected.can_manage ? <div className="organization-actions"><Button onClick={() => setDialog('edit')}><Pencil size={12} />{tr('编辑', 'Edit')}</Button>{selected.can_delete && !selected.is_root ? <Button variant="ghost" onClick={() => setDialog('delete')}><Trash2 size={12} />{tr('删除', 'Delete')}</Button> : null}</div> : null}</header>
        <div className="organization-members-heading"><div><strong>{tr('成员', 'Members')}</strong><span>{tr(`${members.length} 位成员`, `${members.length} members`)}</span></div>{selected.can_manage ? <Button onClick={() => setDialog('member')}><Plus size={12} />{tr('添加成员', 'Add member')}</Button> : null}</div>
        <DataTable rows={members} rowKey={(row) => row.id} emptyText={tr('该组织还没有成员', 'No members in this organization')} columns={[
          { key: 'user', title: tr('用户', 'User'), width: '38%', render: (row) => <MemberIdentity user={row.user} fallback={`#${row.user_id}`} /> },
          { key: 'role', title: tr('组织角色', 'Organization role'), width: 170, render: (row) => selected.can_manage ? <Select value={row.role} onChange={async (event) => { try { await updateOrganizationMember(selected.id, row.user_id, event.target.value as API.OrganizationRole); await loadMembers(selected.id); } catch (error: any) { notify('danger', error?.message); } }}><option value="admin">{roleLabel('admin')}</option><option value="user">{roleLabel('user')}</option></Select> : <span>{roleLabel(row.role)}</span> },
          { key: 'status', title: tr('状态', 'Status'), width: 110, render: (row) => <span className={`user-status is-${row.user?.status}`}>{row.user?.status === 'active' ? tr('正常', 'Active') : tr('已停用', 'Disabled')}</span> },
          { key: 'action', title: tr('操作', 'Actions'), width: 90, fixed: 'right', render: (row) => selected.can_manage ? <button className="liaison-table-link is-danger" onClick={async () => { try { await removeOrganizationMember(selected.id, row.user_id); await loadMembers(selected.id); } catch (error: any) { notify('danger', error?.message); } }}><UserMinus size={12} />{tr('移除', 'Remove')}</button> : '-' },
        ]} />
      </> : <div className="organization-placeholder">{tr('选择一个组织查看成员', 'Select an organization')}</div>}</div>
    </div>
    <OrganizationFormModal open={dialog === 'create'} defaultParentId={selected?.can_manage ? selected.id : orgs.find((item) => item.can_manage)?.id} organizations={orgs} onClose={() => setDialog(undefined)} onSubmit={async (values) => { try { const response = await createOrganization(values); setDialog(undefined); await loadOrganizations(); setSelectedId(response.data?.id); notify('success', tr('组织已创建', 'Organization created')); } catch (error: any) { notify('danger', error?.message); } }} />
    <OrganizationFormModal open={dialog === 'edit'} organization={selected} organizations={orgs} onClose={() => setDialog(undefined)} onSubmit={async (values) => { if (!selected) return; try { await updateOrganization(selected.id, { ...values, set_parent: true }); setDialog(undefined); await loadOrganizations(); notify('success', tr('组织已更新', 'Organization updated')); } catch (error: any) { notify('danger', error?.message); } }} />
    <AddMemberModal open={dialog === 'member'} onClose={() => setDialog(undefined)} onSubmit={async (email, role) => { if (!selected) return; try { await addOrganizationMember(selected.id, { email, role }); setDialog(undefined); await loadMembers(selected.id); notify('success', tr('成员已添加', 'Member added')); } catch (error: any) { notify('danger', error?.message); } }} />
    <Modal open={dialog === 'delete'} width={430} title={tr('删除组织', 'Delete organization')} onClose={() => setDialog(undefined)} footer={<><Button onClick={() => setDialog(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" onClick={async () => { if (!selected) return; try { await deleteOrganization(selected.id); setDialog(undefined); await loadOrganizations(); notify('success', tr('组织已删除', 'Organization deleted')); } catch (error: any) { notify('danger', error?.message); } }}>{tr('删除', 'Delete')}</Button></>}><DangerConfirm title={tr(`确定删除“${selected?.name || ''}”？`, `Delete “${selected?.name || ''}”?`)} description={tr('请先移动其下级组织；成员关系会一并移除。', 'Move child organizations first. Memberships will be removed.')} /></Modal>
  </section>;
}

function OrganizationFormModal({ open, organization, defaultParentId, organizations, onClose, onSubmit }: { open: boolean; organization?: API.Organization; defaultParentId?: number; organizations: API.Organization[]; onClose: () => void; onSubmit: (values: { name: string; description: string; parent_id?: number }) => Promise<void> }) {
  const { tr } = useI18n(); const [name, setName] = useState(''); const [description, setDescription] = useState(''); const [parentId, setParentId] = useState('');
  useEffect(() => { if (open) { setName(organization?.name || ''); setDescription(organization?.description || ''); setParentId(organization?.parent_id ? String(organization.parent_id) : (defaultParentId ? String(defaultParentId) : '')); } }, [open, organization, defaultParentId]);
  const parentOptions = flattenOrganizations(organizations).filter(({ organization: item }) => item.id !== organization?.id);
  return <Modal open={open} width={520} title={organization ? tr('编辑组织', 'Edit organization') : tr('新建组织', 'New organization')} onClose={onClose} footer={<><Button onClick={onClose}>{tr('取消', 'Cancel')}</Button><Button variant="primary" disabled={!name.trim() || (!organization?.is_root && !parentId)} onClick={() => onSubmit({ name: name.trim(), description: description.trim(), parent_id: parentId ? Number(parentId) : undefined })}>{tr('确定', 'Save')}</Button></>}><div className="native-modal-form"><Field label={tr('组织名称', 'Name')} required><Input value={name} maxLength={128} onChange={(event) => setName(event.target.value)} /></Field><Field label={tr('上级组织', 'Parent organization')}><Select value={parentId} disabled={organization?.is_root} onChange={(event) => setParentId(event.target.value)}>{organization?.is_root ? <option value="">{tr('无上级组织', 'No parent organization')}</option> : <option value="" disabled>{tr('请选择上级组织', 'Select parent organization')}</option>}{parentOptions.map(({ organization: item, depth }) => <option key={item.id} value={item.id}>{'— '.repeat(depth)}{item.is_root ? tr('默认组织', 'Default organization') : item.name}</option>)}</Select></Field><Field label={tr('描述', 'Description')}><Input value={description} maxLength={512} onChange={(event) => setDescription(event.target.value)} /></Field></div></Modal>;
}

function AddMemberModal({ open, onClose, onSubmit }: { open: boolean; onClose: () => void; onSubmit: (email: string, role: API.OrganizationRole) => Promise<void> }) {
  const { tr } = useI18n(); const [email, setEmail] = useState(''); const [role, setRole] = useState<API.OrganizationRole>('user');
  useEffect(() => { if (open) { setEmail(''); setRole('user'); } }, [open]);
  return <Modal open={open} width={460} title={tr('添加组织成员', 'Add organization member')} onClose={onClose} footer={<><Button onClick={onClose}>{tr('取消', 'Cancel')}</Button><Button variant="primary" disabled={!email.trim()} onClick={() => onSubmit(email.trim(), role)}>{tr('添加', 'Add')}</Button></>}><div className="native-modal-form"><Field label={tr('用户邮箱', 'User email')} required hint={tr('用户需要先在系统中创建', 'The user must already exist')}><Input type="email" value={email} onChange={(event) => setEmail(event.target.value)} /></Field><Field label={tr('组织角色', 'Organization role')}><Select value={role} onChange={(event) => setRole(event.target.value as API.OrganizationRole)}><option value="admin">{tr('管理员', 'Admin')}</option><option value="user">{tr('用户', 'User')}</option></Select></Field></div></Modal>;
}

function UsersPanel({ currentUserId, notify }: { currentUserId?: number; notify: Notify }) {
  const { tr } = useI18n();
  const [users, setUsers] = useState<API.ManagedUser[]>([]);
  const [organizations, setOrganizations] = useState<API.Organization[]>([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [dialog, setDialog] = useState<'create' | 'password' | 'delete'>();
  const [target, setTarget] = useState<API.ManagedUser>();
  const [initialPassword, setInitialPassword] = useState('');
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [usersResponse, organizationsResponse] = await Promise.all([getManagedUsers(), getOrganizations()]);
      setUsers(usersResponse.data?.users || []);
      setOrganizations((organizationsResponse.data?.organizations || []).filter((item) => item.can_manage));
      setForbidden(false);
    }
    catch (error) { if (error instanceof RequestError && error.response?.status === 403) setForbidden(true); else notify('danger', (error as Error).message); }
    finally { setLoading(false); }
  }, [notify]);
  useEffect(() => { void load(); }, [load]);
  if (forbidden) return <section className="user-admin-empty"><ShieldCheck size={22} /><h2>{tr('需要管理员权限', 'Administrator required')}</h2><p>{tr('管理员可以维护组织成员和登录账号。', 'Administrators can manage organization members and sign-in accounts.')}</p></section>;
  return <section>
    <header className="user-section-heading"><div><h2>{tr('用户', 'Users')}</h2><p>{tr('管理登录账号和管理员角色', 'Manage sign-in accounts and administrator roles')}</p></div><Button variant="primary" onClick={() => setDialog('create')}><Plus size={13} />{tr('新建用户', 'New user')}</Button></header>
    <DataTable rows={users} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无用户', 'No users')} columns={[
      { key: 'user', title: tr('用户', 'User'), width: '28%', render: (row) => <MemberIdentity user={row} /> },
      { key: 'role', title: tr('系统角色', 'System role'), width: 140, render: (row) => <button disabled={row.id === currentUserId} className={`system-role-chip${row.role === 'admin' ? ' is-admin' : ''}`} onClick={async () => { try { await updateManagedUser(row.id, { role: row.role === 'admin' ? 'user' : 'admin' }); await load(); } catch (error: any) { notify('danger', error?.message); } }}>{row.role === 'admin' ? tr('管理员', 'Admin') : tr('用户', 'User')}</button> },
      { key: 'status', title: tr('状态', 'Status'), width: 110, render: (row) => <button disabled={row.id === currentUserId} className={`user-status is-${row.status}`} onClick={async () => { try { await updateManagedUser(row.id, { status: row.status === 'active' ? 'inactive' : 'active' }); await load(); } catch (error: any) { notify('danger', error?.message); } }}>{row.status === 'active' ? tr('正常', 'Active') : tr('已停用', 'Disabled')}</button> },
      { key: 'login', title: tr('最后登录', 'Last login'), width: 170, render: (row) => <Timestamp value={row.last_login} /> },
      { key: 'created', title: tr('创建时间', 'Created'), width: 170, render: (row) => <Timestamp value={row.created_at} /> },
      { key: 'actions', title: tr('操作', 'Actions'), width: 160, fixed: 'right', render: (row) => <div className="liaison-table-actions"><button className="liaison-table-link" onClick={() => { setTarget(row); setDialog('password'); }}><KeyRound size={12} />{tr('密码', 'Password')}</button>{row.id !== currentUserId ? <button className="liaison-table-link is-danger" onClick={() => { setTarget(row); setDialog('delete'); }}>{tr('删除', 'Delete')}</button> : null}</div> },
    ]} />
    <CreateUserModal open={dialog === 'create'} organizations={organizations} onClose={() => setDialog(undefined)} onSubmit={async (values) => { try { const response = await createManagedUser(values); setDialog(undefined); setInitialPassword(response.data?.initial_password || values.password || ''); await load(); notify('success', tr('用户已创建', 'User created')); } catch (error: any) { notify('danger', error?.message); } }} />
    <PasswordModal open={dialog === 'password'} title={tr(`重置 ${target?.name || ''} 的密码`, `Reset password for ${target?.name || ''}`)} onClose={() => setDialog(undefined)} onSubmit={async (password) => { if (!target) return; try { await resetManagedUserPassword(target.id, password); setDialog(undefined); notify('success', tr('密码已重置', 'Password reset')); } catch (error: any) { notify('danger', error?.message); } }} />
    <Modal open={Boolean(initialPassword)} width={440} title={tr('初始密码', 'Initial password')} onClose={() => setInitialPassword('')} footer={<Button variant="primary" onClick={() => setInitialPassword('')}>{tr('完成', 'Done')}</Button>}><Notice tone="warning">{tr('密码只展示这一次，请通过安全渠道交给用户。', 'This password is shown once. Share it securely.')}</Notice><code className="initial-password-value">{initialPassword}</code></Modal>
    <Modal open={dialog === 'delete'} width={430} title={tr('删除用户', 'Delete user')} onClose={() => setDialog(undefined)} footer={<><Button onClick={() => setDialog(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" onClick={async () => { if (!target) return; try { await deleteManagedUser(target.id); setDialog(undefined); await load(); notify('success', tr('用户已删除', 'User deleted')); } catch (error: any) { notify('danger', error?.message); } }}>{tr('删除', 'Delete')}</Button></>}><DangerConfirm title={tr(`删除“${target?.name || ''}”？`, `Delete “${target?.name || ''}”?`)} description={tr('该用户的所有组织成员关系也会被移除。', 'All organization memberships for this user will also be removed.')} /></Modal>
  </section>;
}

function CreateUserModal({ open, organizations, onClose, onSubmit }: { open: boolean; organizations: API.Organization[]; onClose: () => void; onSubmit: (values: { organization_id: number; name: string; email: string; password?: string; role: API.IAMRole }) => Promise<void> }) {
  const { tr } = useI18n(); const [name, setName] = useState(''); const [email, setEmail] = useState(''); const [password, setPassword] = useState(''); const [organizationId, setOrganizationId] = useState(''); const [role, setRole] = useState<API.IAMRole>('user');
  useEffect(() => { if (open) { setName(''); setEmail(''); setPassword(''); setOrganizationId(organizations[0]?.id ? String(organizations[0].id) : ''); setRole('user'); } }, [open, organizations]);
  return <Modal open={open} width={500} title={tr('新建用户', 'New user')} onClose={onClose} footer={<><Button onClick={onClose}>{tr('取消', 'Cancel')}</Button><Button variant="primary" disabled={!email.trim() || !organizationId} onClick={() => onSubmit({ organization_id: Number(organizationId), name, email, password: password || undefined, role })}>{tr('创建', 'Create')}</Button></>}><div className="native-modal-form"><Field label={tr('名称', 'Name')}><Input value={name} onChange={(event) => setName(event.target.value)} placeholder={tr('默认使用邮箱前缀', 'Defaults to email prefix')} /></Field><Field label={tr('邮箱', 'Email')} required><Input type="email" value={email} onChange={(event) => setEmail(event.target.value)} /></Field><Field label={tr('所属组织', 'Organization')} required><Select value={organizationId} onChange={(event) => setOrganizationId(event.target.value)}><option value="" disabled>{tr('请选择组织', 'Select organization')}</option>{flattenOrganizations(organizations).map(({ organization, depth }) => <option key={organization.id} value={organization.id}>{'— '.repeat(depth)}{organization.is_root ? tr('默认组织', 'Default organization') : organization.name}</option>)}</Select></Field><Field label={tr('初始密码', 'Initial password')} hint={tr('留空自动生成高强度密码', 'Leave blank to generate a strong password')}><Input type="password" value={password} onChange={(event) => setPassword(event.target.value)} /></Field><Field label={tr('组织角色', 'Organization role')}><Select value={role} onChange={(event) => setRole(event.target.value as API.IAMRole)}><option value="user">{tr('用户', 'User')}</option><option value="admin">{tr('管理员', 'Admin')}</option></Select></Field></div></Modal>;
}

function PasswordModal({ open, title, onClose, onSubmit }: { open: boolean; title: string; onClose: () => void; onSubmit: (password: string) => Promise<void> }) {
  const { tr } = useI18n(); const [password, setPassword] = useState('');
  useEffect(() => { if (open) setPassword(''); }, [open]);
  return <Modal open={open} width={440} title={title} onClose={onClose} footer={<><Button onClick={onClose}>{tr('取消', 'Cancel')}</Button><Button variant="primary" disabled={password.length < 8} onClick={() => onSubmit(password)}>{tr('确定', 'Save')}</Button></>}><Field label={tr('新密码', 'New password')} hint={tr('至少 8 位', 'At least 8 characters')} required><Input type="password" value={password} onChange={(event) => setPassword(event.target.value)} /></Field></Modal>;
}

function MemberIdentity({ user, fallback = '-' }: { user?: API.ManagedUser; fallback?: string }) {
  const name = user?.name || user?.email || fallback;
  return <div className="member-identity"><span>{name.slice(0, 1).toUpperCase()}</span><div><strong>{user?.name || fallback}</strong>{user?.email ? <small>{user.email}</small> : null}</div></div>;
}

function AccountPanel() {
  const { initialState } = useModel('@@initialState'); const { tr } = useI18n(); const currentUser = initialState?.currentUser;
  const identity = getAvatarIdentity(currentUser); const localAvatar = useAvatarStore((state) => state.avatars[identity]); const setAvatar = useAvatarStore((state) => state.setAvatar); const removeAvatar = useAvatarStore((state) => state.removeAvatar); const avatarInputRef = useRef<HTMLInputElement>(null); const avatar = localAvatar || currentUser?.avatar; const accountLabel = getAccountLabel(currentUser) || tr('用户', 'User');
  const handleAvatarChange = (event: ChangeEvent<HTMLInputElement>) => { const file = event.target.files?.[0]; event.target.value = ''; if (!file || !['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size > 3 * 1024 * 1024) return; const reader = new FileReader(); reader.onload = () => { const image = new Image(); image.onload = () => { const size = Math.min(image.naturalWidth, image.naturalHeight); const canvas = document.createElement('canvas'); canvas.width = 256; canvas.height = 256; const context = canvas.getContext('2d'); if (!context) return; context.drawImage(image, (image.naturalWidth - size) / 2, (image.naturalHeight - size) / 2, size, size, 0, 0, 256, 256); setAvatar(identity, canvas.toDataURL('image/png')); }; image.src = String(reader.result); }; reader.readAsDataURL(file); };
  const apiUser = currentUser as API.CurrentUser & { last_login?: string; login_ip?: string };
  const details = [[tr('系统角色', 'System role'), currentUser?.role === 'admin' ? tr('管理员', 'Admin') : tr('用户', 'User')], [tr('注册时间', 'Created'), currentUser?.created_at || '-'], [tr('最后登录', 'Last login'), currentUser?.last_login_at || apiUser?.last_login || '-'], [tr('登录 IP', 'Login IP'), currentUser?.last_login_ip || apiUser?.login_ip || '-']];
  return <section className="settings-section native-card user-account-panel"><div className="user-profile"><div className="user-avatar-editor"><div className="native-avatar">{avatar ? <img src={avatar} alt="" /> : accountLabel.slice(0, 1).toUpperCase()}</div><button type="button" className="user-avatar-edit" onClick={() => avatarInputRef.current?.click()}><Camera size={13} /></button><input ref={avatarInputRef} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={handleAvatarChange} /></div><div className="user-info"><div className="user-name-row"><h3>{accountLabel}</h3></div><p>{currentUser?.email || '-'}</p><small>{tr('用于登录 Liaison 与识别操作记录', 'Used to sign in and identify account activity')}</small></div><div className="user-avatar-actions"><Button variant="ghost" onClick={() => avatarInputRef.current?.click()}><Camera size={14} />{tr('更换头像', 'Change avatar')}</Button>{avatar ? <Button variant="ghost" onClick={() => removeAvatar(identity)}><Trash2 size={14} />{tr('移除', 'Remove')}</Button> : null}</div></div><dl className="native-description-grid">{details.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl></section>;
}

function PasswordPanel({ notify }: { notify: Notify }) {
  const { tr } = useI18n(); const [values, setValues] = useState({ oldPassword: '', newPassword: '', confirmPassword: '' }); const [loading, setLoading] = useState(false);
  const submit = async (event: FormEvent) => { event.preventDefault(); if (values.newPassword.length < 8 || values.newPassword !== values.confirmPassword) { notify('danger', tr('新密码至少 8 位，且两次输入必须一致', 'Use 8+ matching characters')); return; } setLoading(true); try { await changePassword({ old_password: values.oldPassword, new_password: values.newPassword }); setValues({ oldPassword: '', newPassword: '', confirmPassword: '' }); notify('success', tr('密码修改成功', 'Password changed')); } catch (error: any) { notify('danger', error?.message); } finally { setLoading(false); } };
  return <section className="settings-section native-card"><div className="password-tips"><ShieldCheck size={20} /><div><strong>{tr('密码安全', 'Password security')}</strong><span>{tr('修改后请使用新密码登录。', 'Use the new password for future sign-ins.')}</span></div></div><form className="native-password-form" onSubmit={submit}><Field label={tr('当前密码', 'Current password')} required><Input type="password" value={values.oldPassword} onChange={(event) => setValues({ ...values, oldPassword: event.target.value })} /></Field><Field label={tr('新密码', 'New password')} required><Input type="password" value={values.newPassword} onChange={(event) => setValues({ ...values, newPassword: event.target.value })} /></Field><Field label={tr('确认新密码', 'Confirm password')} required><Input type="password" value={values.confirmPassword} onChange={(event) => setValues({ ...values, confirmPassword: event.target.value })} /></Field><Button type="submit" variant="primary" loading={loading}>{tr('修改密码', 'Change password')}</Button></form></section>;
}

function flattenOrganizations(organizations: API.Organization[]) {
  const children = new Map<number | undefined, API.Organization[]>();
  organizations.forEach((organization) => { const parentId = organization.parent_id ?? undefined; const list = children.get(parentId) || []; list.push(organization); children.set(parentId, list); });
  children.forEach((list) => list.sort((a, b) => a.id - b.id));
  const result: { organization: API.Organization; depth: number }[] = [];
  const visit = (organization: API.Organization, depth: number) => { if (result.some((item) => item.organization.id === organization.id)) return; result.push({ organization, depth }); (children.get(organization.id) || []).forEach((child) => visit(child, depth + 1)); };
  [...(children.get(undefined) || []), ...(children.get(0) || [])].forEach((root) => visit(root, 0));
  organizations.filter((item) => !result.some(({ organization }) => organization.id === item.id)).forEach((item) => visit(item, 0));
  return result;
}

export default UserPage;
