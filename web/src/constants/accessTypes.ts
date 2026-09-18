import {LLM_PROTOCOLS,LLM_PROTOCOL_OPTIONS,protocolFamily,isLLMApplicationType} from './llmProtocols';
export const isLLMAccessType=(value?:string|null)=>value==='aiapi'||(value!=='llm'&&isLLMApplicationType(value||undefined));
import type { ApplicationType } from './applicationTypes';
import {optionalProtocolEnabled} from '../store/optionalProtocols';

export type WebAccessType =
  | 'aiapi'
  | (typeof LLM_PROTOCOLS)[number]['value']
  | 'webssh'
  | 'websftp'
  | 'webrdp'
  | 'webvnc'
  | 'webmysql'
  | 'webmariadb'
  | 'webdoris'
  | 'webstarrocks'
  | 'webtidb'
  | 'websqlserver'
  | 'weboracle'
  | 'webdameng'
  | 'webclickhouse'
  | 'webelasticsearch'
  | 'webopensearch'
  | 'webpostgresql'
  | 'webredis'
  | 'webmemcached'
  | 'webs3'
  | 'websmb'
  | 'webmongodb';

export type AccessType = ApplicationType | WebAccessType;

type AccessTypeOption = { value: AccessType; label: string };

const NATIVE_ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  { value: 'http', label: 'HTTP' },
  { value: 'ssh', label: 'SSH' },
];

// Kept as known values so legacy records can still be identified and hidden
// without being misclassified as TCP. Liaison does not currently terminate
// these native desktop/database protocols, so they are not exposed as product access
// types until a protocol-aware server implementation exists.
const UNSUPPORTED_NATIVE_ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  { value: 'rdp', label: 'RDP' },
  { value: 'vnc', label: 'VNC' },
  { value: 'mysql', label: 'MySQL' },
  { value: 'mariadb', label: 'MariaDB' },
  { value: 'doris', label: 'Doris' },
  { value: 'starrocks', label: 'StarRocks' },
  { value: 'tidb', label: 'TiDB' },
  { value: 'sqlserver', label: 'SQL Server' },
  { value: 'oracle', label: 'Oracle' },
  { value: 'clickhouse', label: 'ClickHouse' },
  { value: 'elasticsearch', label: 'Elasticsearch' },
  { value: 'opensearch', label: 'OpenSearch' },
  { value: 'postgresql', label: 'PostgreSQL' },
  { value: 'redis', label: 'Redis' },
  { value: 'mongodb', label: 'MongoDB' },
];

const WEB_ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  { value: 'webssh', label: 'Web SSH' },
  { value: 'websftp', label: 'Web SFTP' },
  { value: 'webrdp', label: 'Web RDP' },
  { value: 'webvnc', label: 'Web VNC' },
  { value: 'webmysql', label: 'Web MySQL' },
  { value: 'webmariadb', label: 'Web MariaDB' },
  { value: 'webdoris', label: 'WebDoris' },
  { value: 'webstarrocks', label: 'WebStarRocks' },
  { value: 'webtidb', label: 'WebTiDB' },
  { value: 'websqlserver', label: 'Web SQL Server' },
  { value: 'weboracle', label: 'Web Oracle' },
  { value: 'webdameng', label: 'WebDameng' },
  { value: 'webclickhouse', label: 'Web ClickHouse' },
  { value: 'webelasticsearch', label: 'Web Elasticsearch' },
  { value: 'webopensearch', label: 'Web OpenSearch' },
  { value: 'webpostgresql', label: 'Web PostgreSQL' },
  { value: 'webredis', label: 'Web Redis' },
  { value: 'webmemcached', label: 'Web Memcached' },
  { value: 'webs3', label: 'Web S3' },
  { value: 'websmb', label: 'WebSMB' },
  { value: 'webmongodb', label: 'Web MongoDB' },
  ...LLM_PROTOCOL_OPTIONS,
];

// Keep one product-wide order: L4 passthrough, native protocol, browser access.
export const ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  { value: 'tcp', label: 'TCP' },
  ...NATIVE_ACCESS_TYPES,
  ...WEB_ACCESS_TYPES,
];

const KNOWN_ACCESS_TYPES: ReadonlyArray<AccessTypeOption> = [
  ...ACCESS_TYPES,
  {value:'aiapi',label:'—'},
  {value:'openai-compatible',label:'OpenAI'},
  ...UNSUPPORTED_NATIVE_ACCESS_TYPES,
];

// Creation prefers browser workspaces without changing navigation/filter order.
export const ACCESS_CREATION_TYPES: ReadonlyArray<AccessTypeOption> = [
  ...WEB_ACCESS_TYPES,
  ...ACCESS_TYPES.filter(item => !WEB_ACCESS_TYPES.some(web => web.value === item.value)),
];

