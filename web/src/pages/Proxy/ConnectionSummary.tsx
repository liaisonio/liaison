import {isLLMAccessType} from '@/constants/accessTypes';
import { useI18n } from '@/i18n';
import {
  getWebDataTarget,
  getWebDesktopTarget,
  getWebSSHTarget,
} from '@/services/api';
import { useSession } from '@/store/session';
import { useEffect, useState } from 'react';
import { databaseAccessTypes, supportsInitialConnection } from './connection';

// Share only in-flight reads between the three cells, never cache credentials.
const pending = new Map<string, Promise<unknown>>();
function sharedRead<T>(key:string,load:()=>Promise<T>):Promise<T>{
  const existing=pending.get(key);if(existing)return existing as Promise<T>;
  const promise=load().finally(()=>{if(pending.get(key)===promise)pending.delete(key);});
  pending.set(key,promise);return promise;
}

// Deliberate allowlist: never render arbitrary connection parameters, DSNs,
// provider configuration, encrypted fields or passwords from an API response.
type Summary = {
  username?: string;
  database?: string;
  domain?: string;
  schema?: string;
  authDatabase?: string;
  tls?: string;
  redisDB?: number;
  count: number;
};

export default function ConnectionSummary({
  id,
  type,
  revision,
  field,
}: {
  id: number;
  type: string;
  revision?: string;
  field: 'username'|'database'|'advanced';
}) {
  const { tr } = useI18n();
  const owner = useSession((s) => s.token);
  const [state, setState] = useState<{
    owner: typeof owner;
    value?: Summary;
    failed?: boolean;
  }>();
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    if (!supportsInitialConnection(type)) return;
    let alive = true;
    setState(undefined);
    (async () => {
      const key=JSON.stringify([owner,id,type,revision,retry]);
      let value: Summary;
      if (type === 'webssh' || type === 'websftp') {
        const r = await sharedRead(key,()=>getWebSSHTarget(id));
        if (r.code !== 200) throw Error('summary');
        const rows = r.data?.credentials || [];
        value = { username: rows[0]?.username, count: rows.length };
      } else if (type === 'webrdp' || type === 'webvnc') {
        const r = await sharedRead(key,()=>getWebDesktopTarget(id));
        if (r.code !== 200) throw Error('summary');
        const rows = r.data?.credentials || [];
        value = {
          username: rows[0]?.username,
          domain: rows[0]?.domain,
          count: rows.length,
        };
      } else {
        const r = await sharedRead(key,()=>getWebDataTarget(id));
        if (r.code !== 200) throw Error('summary');
        const rows = r.data?.credentials || [],
          c = rows[0];
        value = {
          username: c?.username,
          database: c?.database,
          schema: c?.schema,
          authDatabase: c?.auth_database,
          tls: c?.tls_mode,
          redisDB: c?.redis_db,
          count: rows.length,
        };
      }
      if (alive) setState({ owner, value });
    })().catch(() => {
      if (alive) setState({ owner, failed: true });
    });
    return () => {
      alive = false;
    };
  }, [id, type, revision, owner, retry]);
  if (!supportsInitialConnection(type))
    return (
      <span className="liaison-connection-muted">
        {isLLMAccessType(type)
          ? tr('API 密钥认证', 'API key authentication')
          : tr('客户端认证', 'Client authentication')}
      </span>
    );
  const result = state?.owner === owner ? state : undefined;
  if (result?.failed)
    return (
      <button
        className="liaison-table-link"
        onClick={() => setRetry((n) => n + 1)}
      >
        {tr('加载失败 · 重试', 'Failed · Retry')}
      </button>
    );
  if (!result?.value)
    return (
      <span className="liaison-connection-muted" aria-busy="true">
        {tr('加载中…', 'Loading…')}
      </span>
    );
  const c = result.value;
  if (!c.count)
    return (
      <span className="liaison-connection-muted">
        {tr('未配置连接', 'Not configured')}
      </span>
    );
  const details: string[] = [];
  if(field==='username')return <span title={c.username}>{c.username||'—'}</span>;
  if(field==='database')return <span title={c.database}>{type==='webredis'?String(c.redisDB??0):c.database||'—'}</span>;
  if (c.domain) details.push(`${tr('域', 'Domain')}: ${c.domain}`);
  if (c.schema) details.push(`${type==='websmb'?tr('域','Domain'):type==='webs3'?'Region':'Schema'}: ${c.schema}`);
  if (type === 'webmongodb' && c.authDatabase)
    details.push(`${tr('认证库', 'Auth DB')}: ${c.authDatabase}`);
  if ((databaseAccessTypes.includes(type)||type==='webs3') && c.tls && c.tls !== 'disable')
    details.push(
      `TLS: ${c.tls}`,
    );
  return <span className="liaison-connection-muted" title={details.join(' · ')}>{details.join(' · ')||'—'}</span>;
}
