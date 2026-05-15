import { isSQLProtocol } from './protocol';
import type {
  ObjectFilterCondition,
  ObjectFilterFieldInfo,
  ObjectFilterFieldKind,
  ObjectFilterState,
  WebDataColumnInfo,
} from './types';

export const objectFilterOperatorOptions = (
  tr: (zh: string, en: string) => string,
  kind: ObjectFilterFieldKind = 'unknown',
) =>
  [
    { value: 'eq', label: '=' },
    { value: 'ne', label: '!=' },
    { value: 'contains', label: tr('包含', 'Contains') },
    { value: 'starts_with', label: tr('开头是', 'Starts with') },
    { value: 'ends_with', label: tr('结尾是', 'Ends with') },
    { value: 'gt', label: '>' },
    { value: 'gte', label: '>=' },
    { value: 'lt', label: '<' },
    { value: 'lte', label: '<=' },
    { value: 'is_null', label: tr('为空', 'Is null') },
    { value: 'not_null', label: tr('不为空', 'Not null') },
  ].filter((option) => filterOperatorAllowedForKind(option.value, kind));

export const buildSQLInsertCommand = (
  protocol: string,
  detail: API.WebDataObjectResult,
  values: any,
) => {
  const tableName = sqlQualifiedName(protocol, detail);
  if (!tableName) throw new Error('missing table name');
  const fields = values?.values || {};
  const columns = sqlEditableColumns(detail).filter(
    (column) => String(fields[column.name] ?? '').trim() !== '',
  );
  if (!columns.length) {
    throw new Error('没有可写入字段');
  }
  return `INSERT INTO ${tableName} (${columns
    .map((column) => sqlQuoteIdent(protocol, column.name))
    .join(', ')})\nVALUES (${columns
    .map((column) => sqlLiteralForColumn(protocol, column, fields[column.name]))
    .join(', ')});`;
};

export const buildSQLUpdateRowCommand = (
  protocol: string,
  detail: API.WebDataObjectResult,
  values: any,
  originalRow?: Record<string, any>,
) => {
  const tableName = sqlQualifiedName(protocol, detail);
  if (!tableName) throw new Error('missing table name');
  const whereColumn = String(values?.where_column || '').trim();
  if (!whereColumn) throw new Error('WHERE column is required');
  const fields = values?.values || {};
  const editableColumns = sqlEditableColumns(detail);
  const columns = editableColumns.filter((column) => {
    if (column.name === whereColumn) return false;
    const nextValue = fields[column.name];
    if (String(nextValue ?? '').trim() === '') return false;
    if (!originalRow) return true;
    return !sqlFormValueEqualsRowValue(nextValue, originalRow[column.name]);
  });
  if (!columns.length) {
    throw new Error('没有变更字段');
  }
  const whereColumnInfo =
    editableColumns.find((column) => column.name === whereColumn) ||
    sqlColumns(detail).find((column) => column.name === whereColumn);
  return `UPDATE ${tableName}\nSET ${columns
    .map(
      (column) =>
        `${sqlQuoteIdent(protocol, column.name)} = ${sqlLiteralForColumn(
          protocol,
          column,
          fields[column.name],
        )}`,
    )
    .join(', ')}\nWHERE ${sqlQuoteIdent(protocol, whereColumn)} = ${sqlLiteral(
    sqlNormalizeColumnValue(protocol, whereColumnInfo, values?.where_value),
  )};`;
};

export const buildSQLUpdateCellCommand = (
  protocol: string,
  detail: API.WebDataObjectResult,
  row: Record<string, any>,
  columnName: string,
  value: any,
) => {
  const tableName = sqlQualifiedName(protocol, detail);
  if (!tableName) throw new Error('missing table name');
  const identity = sqlIdentityColumn(protocol, detail, row);
  if (columnName === identity.column) {
    throw new Error('标识列不支持直接编辑');
  }
  const column = sqlEditableColumns(detail).find(
    (item) => item.name === columnName,
  );
  if (!column) throw new Error('当前字段不支持编辑');
  if (sqlFormValueEqualsRowValue(value, row[columnName])) {
    throw new Error('字段没有变化');
  }
  const identityColumnInfo = sqlColumns(detail).find(
    (item) => item.name === identity.column,
  );
  return `UPDATE ${tableName}\nSET ${sqlQuoteIdent(
    protocol,
    column.name,
  )} = ${sqlLiteralForColumn(protocol, column, value)}\nWHERE ${sqlQuoteIdent(
    protocol,
    identity.column,
  )} = ${sqlLiteralForColumn(protocol, identityColumnInfo, identity.value)};`;
};

export const buildSQLDeleteRowCommand = (
  protocol: string,
  detail: API.WebDataObjectResult,
  row: Record<string, any>,
) => {
  const tableName = sqlQualifiedName(protocol, detail);
  if (!tableName) throw new Error('missing table name');
  const identity = sqlIdentityColumn(protocol, detail, row);
  if (!identity.column) throw new Error('WHERE column is required');
  const identityColumnInfo = sqlColumns(detail).find(
    (column) => column.name === identity.column,
  );
  return `DELETE FROM ${tableName}\nWHERE ${sqlQuoteIdent(
    protocol,
    identity.column,
  )} = ${sqlLiteralForColumn(protocol, identityColumnInfo, identity.value)};`;
};

