export const protocolLabels: Record<string, string> = {
  mysql: 'MySQL',
  postgresql: 'PostgreSQL',
  redis: 'Redis',
  mongodb: 'MongoDB',
};

export const isSQLProtocol = (protocol?: string) =>
  ['mysql', 'postgresql'].includes(String(protocol || '').toLowerCase());

type Translate = (zh: string, en: string) => string;

export const protocolWorkspaceCopy = (
  protocol: string | undefined,
  tr: Translate,
) => {
  switch (String(protocol || '').toLowerCase()) {
    case 'redis':
      return {
        navigatorTitle: tr('键空间', 'Keyspace'),
        searchPlaceholder: tr('搜索数据库或键', 'Search databases or keys'),
        editorTitle: tr('Redis 命令', 'Redis Command'),
        resultTitle: tr('执行结果', 'Execution Result'),
      };
    case 'mongodb':
      return {
        navigatorTitle: tr('数据库对象', 'Database Objects'),
        searchPlaceholder: tr('搜索数据库或集合', 'Search databases or collections'),
        editorTitle: tr('MongoDB 命令', 'MongoDB Command'),
        resultTitle: tr('执行结果', 'Execution Result'),
      };
    default:
      return {
        navigatorTitle: tr('数据库对象', 'Database Objects'),
        searchPlaceholder: tr('搜索库、表、字段', 'Search databases, tables, or fields'),
        editorTitle: tr('SQL 编辑器', 'SQL Editor'),
        resultTitle: tr('查询结果', 'Query Result'),
      };
  }
};