const WEB_TYPE_BY_APPLICATION: Partial<Record<ApplicationType, WebAccessType>> = {
  llm: 'aiapi',
  openai:'openai', anthropic:'anthropic', ark:'ark', qwen:'qwen', gemini:'gemini', ollama:'ollama', 'openai-compatible':'openai-compatible',
  ssh: 'webssh',
  rdp: 'webrdp',
  vnc: 'webvnc',
  mysql: 'webmysql',
  mariadb: 'webmariadb',
  doris: 'webdoris',
  starrocks: 'webstarrocks',
  tidb: 'webtidb',
  sqlserver: 'websqlserver',
  oracle: 'weboracle',
  dameng: 'webdameng',
  clickhouse: 'webclickhouse',
  elasticsearch: 'webelasticsearch',
  opensearch: 'webopensearch',
  postgresql: 'webpostgresql',
  redis: 'webredis',
  memcached: 'webmemcached',
  s3: 'webs3',
  smb: 'websmb',
  mongodb: 'webmongodb',
};

export const ACCESS_TYPES_CHANGED_EVENT = 'liaison:access-types-changed';

export const isAccessType = (
  value: string | null | undefined,
): value is AccessType =>
  KNOWN_ACCESS_TYPES.some((accessType) => accessType.value === value);

export const isSupportedAccessType = (
  value: string | null | undefined,
) => optionalProtocolEnabled(value||'') && (value==='openai-compatible'||value==='aiapi'||ACCESS_TYPES.some((accessType) => accessType.value === value));

export const accessTypeLabel = (value: string | null | undefined) =>
  KNOWN_ACCESS_TYPES.find((item) => item.value === value)?.label || value || '-';

export const isWebAccessType = (
  value: string | null | undefined,
): value is WebAccessType => value==='openai-compatible'||value==='aiapi'||WEB_ACCESS_TYPES.some((item) => item.value === value);

export const applicationTypeForAccess = (accessType: AccessType): ApplicationType => {
  if (accessType === 'websftp') return 'ssh';
  const webEntry = Object.entries(WEB_TYPE_BY_APPLICATION).find(([, value]) => value === accessType);
  return (webEntry?.[0] || accessType) as ApplicationType;
};

export const accessProtocolForType = (accessType: AccessType) => {
  if (isLLMAccessType(accessType)) return 'aiapi';
  if (accessType === 'webssh') return 'webssh';
  if (accessType === 'websftp') return 'websftp';
  return isWebAccessType(accessType) ? 'web' : accessType;
};

export const accessTypesForApplication = (applicationType: string): AccessTypeOption[] => {
  if (applicationType === 'http') return [{value: 'http', label: 'HTTP'}, {value: 'tcp', label: 'TCP'}];
  if(applicationType==='llm')return [{value:'aiapi',label:'LLM protocol'}, {value:'tcp',label:'TCP'}];
  const native = NATIVE_ACCESS_TYPES.find((item) => item.value === applicationType);
  const webType = WEB_TYPE_BY_APPLICATION[protocolFamily(applicationType) as ApplicationType];
  return [
    ...(webType ? WEB_ACCESS_TYPES.filter((item) => item.value === webType && optionalProtocolEnabled(item.value)) : []),
    ...(applicationType === 'ssh' ? WEB_ACCESS_TYPES.filter((item) => item.value === 'websftp') : []),
    { value: 'tcp', label: 'TCP' },
    ...(applicationType !== 'tcp' && native ? [native] : []),
  ];
};

export const isProxyPublicPortExposed = (proxy?: API.Proxy) =>
  Boolean(proxy?.expose_public_port ?? (proxy?.port || 0) > 0);

export const getProxyAccessType = (
  proxy?: API.Proxy,
): AccessType | undefined => {
  const accessProtocol = String(proxy?.access_protocol || '').toLowerCase();
  const applicationType = String(
    proxy?.application?.application_type || '',
  ).toLowerCase();
  if (!applicationType) return undefined;
  if(accessProtocol==='aiapi')return isLLMAccessType(applicationType)?protocolFamily(applicationType) as AccessType:'aiapi';
  if (accessProtocol === 'web') {
    return WEB_TYPE_BY_APPLICATION[applicationType as ApplicationType];
  }
  if (accessProtocol === 'webssh') return 'webssh';
  if (isAccessType(accessProtocol)) return accessProtocol;
  if (applicationType === 'ssh' && !isProxyPublicPortExposed(proxy)) {
    return 'webssh';
  }
  // Legacy records do not have access_protocol. Keep their historical routing.
  if (applicationType !== 'http' && isProxyPublicPortExposed(proxy)) {
    return 'tcp';
  }
  return isAccessType(applicationType) ? applicationType : undefined;
};