export const buildRedisSetCommand = (values: any) => {
  const key = String(values?.key || '').trim();
  if (!key) throw new Error('Key 不能为空');
  const ttl = Number(values?.ttl || 0);
  const base = `SET ${quoteRedisArg(key)} ${quoteRedisArgAllowEmpty(
    String(values?.value ?? ''),
  )}`;
  if (ttl > 0) return `${base} EX ${Math.floor(ttl)}`;
  return base;
};

export const buildRedisMemberCommand = (
  detail: API.WebDataObjectResult,
  values: any,
  update: boolean,
) => {
  const key = String(values?.key || detail.key || detail.name || '').trim();
  if (!key) throw new Error('Key 不能为空');
  const quotedKey = quoteRedisArg(key);
  const redisType = redisObjectType(detail);
  switch (redisType) {
    case 'hash': {
      const field = String(values?.field || '').trim();
      if (!field) throw new Error('字段不能为空');
      return `HSET ${quotedKey} ${quoteRedisArg(
        field,
      )} ${quoteRedisArgAllowEmpty(String(values?.value ?? ''))}`;
    }
    case 'list': {
      if (update) {
        const index = Number(values?.index);
        if (!Number.isInteger(index) || index < 0) {
          throw new Error('Index 不正确');
        }
        return `LSET ${quotedKey} ${index} ${quoteRedisArgAllowEmpty(
          String(values?.value ?? ''),
        )}`;
      }
      const direction =
        String(values?.direction || 'RPUSH').toUpperCase() === 'LPUSH'
          ? 'LPUSH'
          : 'RPUSH';
      return `${direction} ${quotedKey} ${quoteRedisArgAllowEmpty(
        String(values?.value ?? ''),
      )}`;
    }
    case 'set': {
      const member = String(values?.member ?? '');
      if (member === '') throw new Error('成员不能为空');
      return `SADD ${quotedKey} ${quoteRedisArgAllowEmpty(member)}`;
    }
    case 'zset': {
      const member = String(values?.member ?? '');
      const score = Number(values?.score);
      if (member === '') throw new Error('成员不能为空');
      if (!Number.isFinite(score)) throw new Error('Score 不正确');
      return `ZADD ${quotedKey} ${score} ${quoteRedisArgAllowEmpty(member)}`;
    }
    case 'stream': {
      const field = String(values?.field || '').trim();
      if (!field) throw new Error('字段不能为空');
      return `XADD ${quotedKey} * ${quoteRedisArg(
        field,
      )} ${quoteRedisArgAllowEmpty(String(values?.value ?? ''))}`;
    }
    default:
      throw new Error('当前 Redis 类型暂不支持快捷成员操作');
  }
};

export const buildRedisRowDeleteCommand = (
  detail: API.WebDataObjectResult,
  row: Record<string, any>,
) => {
  const key = String(detail.key || detail.name || '').trim();
  if (!key) throw new Error('Key 不能为空');
  const quotedKey = quoteRedisArg(key);
  const redisType = redisObjectType(detail);
  switch (redisType) {
    case 'hash': {
      const field = String(row.field || '').trim();
      if (!field) throw new Error('字段不能为空');
      return `HDEL ${quotedKey} ${quoteRedisArg(field)}`;
    }
    case 'set': {
      const member = String(row.value ?? '');
      if (member === '') throw new Error('成员不能为空');
      return `SREM ${quotedKey} ${quoteRedisArgAllowEmpty(member)}`;
    }
    case 'zset': {
      const member = String(row.member ?? '');
      if (member === '') throw new Error('成员不能为空');
      return `ZREM ${quotedKey} ${quoteRedisArgAllowEmpty(member)}`;
    }
    case 'stream': {
      const id = String(row.id || '').trim();
      if (!id) throw new Error('消息 ID 不能为空');
      return `XDEL ${quotedKey} ${quoteRedisArg(id)}`;
    }
    default:
      throw new Error('当前 Redis 类型暂不支持快捷删除');
  }
};

export const defaultRedisMemberValues = (
  detail: API.WebDataObjectResult,
  mode: 'add' | 'update',
  row?: Record<string, any>,
) => {
  const key = detail.key || detail.name || '';
  const redisType = redisObjectType(detail);
  if (mode === 'add') {
    if (redisType === 'list') {
      return { key, direction: 'RPUSH', value: '' };
    }
    if (redisType === 'zset') {
      return { key, member: '', score: 0 };
    }
    if (redisType === 'stream') {
      return { key, field: 'field', value: '' };
    }
    if (redisType === 'hash') {
      return { key, field: '', value: '' };
    }
    return { key, member: '' };
  }
  if (redisType === 'hash') {
    return { key, field: row?.field || '', value: row?.value ?? '' };
  }
  if (redisType === 'list') {
    return { key, index: row?.index ?? 0, value: row?.value ?? '' };
  }
  if (redisType === 'zset') {
    return { key, member: row?.member || '', score: row?.score ?? 0 };
  }
  throw new Error('当前 Redis 类型暂不支持快捷更新');
};

export const redisObjectType = (detail: API.WebDataObjectResult) => {
  const typeRow = (detail.extra || []).find((row) => row.property === 'type');
  return String(typeRow?.value || '').toLowerCase();
};

export const canAddRedisMember = (detail: API.WebDataObjectResult) =>
  ['hash', 'list', 'set', 'zset', 'stream'].includes(redisObjectType(detail));

