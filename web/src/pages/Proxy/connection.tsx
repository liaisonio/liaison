import {isLLMAccessType} from '@/constants/accessTypes';
import { request } from '@/api/client';
import { Field, Input, Select } from '@/components/ui';
import { useI18n } from '@/i18n';
import { tlsOptionsForProtocol } from '@/pages/WebData/connection';
import {
  getWebDataTarget,
  getWebDesktopTarget,
  getWebSSHTarget,
  saveWebDataCredential,
} from '@/services/api';

export type InitialConnection = {
  credentialId?: number;
  saved?: boolean;
  remember: boolean;
  extra?: Pick<API.WebDataCredential, 'auth_mechanism' | 'direct_connection' | 'connection_params'>;
  enabled: boolean;
  username: string;
  password: string;
  database: string;
  auth_database: string;
  domain: string;
  tls_mode: string;
  redis_db: string;
  schema: string;
};
export const emptyConnection = (): InitialConnection => ({
  enabled: true,
  remember: true,
  username: '',
  password: '',
  database: '',
  auth_database: 'admin',
  domain: '',
  tls_mode: 'disable',
  redis_db: '0',
  schema: '',
});
export const databaseAccessTypes = [
  'webmysql',
  'webmariadb',
  'webdoris',
  'webstarrocks',
  'webtidb',
  'webpostgresql',
  'websqlserver',
  'weboracle',
  'webdameng',
  'webclickhouse',
  'webelasticsearch',
  'webopensearch',
  'webredis',
  'webmemcached',
  'webmongodb',
];
export const supportsInitialConnection = (type: string) =>
  ['webssh', 'websftp', 'webrdp', 'webvnc', 'webs3', 'websmb', ...databaseAccessTypes].includes(
    type,
  );

