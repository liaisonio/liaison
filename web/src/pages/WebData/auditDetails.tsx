import { Space, Tag } from 'antd';

export const webDataAuditDetailLabels: Record<string, string> = {
  database: '数据库',
  schema: 'Schema',
  redis_db: 'Redis DB',
  affected_rows: '影响行',
  client_ip: '来源 IP',
  client_ip_source: 'IP 来源',
};

export const renderWebDataAuditDetails = (details?: Record<string, any>) => {
  const entries = Object.entries(details || {}).filter(([, value]) => {
    if (value === undefined || value === null || value === '') return false;
    if (Array.isArray(value)) return value.length > 0;
    if (typeof value === 'object') return Object.keys(value).length > 0;
    return true;
  });
  if (!entries.length) return null;
  return (
    <Space size={[8, 6]} wrap>
      {entries.map(([key, value]) => (
        <Tag key={key}>
          {webDataAuditDetailLabels[key] || key}:{' '}
          {formatAuditDetailValue(value)}
        </Tag>
      ))}
    </Space>
  );
};

export const formatAuditDetailValue = (value: any) => {
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  if (typeof value === 'number') return String(value);
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
};
