import {LLM_PROTOCOL_OPTIONS} from './llmProtocols';
import {optionalProtocolEnabled} from '../store/optionalProtocols';
export const APPLICATION_TYPES = [
  ...LLM_PROTOCOL_OPTIONS,
  { value: 'http', label: 'HTTP' },
  { value: 'tcp', label: 'TCP' },
  { value: 'ssh', label: 'SSH' },
  { value: 'rdp', label: 'RDP' },
  { value: 'vnc', label: 'VNC' },
  { value: 'mysql', label: 'MySQL' },
  { value: 'mariadb', label: 'MariaDB' },
  { value: 'doris', label: 'Doris' },
  { value: 'starrocks', label: 'StarRocks' },
  { value: 'tidb', label: 'TiDB' },
  { value: 'sqlserver', label: 'SQL Server' },
  { value: 'oracle', label: 'Oracle' },
  { value: 'dameng', label: 'Dameng' },
  { value: 'clickhouse', label: 'ClickHouse' },
  { value: 'elasticsearch', label: 'Elasticsearch' },
  { value: 'opensearch', label: 'OpenSearch' },
  { value: 'postgresql', label: 'PostgreSQL' },
  { value: 'redis', label: 'Redis' },
  { value: 'memcached', label: 'Memcached' },
  { value: 's3', label: 'S3' },
  { value: 'smb', label: 'SMB' },
  { value: 'mongodb', label: 'MongoDB' },
] as const;

export type ApplicationType = (typeof APPLICATION_TYPES)[number]['value'] | 'llm' | 'openai-compatible';

export const APPLICATION_TYPES_CHANGED_EVENT =
  'liaison:application-types-changed';

export const availableApplicationTypes = () => APPLICATION_TYPES.filter(item=>optionalProtocolEnabled(item.value));

export const isApplicationType = (
  value: string | null | undefined,
): value is ApplicationType =>
  value==='llm'||value==='openai-compatible'||APPLICATION_TYPES.some((applicationType) => applicationType.value === value);
