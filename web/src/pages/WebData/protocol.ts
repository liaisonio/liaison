export const protocolLabels: Record<string, string> = {
  mysql: 'MySQL',
  mariadb: 'MariaDB',
  doris: 'Doris',
  starrocks: 'StarRocks',
  tidb: 'TiDB',
  sqlserver: 'SQL Server',
  oracle: 'Oracle',
  dameng: 'Dameng',
  clickhouse: 'ClickHouse',
  elasticsearch: 'Elasticsearch',
  opensearch: 'OpenSearch',
  postgresql: 'PostgreSQL',
  redis: 'Redis',
  memcached: 'Memcached',
  mongodb: 'MongoDB',
};

export const isSQLProtocol = (protocol?: string) =>
  ['mysql', 'mariadb', 'doris', 'starrocks', 'tidb', 'sqlserver', 'oracle', 'dameng', 'clickhouse', 'postgresql'].includes(String(protocol || '').toLowerCase());

type Translate = (zh: string, en: string) => string;

export const protocolWorkspaceCopy = (
  protocol: string | undefined,
  tr: Translate,
) => {
  switch (String(protocol || '').toLowerCase()) {
    case 'memcached':
      return {navigatorTitle:tr('缓存操作','Cache operations'),searchPlaceholder:tr('指定 Key 查询，不提供全量列表','Query a specific key; no full key listing'),editorTitle:tr('缓存请求','Cache request'),resultTitle:tr('请求结果','Request result')};
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
