export const APPLICATION_TYPES = [
  { value: 'llm', label: 'LLM API' },
  { value: 'http', label: 'HTTP' },
  { value: 'tcp', label: 'TCP' },
  { value: 'ssh', label: 'SSH' },
  { value: 'rdp', label: 'RDP' },
  { value: 'vnc', label: 'VNC' },
  { value: 'mysql', label: 'MySQL' },
  { value: 'mariadb', label: 'MariaDB' },
  { value: 'sqlserver', label: 'SQL Server' },
  { value: 'oracle', label: 'Oracle' },
  { value: 'clickhouse', label: 'ClickHouse' },
  { value: 'elasticsearch', label: 'Elasticsearch' },
  { value: 'opensearch', label: 'OpenSearch' },
  { value: 'postgresql', label: 'PostgreSQL' },
  { value: 'redis', label: 'Redis' },
  { value: 'mongodb', label: 'MongoDB' },
] as const;

export type ApplicationType = (typeof APPLICATION_TYPES)[number]['value'];

export const APPLICATION_TYPES_CHANGED_EVENT =
  'liaison:application-types-changed';

export const isApplicationType = (
  value: string | null | undefined,
): value is ApplicationType =>
  APPLICATION_TYPES.some((applicationType) => applicationType.value === value);