export const canUpdateRedisRow = (detail: API.WebDataObjectResult) =>
  ['hash', 'list', 'zset'].includes(redisObjectType(detail));

export const canDeleteRedisRow = (detail: API.WebDataObjectResult) =>
  ['hash', 'set', 'zset', 'stream'].includes(redisObjectType(detail));

export const hasRedisRowActions = (detail: API.WebDataObjectResult) =>
  canUpdateRedisRow(detail) || canDeleteRedisRow(detail);

export const redisActionDescription = (redisType: string) => {
  switch (redisType) {
    case 'hash':
      return 'Hash 使用 HSET 写入字段，预览行可 HDEL 删除字段。';
    case 'list':
      return 'List 新增使用 LPUSH/RPUSH，预览行更新使用 LSET。';
    case 'set':
      return 'Set 使用 SADD 添加成员，预览行可 SREM 删除成员。';
    case 'zset':
      return 'ZSet 使用 ZADD 写入成员和分数，预览行可 ZREM 删除成员。';
    case 'stream':
      return 'Stream 使用 XADD 追加消息，预览行可 XDEL 删除消息。';
    default:
      return '当前 Redis 类型暂不支持快捷成员操作。';
  }
};

export const buildMongoInsertCommand = (
  detail: API.WebDataObjectResult,
  values: any,
) => {
  const collection = detail.name;
  if (!collection) throw new Error('collection is required');
  let parsed: any;
  try {
    parsed = JSON.parse(String(values?.document || ''));
  } catch {
    throw new Error('JSON 格式不正确');
  }
  const documents = Array.isArray(parsed) ? parsed : [parsed];
  if (!documents.length) throw new Error('文档不能为空');
  return JSON.stringify({ insert: collection, documents }, null, 2);
};

export const buildMongoUpdateCommand = (
  detail: API.WebDataObjectResult,
  row: Record<string, any> | undefined,
  values: any,
) => {
  const collection = detail.name;
  if (!collection) throw new Error('collection is required');
  const identity = mongoDocumentIdentity(row);
  let parsed: any;
  try {
    parsed = JSON.parse(String(values?.document || ''));
  } catch {
    throw new Error('JSON 格式不正确');
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error('$set 必须是 JSON 对象');
  }
  const document = stripMongoInternalFields(parsed);
  if (!Object.keys(document).length) {
    throw new Error('没有可更新字段');
  }
  return JSON.stringify(
    {
      update: collection,
      updates: [{ q: { _id: identity }, u: { $set: document }, multi: false }],
    },
    null,
    2,
  );
};

export const buildMongoUpdateFieldCommand = (
  detail: API.WebDataObjectResult,
  row: Record<string, any>,
  field: string,
  value: string,
) => {
  const collection = detail.name;
  if (!collection) throw new Error('collection is required');
  if (!field || field === '_id') throw new Error('当前字段不支持编辑');
  const identity = mongoDocumentIdentity(row);
  const parsedValue = parseMongoInlineValue(value, row[field]);
  if (JSON.stringify(parsedValue) === JSON.stringify(row[field])) {
    throw new Error('字段没有变化');
  }
  return JSON.stringify(
    {
      update: collection,
      updates: [
        {
          q: { _id: identity },
          u: { $set: { [field]: parsedValue } },
          multi: false,
        },
      ],
    },
    null,
    2,
  );
};

export const buildMongoDeleteCommand = (
  detail: API.WebDataObjectResult,
  row: Record<string, any>,
) => {
  const collection = detail.name;
  if (!collection) throw new Error('collection is required');
  const identity = mongoDocumentIdentity(row);
  return JSON.stringify(
    { delete: collection, deletes: [{ q: { _id: identity }, limit: 1 }] },
    null,
    2,
  );
};

export const mongoDocumentIdentity = (row?: Record<string, any>) => {
  const identity = row?._id;
  if (identity === undefined || identity === null || identity === '') {
    throw new Error('当前文档没有 _id，无法生成精确操作');
  }
  return identity;
};