export function InitialConnectionFields({
  type,
  value,
  onChange,
  disabled = false,
}: {
  type: string;
  value: InitialConnection;
  onChange: (v: InitialConnection) => void;
  disabled?: boolean;
}) {
  const { tr } = useI18n();
  if (!supportsInitialConnection(type)) return null;
  if(type==='websmb')return <fieldset className="is-full liaison-initial-connection" disabled={disabled}>
    <legend>{tr('SMB 共享连接','SMB share connection')}</legend>
    <div className="liaison-initial-connection-fields">
      <Field label={tr('用户名','Username')} required><Input required value={value.username} onChange={e=>onChange({...value,username:e.target.value})}/></Field>
      <Field label={tr('域（可选）','Domain (optional)')}><Input value={value.schema} onChange={e=>onChange({...value,schema:e.target.value})}/></Field>
      <Field label={tr('共享名','Share name')} required hint={tr('仅填写共享名，不是 UNC 地址或目录路径。','Enter one share name, not a UNC address or directory path.')}><Input required value={value.database} onChange={e=>onChange({...value,database:e.target.value})}/></Field>
      <p className="is-full">{tr('SMB2/3，要求消息签名。当前支持浏览、文本预览和下载，不提供写入操作。','SMB2/3 with required message signing. Browsing, text preview and download only; no write operations.')}</p>
      <label className="liaison-checkbox is-full"><input type="checkbox" checked={value.remember} onChange={e=>onChange({...value,remember:e.target.checked,password:''})}/>{tr('保存密码','Save password')}</label>
      {value.remember&&<Field label={tr('密码','Password')} hint={value.saved?tr('留空保留已保存密码','Leave blank to keep the saved password'):undefined}><Input type="password" autoComplete="new-password" required={!value.saved} placeholder={value.saved?'••••••••':undefined} value={value.password} onChange={e=>onChange({...value,password:e.target.value})}/></Field>}
      {!value.remember&&<p>{tr('访问时输入密码，不保存。','Enter password when connecting; it will not be saved.')}</p>}
    </div>
  </fieldset>;
  if(type==='webs3')return <fieldset className="is-full liaison-initial-connection" disabled={disabled}>
    <legend>{tr('对象存储连接','Object storage connection')}</legend>
    <div className="liaison-initial-connection-fields">
      <Field label="Access Key" required><Input required value={value.username} onChange={e=>onChange({...value,username:e.target.value})}/></Field>
      <Field label={tr('区域','Region')}><Input placeholder="us-east-1" value={value.schema} onChange={e=>onChange({...value,schema:e.target.value})}/></Field>
      <Field label={tr('指定桶（可选）','Bucket (optional)')} hint={tr('限制在此桶内浏览；留空列出可见的桶。','Browse only this bucket; leave empty to list visible buckets.')}><Input value={value.database} onChange={e=>onChange({...value,database:e.target.value})}/></Field>
      <Field label="TLS"><Select value={value.tls_mode} onChange={e=>onChange({...value,tls_mode:e.target.value})}><option value="require">{tr('启用并验证证书','Enable and verify certificate')}</option><option value="disable">{tr('关闭（连接器到服务为 HTTP）','Off (HTTP from connector to service)')}</option></Select></Field>
      <label className="liaison-checkbox is-full"><input type="checkbox" checked={value.remember} onChange={e=>onChange({...value,remember:e.target.checked,password:''})}/>{tr('保存 Secret Key','Save Secret Key')}</label>
      {value.remember&&<Field label="Secret Key" hint={value.saved?tr('留空保留已保存密钥','Leave blank to keep the saved key'):undefined}><Input type="password" autoComplete="new-password" required={!value.saved} placeholder={value.saved?'••••••••':undefined} value={value.password} onChange={e=>onChange({...value,password:e.target.value})}/></Field>}
      {!value.remember&&<p>{tr('访问时输入 Secret Key，不保存。','Enter Secret Key when connecting. It will not be saved.')}</p>}
    </div>
  </fieldset>;
  if (type === 'webmemcached') return <fieldset className="is-full liaison-initial-connection" disabled={disabled}>
    <legend>{tr('连接选项','Connection options')}</legend>
    <p>{tr('使用 Memcached basic-text 协议，不支持用户名密码或 SASL 认证。','Uses Memcached basic-text protocol. Username/password and SASL authentication are not supported.')}</p>
    <Field label="TLS"><Select value={value.tls_mode} onChange={e=>onChange({...value,tls_mode:e.target.value})}>
      <option value="disable">{tr('关闭','Off')}</option><option value="require">{tr('启用并验证证书','Enable and verify certificate')}</option>
    </Select></Field>
  </fieldset>;
  const data = databaseAccessTypes.includes(type),
    protocol = type.replace(/^web/, '');
  const optionalUser = [
    'webvnc',
    'webredis',
    'webelasticsearch',
    'webopensearch',
    'webclickhouse',
  ].includes(type);
  const set = (key: keyof InitialConnection, text: string) =>
    onChange({ ...value, [key]: text });
  return (
    <fieldset
      className="is-full liaison-initial-connection"
      disabled={disabled}
    >
      <legend>{tr('我的连接', 'My connection')}</legend>
      <p>
        {tr(
          '一个访问对应一个账号。不同账号请新建访问，凭据仅自己可用。',
          'One account per access. Create another access for a different account. Credentials are private to you.',
        )}
      </p>
      {value.enabled && (
        <div className="liaison-initial-connection-fields">
          {type !== 'webvnc' && (
            <Field label={tr('用户名', 'Username')} required={!optionalUser}>
              <Input
                autoComplete="username"
                value={value.username}
                readOnly={!!value.credentialId}
                required={!optionalUser}
                maxLength={255}
                onChange={(e) => set('username', e.target.value)}
              />
            </Field>
          )}
          {type === 'webdameng' && <Field label="Schema"><Input value={value.schema} placeholder={tr('默认使用账号的 Schema','Use account default if empty')} onChange={e=>set('schema',e.target.value)}/></Field>}
          <label className="liaison-checkbox is-full"><input type="checkbox" checked={value.remember} onChange={e=>onChange({...value,remember:e.target.checked,password:''})}/>{tr('保存密码','Save password')}</label>
          {!value.remember&&<p className="is-full">{tr('每次建立新会话时输入密码，密码不会保存。','Enter the password when starting a new session. It will not be saved.')}</p>}
          {value.remember&&<Field label={tr('密码', 'Password')} required={!data&&!value.saved} hint={value.saved?tr('留空保留已保存的密码','Leave blank to keep the saved password'):undefined}>
            <Input
              autoComplete="new-password"
              type="password"
              value={value.password}
              required={!data&&!value.saved}
              placeholder={value.saved?'••••••••':undefined}
              onChange={(e) => set('password', e.target.value)}
            />
          </Field>}
          {type === 'webrdp' && (
            <Field label={tr('域', 'Domain')}>
              <Input
                value={value.domain}
                readOnly={!!value.credentialId}
                maxLength={255}
                onChange={(e) => set('domain', e.target.value)}
              />
            </Field>
          )}
          {data && type !== 'webredis' && type !== 'webdameng' && (
            <Field
              label={
                type === 'weboracle'
                  ? tr('服务名 / SID', 'Service name / SID')
                  : tr('默认数据库', 'Default database')
              }
            >
              <Input
                value={value.database}
                onChange={(e) => set('database', e.target.value)}
              />
            </Field>
          )}
          {type === 'webmongodb' && (
            <Field label={tr('认证数据库', 'Authentication database')}>
              <Input
                value={value.auth_database}
                onChange={(e) => set('auth_database', e.target.value)}
              />
            </Field>
          )}
          {type === 'webredis' && (
            <Field label={tr('数据库编号', 'Database index')}>
              <Input
                type="number"
                min={0}
                max={15}
                value={value.redis_db}
                onChange={(e) => set('redis_db', e.target.value)}
              />
            </Field>
          )}
          {type === 'webpostgresql' && (
            <Field label="Schema">
              <Input
                value={value.schema}
                onChange={(e) => set('schema', e.target.value)}
              />
            </Field>
          )}
          {data && (
            <Field label="TLS">
              <Select
                value={value.tls_mode}
                onChange={(e) => set('tls_mode', e.target.value)}
              >
                {tlsOptionsForProtocol(protocol, tr).map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          {type==='webdameng'&&<p className="is-full">{tr('暂不支持数据库 TLS；连接器链路加密不受影响。','Database TLS is not yet supported. Connector encryption is unchanged.')}</p>}
        </div>
      )}
    </fieldset>
  );
}

export async function saveInitialConnection(
  id: number,
  type: string,
  name: string,
  c: InitialConnection,
) {
  if (!supportsInitialConnection(type)) return;
  if (type === 'webssh' || type === 'websftp') {
    const res = await request<API.Response>(
      `/api/v1/webssh/proxies/${id}/credential`,
      {
        method: c.credentialId?'PUT':'POST',
        data: {
          name,
          username: c.username.trim(),
          password: c.remember?c.password:'',
          remember_password: c.remember,
        },
      },
    );
    if (res.code !== 200) throw Error('save');
  } else if (type === 'webrdp' || type === 'webvnc') {
    if(c.remember&&c.credentialId&&c.saved&&!c.password)return;
    const res = await request<API.Response>(
      `/api/v1/webdesktop/proxies/${id}/credential`,
      {
        method: 'POST',
        data: {
          username: c.username.trim(),
          password: c.remember?c.password:'',
          remember_password: c.remember,
          domain: c.domain.trim(),
        },
      },
    );
    if (res.code !== 200) throw Error('save');
  } else {
    const res = await saveWebDataCredential(id, {
      ...c.extra,
      id:c.credentialId,
      name,
      protocol: type.replace(/^web/, ''),
      username: c.username.trim(),
      password: c.remember?c.password:'',
      remember_password: c.remember,
      database: c.database.trim(),
      auth_database: type === 'webmongodb' ? c.auth_database.trim() : '',
      tls_mode: type === 'websmb' ? 'disable' : c.tls_mode,
      redis_db: Number(c.redis_db) || 0,
      schema: c.schema.trim(),
      direct_connection: c.extra?.direct_connection??true,
    });
    if (res.code !== 200) throw Error('save');
  }
}

export async function loadAccessConnection(id:number,type:string):Promise<InitialConnection> {
  const blank=emptyConnection();
  if(!supportsInitialConnection(type))return blank;
  if(type==='webssh'||type==='websftp'){
    const r=await getWebSSHTarget(id);if(r.code!==200)throw Error('connection');const c=r.data?.credentials?.[0];
    return c?{...blank,credentialId:c.id,username:c.username||'',saved:c.saved,remember:!!c.saved}:blank;
  }
  if(type==='webrdp'||type==='webvnc'){
    const r=await getWebDesktopTarget(id);if(r.code!==200)throw Error('connection');const c=r.data?.credentials?.[0];
    return c?{...blank,credentialId:c.id,username:c.username||'',domain:c.domain||'',saved:c.saved,remember:!!c.saved}:blank;
  }
  const r=await getWebDataTarget(id);if(r.code!==200)throw Error('connection');const c=r.data?.credentials?.[0];
  return c?{...blank,credentialId:c.id,saved:c.saved,remember:!!c.saved,username:c.username||'',database:c.database||'',auth_database:c.auth_database||'',redis_db:String(c.redis_db??0),schema:c.schema||'',tls_mode:c.tls_mode||'disable',extra:{auth_mechanism:c.auth_mechanism,direct_connection:c.direct_connection,connection_params:c.connection_params}}:blank;
}

export class AccessConfigurationRequired extends Error {}

// Selection is made from authenticated, user-scoped APIs, never from the shared
// access record. No password, username or connection secret goes in the URL.
export async function directAccessPath(
  id: number,
  type: string,
): Promise<string> {
  if (isLLMAccessType(type)) return `/ai/${id}?tab=playground`;
  if (type === 'websftp') {
    const c=await loadAccessConnection(id,type);if(!c.credentialId)throw new AccessConfigurationRequired();
    return `/websftp/${id}?connect=1`;
  }
  if (type === 'webssh') {
    const r = await getWebSSHTarget(id);
    if (r.code !== 200) throw Error('target');
    const c = r.data?.credentials?.[0];
    if(!c)throw new AccessConfigurationRequired();
    return `/webssh/${id}/connections/${c.id}`;
  }
  if (type === 'webrdp' || type === 'webvnc') {
    const r = await getWebDesktopTarget(id);
    if (r.code !== 200) throw Error('target');
    const c = r.data?.credentials?.[0];
    if(!c)throw new AccessConfigurationRequired();
    return `/webdesktop/${id}/connections/${c.id}`;
  }
  const r = await getWebDataTarget(id);
  if (r.code !== 200) throw Error('target');
  const c = r.data?.credentials?.[0];
  if(!c)throw new AccessConfigurationRequired();
  return `/${type==='webs3'?'webs3':type==='websmb'?'websmb':'webdata'}/${id}/connections/${c.id}`;
}
