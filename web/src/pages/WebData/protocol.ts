export const protocolLabels: Record<string, string> = {
  mysql: 'MySQL',
  postgresql: 'PostgreSQL',
  redis: 'Redis',
  mongodb: 'MongoDB',
};

export const isSQLProtocol = (protocol?: string) =>
  ['mysql', 'postgresql'].includes(String(protocol || '').toLowerCase());
