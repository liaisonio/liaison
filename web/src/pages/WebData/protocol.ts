export const protocolLabels: Record<string, string> = {
  mysql: 'MySQL',
  mariadb: 'MariaDB',
  sqlserver: 'SQL Server',
  oracle: 'Oracle',
  clickhouse: 'ClickHouse',
  elasticsearch: 'Elasticsearch',
  opensearch: 'OpenSearch',
  postgresql: 'PostgreSQL',
  redis: 'Redis',
  mongodb: 'MongoDB',
};

export const isSQLProtocol = (protocol?: string) =>
  ['mysql', 'mariadb', 'sqlserver', 'oracle', 'clickhouse', 'postgresql'].includes(String(protocol || '').toLowerCase());

type Translate = (zh: string, en: string) => string;

export const protocolWorkspaceCopy = (
  protocol: string | undefined,
  tr: Translate,
) => {
  switch (String(protocol || '').toLowerCase()) {
    case 'elasticsearch':
    case 'opensearch':
      return {
        navigatorTitle: tr('索引', 'Indices'),
        searchPlaceholder: tr('搜索索引', 'Search indices'),
        editorTitle: tr('JSON 请求', 'JSON Request'),
        resultTitle: tr('搜索结果', 'Search Results'),
      };
    case 'redis':
      return {
        navigatorTitle: tr('键空间', 'Keyspace'),
        searchPlaceholder: tr('搜索键名', 'Search keys'),
        editorTitle: tr('命令编辑器', 'Command Editor'),
        resultTitle: tr('命令结果', 'Command Result'),
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