export const parseMongoInlineValue = (value: string, originalValue: any) => {
  const text = String(value ?? '');
  const trimmed = text.trim();
  if (originalValue === null && /^null$/i.test(trimmed)) return null;
  if (typeof originalValue === 'number') {
    const parsed = Number(trimmed);
    if (Number.isFinite(parsed)) return parsed;
    throw new Error('数字格式不正确');
  }
  if (typeof originalValue === 'boolean') {
    if (/^true$/i.test(trimmed)) return true;
    if (/^false$/i.test(trimmed)) return false;
    throw new Error('布尔值请输入 true 或 false');
  }
  if (typeof originalValue === 'object' && originalValue !== null) {
    try {
      return JSON.parse(trimmed);
    } catch {
      throw new Error('JSON 格式不正确');
    }
  }
  if (typeof originalValue === 'string') {
    return text;
  }
  if (/^(null|true|false)$/i.test(trimmed) || /^-?\d+(\.\d+)?$/.test(trimmed)) {
    try {
      return JSON.parse(trimmed.toLowerCase());
    } catch {
      return text;
    }
  }
  if (/^[\[{"]/.test(trimmed)) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return text;
    }
  }
  return text;
};

export const mongoEditableDocument = (row: Record<string, any>) =>
  stripMongoInternalFields(row);

export const stripMongoInternalFields = (row: Record<string, any>) => {
  const document = { ...(row || {}) };
  delete document._id;
  delete document._webdata_key;
  return document;
};

export const sqlColumns = (
  detail: API.WebDataObjectResult,
): WebDataColumnInfo[] =>
  (detail.columns || [])
    .map((row) => ({
      name: objectColumnName(row),
      type: objectColumnType(row),
      generated: isGeneratedColumn(row),
      raw: row,
    }))
    .filter((column) => column.name);

export const sqlEditableColumns = (
  detail: API.WebDataObjectResult,
): WebDataColumnInfo[] =>
  sqlColumns(detail).filter((column) => !column.generated);

export const sqlIdentityColumn = (
  protocol: string,
  detail: API.WebDataObjectResult,
  row: Record<string, any>,
) => {
  const primaryColumn =
    protocol === 'mysql'
      ? mysqlPrimaryColumn(detail)
      : postgresPrimaryColumn(detail);
  const fallbackColumn =
    primaryColumn || objectColumnNames(detail).find((column) => column in row);
  if (!fallbackColumn) {
    throw new Error('无法确定 WHERE 字段');
  }
  return { column: fallbackColumn, value: row[fallbackColumn] };
};

export const canIdentifySQLRow = (
  protocol: string,
  detail: API.WebDataObjectResult,
  row: Record<string, any>,
) => {
  try {
    sqlIdentityColumn(protocol, detail, row);
    return true;
  } catch {
    return false;
  }
};

export const canIdentifyMongoRow = (row: Record<string, any>) => {
  const identity = row?._id;
  return identity !== undefined && identity !== null && identity !== '';
};

export const mysqlPrimaryColumn = (detail: API.WebDataObjectResult) => {
  const column = sqlColumns(detail).find(
    (item) =>
      String(item.raw.Key || item.raw.key || '').toUpperCase() === 'PRI',
  );
  return column?.name || '';
};

export const postgresPrimaryColumn = (detail: API.WebDataObjectResult) => {
  const primaryIndex = (detail.indexes || []).find((item) =>
    String(item.indexname || item.name || '').endsWith('_pkey'),
  );
  const indexDef = String(primaryIndex?.indexdef || '');
  const match = indexDef.match(/\(([^)]+)\)/);
  if (!match) return '';
  return match[1].split(',')[0].trim().replace(/^"|"$/g, '');
};

export const objectColumnName = (row: Record<string, any>) =>
  String(
    row.Field ||
      row.field ||
      row.column_name ||
      row.COLUMN_NAME ||
      row.name ||
      '',
  );

export const objectColumnType = (row: Record<string, any>) =>
  String(
    row.Type ||
      row.type ||
      row.data_type ||
      row.DATA_TYPE ||
      row.column_type ||
      '',
  );

export const isGeneratedColumn = (row: Record<string, any>) => {
  const extra = String(row.Extra || row.extra || '').toLowerCase();
  const defaultValue = String(
    row.Default || row.default || row.column_default || '',
  ).toLowerCase();
  return (
    extra.includes('auto_increment') || defaultValue.startsWith('nextval(')
  );
};

export const sqlFormValueEqualsRowValue = (nextValue: any, rowValue: any) =>
  String(nextValue ?? '').trim() === String(rowValue ?? '').trim();

export const sqlLiteralForColumn = (
  protocol: string,
  column: WebDataColumnInfo | undefined,
  value: any,
) => sqlLiteral(sqlNormalizeColumnValue(protocol, column, value));

export const sqlNormalizeColumnValue = (
  protocol: string,
  column: WebDataColumnInfo | undefined,
  value: any,
) => {
  const text = String(value ?? '').trim();
  if (!text || /^null$/i.test(text)) return text;
  if (protocol !== 'mysql' || !isSQLTemporalColumn(column)) return text;
  return normalizeMySQLTemporalLiteral(column?.type || '', text);
};

export const isSQLTemporalColumn = (column?: WebDataColumnInfo) =>
  /\b(date|time|timestamp|datetime)\b/i.test(column?.type || '');

export const normalizeMySQLTemporalLiteral = (type: string, value: string) => {
  const normalizedType = type.toLowerCase();
  const isoMatch = value.match(
    /^(\d{4}-\d{2}-\d{2})[tT\s](\d{2}:\d{2}:\d{2})(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?$/,
  );
  if (!isoMatch) return value;
  if (
    /\bdate\b/.test(normalizedType) &&
    !/\b(datetime|timestamp)\b/.test(normalizedType)
  ) {
    return isoMatch[1];
  }
  if (
    /\btime\b/.test(normalizedType) &&
    !/\b(datetime|timestamp)\b/.test(normalizedType)
  ) {
    return isoMatch[2];
  }
  return `${isoMatch[1]} ${isoMatch[2]}`;
};

export const sqlLiteral = (value: any) => {
  const text = String(value ?? '').trim();
  if (/^null$/i.test(text)) return 'NULL';
  if (/^-?\d+(\.\d+)?$/.test(text)) return text;
  if (/^(true|false)$/i.test(text)) return text.toLowerCase();
  return `'${text.replace(/'/g, "''")}'`;
};

export const createObjectFilterCondition = (
  field?: string,
  operator = 'eq',
): ObjectFilterCondition => ({
  id: `filter-${Date.now()}-${Math.random().toString(36).slice(2)}`,
  field,
  operator,
  value: '',
});

export const createDefaultObjectFilter = (
  field?: string,
  operator = 'eq',
): ObjectFilterState => ({
  conditions: field ? [createObjectFilterCondition(field, operator)] : [],
  limit: 100,
});

export const normalizeFilterFieldInfos = (
  fields: string[] | ObjectFilterFieldInfo[],
): ObjectFilterFieldInfo[] =>
  fields.map((field) =>
    typeof field === 'string'
      ? { name: field, kind: 'unknown' }
      : {
          name: field.name,
          kind: field.kind || 'unknown',
          typeLabel: field.typeLabel,
        },
  );

export const normalizeObjectFilter = (
  filter: ObjectFilterState,
  fields: string[] | ObjectFilterFieldInfo[],
): ObjectFilterState => {
  const fieldInfos = normalizeFilterFieldInfos(fields);
  const fieldNames = fieldInfos.map((field) => field.name);
  if (!fieldNames.length) {
    return { conditions: [], limit: filter.limit || 100 };
  }
  const conditions = filter.conditions.length
    ? filter.conditions
    : [createObjectFilterCondition(fieldNames[0])];
  return {
    limit: filter.limit || 100,
    conditions: conditions.map((condition) => {
      const field = fieldNames.includes(condition.field || '')
        ? String(condition.field)
        : fieldNames[0];
      const kind = objectFilterFieldKind(fieldInfos, field);
      const operator = coerceFilterOperator(condition.operator, kind);
      return {
        ...condition,
        field,
        operator,
        value: filterOperatorNeedsValue(operator) ? condition.value : '',
      };
    }),
  };
};

export const filterOperatorNeedsValue = (operator?: string) =>
  !['is_null', 'not_null'].includes(String(operator || ''));

export const filterOperatorAllowedForKind = (
  operator: string,
  kind: ObjectFilterFieldKind,
) => {
  if (['is_null', 'not_null'].includes(operator)) return true;
  if (['eq', 'ne'].includes(operator)) return true;
  if (kind === 'number' || kind === 'date') {
    return ['gt', 'gte', 'lt', 'lte'].includes(operator);
  }
  if (kind === 'text') {
    return ['contains', 'starts_with', 'ends_with'].includes(operator);
  }
  if (kind === 'unknown') return true;
  return false;
};

export const defaultFilterOperatorForKind = (_kind: ObjectFilterFieldKind) =>
  'eq';

export const coerceFilterOperator = (
  operator: string | undefined,
  kind: ObjectFilterFieldKind,
) => {
  const nextOperator = operator || defaultFilterOperatorForKind(kind);
  return filterOperatorAllowedForKind(nextOperator, kind)
    ? nextOperator
    : defaultFilterOperatorForKind(kind);
};

export const objectFilterFieldKind = (
  fieldInfos: ObjectFilterFieldInfo[],
  field?: string,
) => fieldInfos.find((item) => item.name === field)?.kind || 'unknown';

export const objectFilterValuePlaceholder = (
  kind: ObjectFilterFieldKind,
  tr: (zh: string, en: string) => string,
) => {
  if (kind === 'number') return '123';
  if (kind === 'date') return '2026-05-14 12:00:00';
  if (kind === 'boolean') return 'true / false';
  if (kind === 'object') return '{ "key": "value" }';
  return tr('筛选值', 'Filter value');
};

export const objectFilterFieldInfos = (
  protocol?: string,
  detail?: API.WebDataObjectResult,
): ObjectFilterFieldInfo[] => {
  if (!detail) return [];
  if (isSQLProtocol(protocol)) {
    const columns = sqlColumns(detail);
    return columns.map((column) => ({
      name: column.name,
      kind: classifySQLColumnKind(column.type),
      typeLabel: column.type || undefined,
    }));
  }
  if (protocol === 'mongodb') {
    return (detail.columns || [])
      .map((column) => {
        const name = objectColumnName(column);
        const typeLabel = objectColumnType(column);
        const kind =
          typeLabel.toLowerCase() === 'objectid'
            ? 'text'
            : classifyMongoColumnKind(typeLabel);
        return {
          name,
          kind,
          typeLabel: typeLabel || mongoFilterKindLabel(kind),
        };
      })
      .filter((field) => field.name && field.name !== '_webdata_key');
  }
  return [];
};

export const classifySQLColumnKind = (type?: string): ObjectFilterFieldKind => {
  const normalized = String(type || '').toLowerCase();
  if (!normalized) return 'unknown';
  if (/\b(bool|boolean)\b/.test(normalized)) return 'boolean';
  if (
    /\b(date|time|timestamp|datetime|year|interval)\b/.test(normalized) ||
    normalized.includes('timestamptz')
  ) {
    return 'date';
  }
  if (
    /\b(int|integer|bigint|smallint|tinyint|mediumint|serial|bigserial|smallserial|decimal|numeric|number|float|double|real|money)\b/.test(
      normalized,
    )
  ) {
    return 'number';
  }
  if (/\b(json|jsonb|array|geometry|point|line|polygon)\b/.test(normalized)) {
    return 'object';
  }
  if (
    /\b(char|varchar|text|uuid|enum|set|blob|clob|binary|varbinary)\b/.test(
      normalized,
    )
  ) {
    return 'text';
  }
  return 'unknown';
};

export const classifyMongoColumnKind = (
  type?: string,
): ObjectFilterFieldKind => {
  const normalized = String(type || '').toLowerCase();
  if (!normalized || normalized === 'unknown') return 'unknown';
  if (normalized === 'objectid') return 'text';
  if (/(int|long|double|decimal|number)/.test(normalized)) return 'number';
  if (normalized.includes('bool')) return 'boolean';
  if (/(date|time)/.test(normalized)) return 'date';
  if (/(object|array|document)/.test(normalized)) return 'object';
  return 'text';
};

export const inferFilterKindFromValues = (
  values: any[],
): ObjectFilterFieldKind => {
  const kinds = values
    .filter((value) => value !== undefined && value !== null)
    .map((value) => inferFilterKindFromValue(value))
    .filter((kind) => kind !== 'unknown');
  if (!kinds.length) return 'unknown';
  if (kinds.includes('object')) return 'object';
  if (kinds.includes('text')) return 'text';
  if (kinds.every((kind) => kind === 'number')) return 'number';
  if (kinds.every((kind) => kind === 'date')) return 'date';
  if (kinds.every((kind) => kind === 'boolean')) return 'boolean';
  return 'text';
};

export const inferFilterKindFromValue = (value: any): ObjectFilterFieldKind => {
  if (value === undefined || value === null) return 'unknown';
  if (typeof value === 'number') return 'number';
  if (typeof value === 'boolean') return 'boolean';
  if (typeof value === 'string') {
    if (/^\d{4}-\d{2}-\d{2}(?:[T\s]\d{2}:\d{2}:\d{2})?/.test(value)) {
      return 'date';
    }
    return 'text';
  }
  if (Array.isArray(value)) return 'object';
  if (typeof value === 'object') {
    if ('$date' in value) return 'date';
    if (
      '$numberInt' in value ||
      '$numberLong' in value ||
      '$numberDouble' in value ||
      '$numberDecimal' in value
    ) {
      return 'number';
    }
    if ('$oid' in value) return 'text';
    return 'object';
  }
  return 'unknown';
};

export const mongoFilterKindLabel = (kind: ObjectFilterFieldKind) => {
  if (kind === 'number') return 'number';
  if (kind === 'date') return 'date';
  if (kind === 'boolean') return 'boolean';
  if (kind === 'object') return 'object';
  if (kind === 'text') return 'string';
  return undefined;
};

export const MONGO_OBJECT_ID_PATTERN = /^[a-f\d]{24}$/i;

export const isMongoObjectIdValue = (value: any) =>
  !!value &&
  typeof value === 'object' &&
  !Array.isArray(value) &&
  typeof value.$oid === 'string' &&
  MONGO_OBJECT_ID_PATTERN.test(value.$oid);

export const isMongoObjectIdField = (field: string) => {
  const normalized = String(field || '').trim();
  return normalized === '_id' || normalized.endsWith('._id');
};

export const buildObjectFilterCommand = (
  protocol: string,
  detail: API.WebDataObjectResult,
  filter: ObjectFilterState,
) => {
  if (isSQLProtocol(protocol)) {
    return buildSQLFilterCommand(protocol, detail, filter);
  }
  if (protocol === 'mongodb') {
    return buildMongoFilterCommand(detail, filter);
  }
  throw new Error('当前协议暂不支持字段筛选');
};

export const normalizeFilterLimit = (value?: number) => {
  const limit = Number(value || 100);
  if (!Number.isFinite(limit)) return 100;
  return Math.max(1, Math.min(1000, Math.floor(limit)));
};

export const assertFilterValue = (operator: string, value?: string) => {
  if (!filterOperatorNeedsValue(operator)) return;
  if (String(value ?? '').trim() === '') {
    throw new Error('请输入筛选值');
  }
};

export const activeFilterConditions = (filter: ObjectFilterState) =>
  filter.conditions.filter((condition) => String(condition.field || '').trim());

export const buildSQLFilterCommand = (
  protocol: string,
  detail: API.WebDataObjectResult,
  filter: ObjectFilterState,
) => {
  const tableName = sqlQualifiedName(protocol, detail);
  if (!tableName) throw new Error('missing table name');
  const fieldInfos = objectFilterFieldInfos(protocol, detail);
  const conditions = activeFilterConditions(filter);
  if (!conditions.length) throw new Error('请选择字段');
  const where = conditions
    .map((condition) => {
      const field = String(condition.field || '').trim();
      const operator = condition.operator || 'eq';
      assertFilterValue(operator, condition.value);
      const kind = objectFilterFieldKind(fieldInfos, field);
      return buildSQLFilterCondition(
        protocol,
        field,
        operator,
        condition.value,
        kind,
      );
    })
    .join('\n  AND ');
  return `SELECT *\nFROM ${tableName}\nWHERE ${where}\nLIMIT ${normalizeFilterLimit(
    filter.limit,
  )};`;
};

export const buildSQLFilterCondition = (
  protocol: string,
  field: string,
  operator: string,
  value?: string,
  kind: ObjectFilterFieldKind = 'unknown',
) => {
  const quotedField = sqlQuoteIdent(protocol, field);
  const likeEscape = ` ESCAPE ${sqlLiteral(SQL_LIKE_ESCAPE_CHAR)}`;
  switch (operator) {
    case 'ne':
      return `${quotedField} <> ${sqlFilterLiteral(kind, value)}`;
    case 'contains':
      return `${quotedField} LIKE ${sqlLiteral(
        `%${escapeSQLLikeValue(String(value || ''))}%`,
      )}${likeEscape}`;
    case 'starts_with':
      return `${quotedField} LIKE ${sqlLiteral(
        `${escapeSQLLikeValue(String(value || ''))}%`,
      )}${likeEscape}`;
    case 'ends_with':
      return `${quotedField} LIKE ${sqlLiteral(
        `%${escapeSQLLikeValue(String(value || ''))}`,
      )}${likeEscape}`;
    case 'gt':
      return `${quotedField} > ${sqlFilterLiteral(kind, value)}`;
    case 'gte':
      return `${quotedField} >= ${sqlFilterLiteral(kind, value)}`;
    case 'lt':
      return `${quotedField} < ${sqlFilterLiteral(kind, value)}`;
    case 'lte':
      return `${quotedField} <= ${sqlFilterLiteral(kind, value)}`;
    case 'is_null':
      return `${quotedField} IS NULL`;
    case 'not_null':
      return `${quotedField} IS NOT NULL`;
    default:
      return `${quotedField} = ${sqlFilterLiteral(kind, value)}`;
  }
};

export const buildMongoFilterCommand = (
  detail: API.WebDataObjectResult,
  filter: ObjectFilterState,
) => {
  const collection = detail.name;
  if (!collection) throw new Error('collection is required');
  const fieldInfos = objectFilterFieldInfos('mongodb', detail);
  const conditions = activeFilterConditions(filter);
  if (!conditions.length) throw new Error('请选择字段');
  const filters = conditions.map((condition) => {
    const field = String(condition.field || '').trim();
    const operator = condition.operator || 'eq';
    assertFilterValue(operator, condition.value);
    return buildMongoFilter(
      field,
      operator,
      condition.value,
      objectFilterFieldKind(fieldInfos, field),
    );
  });
  return JSON.stringify(
    {
      find: collection,
      filter: filters.length === 1 ? filters[0] : { $and: filters },
      limit: normalizeFilterLimit(filter.limit),
    },
    null,
    2,
  );
};

export const buildMongoFilter = (
  field: string,
  operator: string,
  value?: string,
  kind: ObjectFilterFieldKind = 'unknown',
) => {
  const parsedValue = parseMongoFilterValue(field, value, kind);
  switch (operator) {
    case 'ne':
      return { [field]: { $ne: parsedValue } };
    case 'contains':
      if (isMongoObjectIdField(field)) {
        return buildMongoObjectIdRegexFilter(field, String(value || ''));
      }
      return {
        [field]: {
          $regex: escapeRegexValue(String(value || '')),
          $options: 'i',
        },
      };
    case 'starts_with':
      if (isMongoObjectIdField(field)) {
        return buildMongoObjectIdRegexFilter(field, String(value || ''), '^');
      }
      return {
        [field]: {
          $regex: `^${escapeRegexValue(String(value || ''))}`,
          $options: 'i',
        },
      };
    case 'ends_with':
      if (isMongoObjectIdField(field)) {
        return buildMongoObjectIdRegexFilter(
          field,
          String(value || ''),
          '',
          '$',
        );
      }
      return {
        [field]: {
          $regex: `${escapeRegexValue(String(value || ''))}$`,
          $options: 'i',
        },
      };
    case 'gt':
      return { [field]: { $gt: parsedValue } };
    case 'gte':
      return { [field]: { $gte: parsedValue } };
    case 'lt':
      return { [field]: { $lt: parsedValue } };
    case 'lte':
      return { [field]: { $lte: parsedValue } };
    case 'is_null':
      return { [field]: null };
    case 'not_null':
      return { [field]: { $ne: null } };
    default:
      return { [field]: parsedValue };
  }
};

export const buildMongoObjectIdRegexFilter = (
  field: string,
  value: string,
  prefix = '',
  suffix = '',
) => ({
  $expr: {
    $regexMatch: {
      input: { $toString: `$${field}` },
      regex: `${prefix}${escapeRegexValue(value)}${suffix}`,
      options: 'i',
    },
  },
});

export const parseMongoFilterValue = (
  field: string,
  value: string | undefined,
  kind: ObjectFilterFieldKind,
) => {
  const text = String(value ?? '').trim();
  if (isMongoObjectIdField(field)) {
    if (MONGO_OBJECT_ID_PATTERN.test(text)) return { $oid: text };
    const objectIdCall = text.match(/^ObjectId\(["']([a-f\d]{24})["']\)$/i);
    if (objectIdCall) return { $oid: objectIdCall[1] };
    if (/^\{\s*"\$oid"\s*:/.test(text)) {
      try {
        const parsed = JSON.parse(text);
        if (isMongoObjectIdValue(parsed)) return parsed;
      } catch {
        return text;
      }
    }
  }
  return parseFilterValueByKind(value, kind);
};

export const parseFilterValueByKind = (
  value: string | undefined,
  kind: ObjectFilterFieldKind,
) => {
  const text = String(value ?? '').trim();
  if (kind === 'text' || kind === 'date') return text;
  if (kind === 'number') {
    return /^-?\d+(\.\d+)?$/.test(text) ? Number(text) : text;
  }
  if (kind === 'boolean') {
    if (/^(true|false)$/i.test(text)) return /^true$/i.test(text);
    if (/^(1|0)$/.test(text)) return text === '1';
    return text;
  }
  if (kind === 'object') {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }
  if (/^-?\d+(\.\d+)?$/.test(text)) return Number(text);
  if (/^(true|false)$/i.test(text)) return /^true$/i.test(text);
  if (/^null$/i.test(text)) return null;
  if (/^[\[{]/.test(text)) {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }
  return text;
};

export const sqlFilterLiteral = (
  kind: ObjectFilterFieldKind,
  value: string | undefined,
) => {
  const text = String(value ?? '').trim();
  if (kind === 'number') {
    return /^-?\d+(\.\d+)?$/.test(text) ? text : sqlLiteral(text);
  }
  if (kind === 'boolean') {
    if (/^(true|false)$/i.test(text)) return text.toLowerCase();
    if (/^(1|0)$/.test(text)) return text;
    return sqlLiteral(text);
  }
  if (kind === 'text' || kind === 'date' || kind === 'object') {
    return `'${text.replace(/'/g, "''")}'`;
  }
  return sqlLiteral(value);
};

export const SQL_LIKE_ESCAPE_CHAR = '!';

export const escapeSQLLikeValue = (value: string) =>
  value.replace(/[!%_]/g, (char) => `${SQL_LIKE_ESCAPE_CHAR}${char}`);

export const escapeRegexValue = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

export const buildObjectTemplate = (
  protocol: string,
  detail: API.WebDataObjectResult,
  action: string,
) => {
  if (protocol === 'redis') {
    const key = quoteRedisArg(detail.key || detail.name || '');
    if (!key) return '';
    if (action === 'preview') return redisPreviewCommand(detail, key);
    if (action === 'delete') return `DEL ${key}`;
    return '';
  }
  if (protocol === 'mongodb') {
    const collection = detail.name || '';
    if (!collection) return '';
    if (action === 'preview') {
      return JSON.stringify(
        { find: collection, filter: {}, limit: 100 },
        null,
        2,
      );
    }
    if (action === 'insert') {
      return JSON.stringify(
        { insert: collection, documents: [{ name: 'example' }] },
        null,
        2,
      );
    }
    if (action === 'update') {
      return JSON.stringify(
        {
          update: collection,
          updates: [{ q: {}, u: { $set: { name: 'example' } }, multi: false }],
        },
        null,
        2,
      );
    }
    if (action === 'delete') {
      return JSON.stringify(
        { delete: collection, deletes: [{ q: {}, limit: 1 }] },
        null,
        2,
      );
    }
    return '';
  }

  const tableName = sqlQualifiedName(protocol, detail);
  if (!tableName) return '';
  const columns = objectColumnNames(detail);
  const firstColumn = columns[0] || 'id';
  if (action === 'preview') return `SELECT *\nFROM ${tableName}\nLIMIT 100;`;
  if (action === 'ddl') return detail.ddl || '';
  if (action === 'insert') {
    const names = columns.length ? columns : ['id', 'name'];
    const placeholders = names.map(() => '?').join(', ');
    return `INSERT INTO ${tableName} (${names
      .map((item) => sqlQuoteIdent(protocol, item))
      .join(', ')})\nVALUES (${placeholders});`;
  }
  if (action === 'update') {
    return `UPDATE ${tableName}\nSET ${sqlQuoteIdent(
      protocol,
      firstColumn,
    )} = ?\nWHERE ${sqlQuoteIdent(protocol, firstColumn)} = ?;`;
  }
  if (action === 'delete') {
    return `DELETE FROM ${tableName}\nWHERE ${sqlQuoteIdent(
      protocol,
      firstColumn,
    )} = ?;`;
  }
  return '';
};

export const objectColumnNames = (detail: API.WebDataObjectResult) =>
  (detail.columns || []).map((row) => objectColumnName(row)).filter(Boolean);

export const redisPreviewCommand = (
  detail: API.WebDataObjectResult,
  key: string,
) => {
  const redisType = redisObjectType(detail);
  switch (redisType) {
    case 'string':
      return `GET ${key}`;
    case 'hash':
      return `HGETALL ${key}`;
    case 'list':
      return `LRANGE ${key} 0 99`;
    case 'set':
      return `SMEMBERS ${key}`;
    case 'zset':
      return `ZRANGE ${key} 0 99 WITHSCORES`;
    case 'stream':
      return `XRANGE ${key} - + COUNT 100`;
    default:
      return `TYPE ${key}`;
  }
};

export const sqlQualifiedName = (
  protocol: string,
  detail: API.WebDataObjectResult,
) => {
  if (!detail.name) return '';
  if (protocol === 'mysql') {
    return detail.database
      ? `${sqlQuoteIdent(protocol, detail.database)}.${sqlQuoteIdent(
          protocol,
          detail.name,
        )}`
      : sqlQuoteIdent(protocol, detail.name);
  }
  if (protocol === 'postgresql') {
    const schema = detail.schema || 'public';
    return `${sqlQuoteIdent(protocol, schema)}.${sqlQuoteIdent(
      protocol,
      detail.name,
    )}`;
  }
  return detail.name;
};

export const sqlQuoteIdent = (protocol: string, value: string) => {
  if (protocol === 'mysql') return `\`${value.replace(/`/g, '``')}\``;
  return `"${value.replace(/"/g, '""')}"`;
};

export const quoteRedisArg = (value: string) => {
  if (!value) return '';
  if (/^[^\s"']+$/.test(value)) return value;
  return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
};

export const quoteRedisArgAllowEmpty = (value: string) => {
  if (value === '') return '""';
  return quoteRedisArg(value);
};
