import {
  DatabaseOutlined,
  FileTextOutlined,
  KeyOutlined,
  TableOutlined,
} from '@ant-design/icons';
import { quoteRedisArg } from './objectCommands';
import { isSQLProtocol } from './protocol';
import type {
  CompletionContext,
  CompletionIntent,
  CompletionItem,
  CompletionPosition,
} from './types';

export const statementPlaceholder = (
  protocol: string | undefined,
  tr: (zh: string, en: string) => string,
) => {
  if (protocol === 'redis') return 'PING';
  if (protocol === 'mongodb')
    return '{ "find": "collection_name", "limit": 100 }';
  return tr('输入 SQL 语句', 'Enter SQL statement');
};

export const sqlHighlightKeywords = new Set([
  'select',
  'from',
  'where',
  'and',
  'or',
  'join',
  'inner',
  'left',
  'right',
  'outer',
  'on',
  'as',
  'group',
  'by',
  'order',
  'limit',
  'insert',
  'into',
  'values',
  'update',
  'set',
  'delete',
  'create',
  'alter',
  'drop',
  'table',
  'show',
  'describe',
  'desc',
  'explain',
  'distinct',
  'is',
  'not',
  'null',
  'like',
  'between',
]);

export const sqlCommandKeywords = [
  'SELECT',
  'WITH',
  'INSERT INTO',
  'UPDATE',
  'DELETE FROM',
  'CREATE TABLE',
  'ALTER TABLE',
  'DROP TABLE',
  'TRUNCATE TABLE',
  'EXPLAIN',
  'SHOW TABLES',
  'SHOW DATABASES',
  'SHOW COLUMNS',
  'DESCRIBE',
  'DESC',
  'SET',
  'SAVEPOINT',
  'RELEASE SAVEPOINT',
  'ROLLBACK',
  'COMMIT',
  'BEGIN',
  'START TRANSACTION',
  'USE',
  'CALL',
  'CREATE INDEX',
  'DROP INDEX',
  'CREATE VIEW',
  'DROP VIEW',
  'CREATE SCHEMA',
  'DROP SCHEMA',
  'ANALYZE',
  'OPTIMIZE TABLE',
  'LOCK TABLES',
  'UNLOCK TABLES',
  'GRANT',
  'REVOKE',
  'SIGNAL',
  'DECLARE',
  'DO',
  'VALUES',
];

export const sqlSelectFollowKeywords = [
  'FROM',
  'WHERE',
  'ORDER BY',
  'GROUP BY',
  'HAVING',
  'LIMIT 100',
  'UNION',
  'UNION ALL',
  'INNER JOIN',
  'LEFT JOIN',
  'RIGHT JOIN',
  'FULL OUTER JOIN',
  'JOIN',
];

export const sqlSelectListKeywords = [
  'AS',
  ',',
  'FROM',
  'OVER',
  'PARTITION BY',
];

export const sqlClauseKeywords = [
  'WHERE',
  'GROUP BY',
  'ORDER BY',
  'HAVING',
  'LIMIT',
  'LIMIT 100',
  'OFFSET',
  'UNION',
  'UNION ALL',
  'EXCEPT',
  'INTERSECT',
  '=',
  '<>',
  '!=',
  '>',
  '>=',
  '<',
  '<=',
  'INNER JOIN',
  'LEFT JOIN',
  'RIGHT JOIN',
  'FULL OUTER JOIN',
  'CROSS JOIN',
  'STRAIGHT_JOIN',
  'JOIN',
  'ON',
  'AND',
  'OR',
  'LIKE',
  'NOT LIKE',
  'BETWEEN',
  'IN',
  'NOT IN',
  'IS NULL',
  'IS NOT NULL',
  'EXISTS',
  'NOT EXISTS',
  'FOR UPDATE',
  'LOCK IN SHARE MODE',
  'FALSE',
  'TRUE',
  'UNKNOWN',
  'NULL',
];

export const sqlFieldKeywords = [
  '=',
  '<>',
  '!=',
  '>',
  '>=',
  '<',
  '<=',
  'IS',
  'IS NOT',
  'AND',
  'OR',
  'BETWEEN',
  'LIKE',
  'NOT LIKE',
  'IN',
  'NOT IN',
  'IS NULL',
  'IS NOT NULL',
  'EXISTS',
  'NOT EXISTS',
  'AS',
  ',',
  'ASC',
  'DESC',
  'NULLS FIRST',
  'NULLS LAST',
  'TRUE',
  'FALSE',
  'UNKNOWN',
  'NULL',
];

export const sqlExpressionTailKeywords = [
  'AND',
  'OR',
  'AS',
  ',',
  'GROUP BY',
  'ORDER BY',
  'HAVING',
  'ASC',
  'DESC',
  'NULLS FIRST',
  'NULLS LAST',
  'LIMIT',
  'LIMIT 100',
  'OFFSET',
  'UNION',
  'UNION ALL',
  'EXCEPT',
  'INTERSECT',
  'FOR UPDATE',
  'LOCK IN SHARE MODE',
];

export const sqlSelectSnippets = [
  {
    label: '*',
    insertText: '* ',
    detailZh: '所有字段',
    detailEn: 'All columns',
  },
  {
    label: 'DISTINCT()',
    insertText: 'DISTINCT ',
    detailZh: '去重',
    detailEn: 'Distinct',
  },
  {
    label: 'COUNT()',
    insertText: 'COUNT(*) ',
    detailZh: '聚合函数',
    detailEn: 'Aggregate',
  },
  {
    label: 'MAX()',
    insertText: 'MAX() ',
    detailZh: '聚合函数',
    detailEn: 'Aggregate',
  },
  {
    label: 'MIN()',
    insertText: 'MIN() ',
    detailZh: '聚合函数',
    detailEn: 'Aggregate',
  },
  {
    label: 'SUM()',
    insertText: 'SUM() ',
    detailZh: '聚合函数',
    detailEn: 'Aggregate',
  },
  {
    label: 'AVG()',
    insertText: 'AVG() ',
    detailZh: '聚合函数',
    detailEn: 'Aggregate',
  },
  {
    label: 'CASE',
    insertText: 'CASE WHEN  THEN  ELSE  END ',
    detailZh: '条件表达式',
    detailEn: 'Expression',
  },
  {
    label: 'NULL',
    insertText: 'NULL ',
    detailZh: '常量',
    detailEn: 'Constant',
  },
  {
    label: 'TRUE',
    insertText: 'TRUE ',
    detailZh: '常量',
    detailEn: 'Constant',
  },
  {
    label: 'FALSE',
    insertText: 'FALSE ',
    detailZh: '常量',
    detailEn: 'Constant',
  },
  {
    label: 'UNKNOWN',
    insertText: 'UNKNOWN ',
    detailZh: '常量',
    detailEn: 'Constant',
  },
];

export const redisCommandKeywords = [
  'GET',
  'SET',
  'DEL',
  'EXISTS',
  'EXPIRE',
  'TTL',
  'TYPE',
  'SCAN',
  'KEYS',
  'MGET',
  'MSET',
  'INCR',
  'DECR',
  'HGETALL',
  'HGET',
  'HMGET',
  'HSET',
  'HDEL',
  'HEXISTS',
  'HKEYS',
  'HVALS',
  'LRANGE',
  'LPUSH',
  'RPUSH',
  'LPOP',
  'RPOP',
  'LLEN',
  'ZRANGE',
  'ZREVRANGE',
  'ZADD',
  'ZREM',
  'ZSCORE',
  'ZCARD',
  'SADD',
  'SREM',
  'SMEMBERS',
  'SISMEMBER',
  'SCARD',
  'INFO',
  'PING',
  'DBSIZE',
  'SELECT',
  'FLUSHDB',
];

export const redisArgumentKeywords = [
  'MATCH',
  'COUNT 100',
  'TYPE',
  'WITHSCORES',
  'LIMIT',
  'NX',
  'XX',
  'EX',
  'PX',
  'KEEPTTL',
];

export const mongoCommandSnippets = [
  {
    label: 'find',
    value: { find: 'collection_name', filter: {}, limit: 100 },
    priority: 90,
  },
  {
    label: 'aggregate',
    value: { aggregate: 'collection_name', pipeline: [], cursor: {} },
    priority: 86,
  },
  {
    label: 'insert',
    value: { insert: 'collection_name', documents: [{ field: 'value' }] },
    priority: 82,
  },
  {
    label: 'update',
    value: {
      update: 'collection_name',
      updates: [{ q: {}, u: { $set: { field: 'value' } }, multi: false }],
    },
    priority: 80,
  },
  {
    label: 'delete',
    value: { delete: 'collection_name', deletes: [{ q: {}, limit: 1 }] },
    priority: 78,
  },
  {
    label: 'count',
    value: { count: 'collection_name', query: {} },
    priority: 72,
  },
  {
    label: 'distinct',
    value: { distinct: 'collection_name', key: 'field', query: {} },
    priority: 70,
  },
  {
    label: 'findAndModify',
    value: {
      findAndModify: 'collection_name',
      query: {},
      update: { $set: { field: 'value' } },
    },
    priority: 68,
  },
  {
    label: 'createIndexes',
    value: {
      createIndexes: 'collection_name',
      indexes: [{ key: { field: 1 }, name: 'field_1' }],
    },
    priority: 60,
  },
  {
    label: 'drop',
    value: { drop: 'collection_name' },
    priority: 58,
  },
  {
    label: 'listCollections',
    value: { listCollections: 1 },
    priority: 56,
  },
  {
    label: 'ping',
    value: { ping: 1 },
    priority: 54,
  },
];

export const mongoStageSnippets = [
  { label: '$match', value: { $match: {} }, priority: 90 },
  { label: '$project', value: { $project: {} }, priority: 86 },
  { label: '$sort', value: { $sort: {} }, priority: 84 },
  { label: '$limit', value: { $limit: 100 }, priority: 82 },
  { label: '$group', value: { $group: { _id: '$field' } }, priority: 80 },
  {
    label: '$lookup',
    value: { $lookup: { from: '', localField: '', foreignField: '', as: '' } },
    priority: 72,
  },
  { label: '$unwind', value: { $unwind: '$field' }, priority: 70 },
  { label: '$addFields', value: { $addFields: {} }, priority: 68 },
  { label: '$count', value: { $count: 'total' }, priority: 66 },
  { label: '$skip', value: { $skip: 0 }, priority: 64 },
  { label: '$sample', value: { $sample: { size: 10 } }, priority: 62 },
  {
    label: '$replaceRoot',
    value: { $replaceRoot: { newRoot: '$field' } },
    priority: 60,
  },
  { label: '$facet', value: { $facet: {} }, priority: 58 },
];

[
  ...sqlCommandKeywords,
  ...sqlSelectFollowKeywords,
  ...sqlSelectListKeywords,
  ...sqlClauseKeywords,
  ...sqlFieldKeywords,
  ...sqlExpressionTailKeywords,
].forEach((label) =>
  label
    .split(/\s+/)
    .filter(Boolean)
    .forEach((word) => sqlHighlightKeywords.add(word.toLowerCase())),
);

export const redisHighlightKeywords = new Set(
  [...redisCommandKeywords, ...redisArgumentKeywords].map((label) =>
    label.toLowerCase(),
  ),
);

export const mongoHighlightKeywords = new Set([
  ...mongoCommandSnippets.map((item) => item.label.toLowerCase()),
  ...mongoStageSnippets.map((item) => item.label.toLowerCase()),
  'filter',
  'limit',
  'pipeline',
  'documents',
  'updates',
  'deletes',
  '$set',
]);

export const renderStatementHighlight = (
  value: string,
  protocol: string | undefined,
) => {
  const text = value || ' ';
  const keywordSet =
    protocol === 'redis'
      ? redisHighlightKeywords
      : protocol === 'mongodb'
      ? mongoHighlightKeywords
      : sqlHighlightKeywords;
  const parts = text.split(
    /(`[^`]*`|"[^"]*"|'[^']*'|\$?[A-Za-z_][\w$]*|\b\d+(?:\.\d+)?\b|[{}[\]().,;*+=<>/-])/g,
  );
  return parts.map((part, index) => {
    if (!part) return null;
    if (/^\s+$/.test(part)) return part;
    const lower = part.toLowerCase();
    let className = 'webdata-token-default';
    if (/^`[^`]*`$|^"[^"]*"$|^'[^']*'$/.test(part)) {
      className = 'webdata-token-string';
    } else if (/^\d+(?:\.\d+)?$/.test(part)) {
      className = 'webdata-token-number';
    } else if (keywordSet.has(lower)) {
      className = 'webdata-token-keyword';
    } else if (part.startsWith('$')) {
      className = 'webdata-token-function';
    } else if (/^[{}[\]().,;*+=<>/-]$/.test(part)) {
      className = 'webdata-token-operator';
    }
    return (
      <span className={className} key={`${index}-${part}`}>
        {part}
      </span>
    );
  });
};

export const getStatementTextArea = (
  ref: any,
): HTMLTextAreaElement | undefined =>
  ref?.resizableTextArea?.textArea || ref?.textArea || ref?.nativeElement;

export const getTextareaCompletionPosition = (
  textarea: HTMLTextAreaElement,
  cursor: number,
): CompletionPosition => {
  const host =
    (textarea.closest('.webdata-editor-input') as HTMLElement | null) ||
    textarea.parentElement;
  const hostWidth = host?.clientWidth || textarea.clientWidth;
  const viewportWidth = window.innerWidth || hostWidth;
  const viewportHeight = window.innerHeight || textarea.clientHeight;
  const popupWidth = Math.max(330, Math.min(460, hostWidth - 16));
  const styles = window.getComputedStyle(textarea);
  const lineHeight = Number.parseFloat(styles.lineHeight) || 22;
  const textareaRect = textarea.getBoundingClientRect();
  const mirror = document.createElement('div');
  const copiedStyles = [
    'box-sizing',
    'border',
    'font-family',
    'font-size',
    'font-weight',
    'letter-spacing',
    'line-height',
    'padding',
    'tab-size',
    'text-indent',
    'text-transform',
    'white-space',
    'word-break',
    'word-spacing',
    'word-wrap',
    'overflow-wrap',
  ];
  copiedStyles.forEach((property) => {
    mirror.style.setProperty(property, styles.getPropertyValue(property));
  });
  mirror.style.position = 'fixed';
  mirror.style.visibility = 'hidden';
  mirror.style.pointerEvents = 'none';
  mirror.style.overflow = 'hidden';
  mirror.style.left = `${textareaRect.left}px`;
  mirror.style.top = `${textareaRect.top}px`;
  mirror.style.width = `${textarea.clientWidth}px`;
  mirror.style.whiteSpace = 'pre-wrap';
  const safeCursor = Math.max(0, Math.min(cursor, textarea.value.length));
  mirror.textContent = textarea.value.slice(0, safeCursor);

  const marker = document.createElement('span');
  marker.textContent = '\u200b';
  marker.style.display = 'inline-block';
  marker.style.width = '0';
  marker.style.height = `${lineHeight}px`;
  marker.style.verticalAlign = 'top';
  mirror.appendChild(marker);
  document.body.appendChild(mirror);

  const markerRect = marker.getBoundingClientRect();
  const caretTop = markerRect.top - textarea.scrollTop;
  const caretBottom = markerRect.bottom - textarea.scrollTop;
  const rawLeft = markerRect.left - textarea.scrollLeft + 6;
  const rawTop = caretBottom + 8;
  const left = clampNumber(
    rawLeft,
    8,
    Math.max(8, viewportWidth - popupWidth - 8),
  );
  const availableBelow = Math.max(0, viewportHeight - rawTop - 8);
  const availableAbove = Math.max(0, caretTop - 8);
  const openBelow = availableBelow >= 120 || availableBelow >= availableAbove;
  const popupHeight = Math.min(
    292,
    Math.max(120, openBelow ? availableBelow : availableAbove),
  );
  const top = openBelow
    ? clampNumber(rawTop, 8, Math.max(8, viewportHeight - popupHeight - 8))
    : clampNumber(
        caretTop - popupHeight - 8,
        8,
        Math.max(8, viewportHeight - popupHeight - 8),
      );
  document.body.removeChild(mirror);

  return {
    left,
    top,
    width: popupWidth,
    maxHeight: popupHeight,
  };
};

export const clampNumber = (value: number, min: number, max: number) =>
  Math.min(Math.max(value, min), Math.max(min, max));

export const buildCompletionContext = (
  value: string,
  cursor: number,
): CompletionContext => {
  const safeCursor = Math.max(0, Math.min(cursor, value.length));
  let start = safeCursor;
  while (start > 0 && !isCompletionBoundary(value[start - 1])) {
    start -= 1;
  }
  let end = safeCursor;
  while (end < value.length && !isCompletionBoundary(value[end])) {
    end += 1;
  }
  const statementStart = currentCompletionStatementStart(value, safeCursor);
  const before = value.slice(0, start).trimEnd();
  const previousMatch = before.match(/([A-Za-z_$][\w$]*|\*)$/);
  const previousWord = (previousMatch?.[1] || '').toLowerCase();
  const statementPrefix = value.slice(statementStart, start);
  return {
    start,
    end,
    query: value.slice(start, safeCursor),
    previousWord,
    statementPrefix,
    intent: inferSqlCompletionIntent(value, start, safeCursor, previousWord),
  };
};

export const isCompletionBoundary = (char: string) =>
  /\s/.test(char) ||
  ['(', ')', '{', '}', '[', ']', ',', ';', ':', '"', "'", '`'].includes(char);

export const currentCompletionStatementStart = (
  value: string,
  cursor: number,
) => Math.max(value.lastIndexOf(';', Math.max(0, cursor - 1)) + 1, 0);

export const inferSqlCompletionIntent = (
  value: string,
  tokenStart: number,
  cursor: number,
  previousWord: string,
): CompletionIntent => {
  const statementStart = currentCompletionStatementStart(value, cursor);
  const beforeToken = value.slice(statementStart, tokenStart);
  if (!beforeToken.trim()) return 'command';
  if (previousWord === '*') return 'from-keyword';
  if (
    ['from', 'join', 'update', 'into', 'table', 'describe', 'desc'].includes(
      previousWord,
    )
  ) {
    return 'table';
  }
  if (['use', 'database'].includes(previousWord)) return 'database';
  if (['schema', 'search_path'].includes(previousWord)) return 'schema';
  if (
    [
      'select',
      'distinct',
      'where',
      'by',
      'on',
      'set',
      'order',
      'group',
      'having',
    ].includes(previousWord)
  ) {
    return previousWord === 'select' || previousWord === 'distinct'
      ? 'select-list'
      : 'field';
  }

  const tokens = tokenizeSqlCompletionPrefix(
    value.slice(statementStart, tokenStart),
  );
  if (!tokens.length) return 'command';
  const fromIndex = tokens.lastIndexOf('from');
  const selectIndex = tokens.lastIndexOf('select');
  const lastClause = findLastSqlClause(tokens);
  if (selectIndex >= 0 && fromIndex < selectIndex) return 'select-list';
  if (lastClause === 'from' || lastClause === 'join') {
    const clauseIndex = tokens.lastIndexOf(lastClause);
    return tokens.length > clauseIndex + 1 ? 'sql-clause' : 'table';
  }
  if (['limit', 'offset'].includes(lastClause)) return 'any';
  if (
    ['where', 'on', 'set', 'group', 'order', 'by', 'having'].includes(
      lastClause,
    )
  ) {
    return 'field';
  }
  return 'command';
};

export const completionIntentForProtocol = (
  protocol: string,
  context: CompletionContext,
) => {
  if (isSQLProtocol(protocol)) return context.intent;
  if (protocol === 'redis') return inferRedisCompletionIntent(context);
  if (protocol === 'mongodb') return inferMongoCompletionIntent(context);
  return context.intent;
};

export const inferRedisCompletionIntent = (
  context: CompletionContext,
): CompletionIntent => {
  const tokens = tokenizeRedisCompletionPrefix(context.statementPrefix);
  if (!tokens.length) return 'command';
  const command = tokens[0];
  const keyCommands = new Set([
    'get',
    'set',
    'del',
    'exists',
    'expire',
    'ttl',
    'type',
    'mget',
    'mset',
    'incr',
    'decr',
    'hgetall',
    'hget',
    'hmget',
    'hset',
    'hdel',
    'hexists',
    'lrange',
    'lpush',
    'rpush',
    'lpop',
    'rpop',
    'llen',
    'zrange',
    'zrevrange',
    'zadd',
    'zrem',
    'zscore',
    'zcard',
    'sadd',
    'srem',
    'smembers',
    'sismember',
    'scard',
  ]);
  if (keyCommands.has(command) && tokens.length <= 1) return 'redis-key';
  return 'redis-argument';
};

export const tokenizeRedisCompletionPrefix = (value: string) =>
  value
    .trim()
    .split(/\s+/)
    .map((token) => token.toLowerCase())
    .filter(Boolean);

export const inferMongoCompletionIntent = (
  context: CompletionContext,
): CompletionIntent => {
  const prefix = context.statementPrefix;
  if (!prefix.trim() || /[{,]\s*"?[$A-Za-z_]*$/.test(prefix)) {
    return 'command';
  }
  if (
    /"?(find|aggregate|insert|update|delete)"?\s*:\s*"?[A-Za-z0-9_.$-]*$/i.test(
      prefix,
    )
  ) {
    return 'mongo-collection';
  }
  if (/pipeline"?\s*:\s*\[[\s\S]*[{,]\s*"?\$?[A-Za-z_]*$/i.test(prefix)) {
    return 'mongo-stage';
  }
  if (['use', 'database'].includes(context.previousWord)) return 'database';
  return 'any';
};

export const tokenizeSqlCompletionPrefix = (value: string) =>
  value
    .replace(/`[^`]*`|"[^"]*"|'[^']*'/g, ' ')
    .match(/[A-Za-z_$][\w$]*|\*/g)
    ?.map((token) => token.toLowerCase()) || [];

export const findLastSqlClause = (tokens: string[]) => {
  const clauses = new Set([
    'select',
    'from',
    'join',
    'where',
    'on',
    'set',
    'group',
    'order',
    'by',
    'having',
    'limit',
    'offset',
    'union',
    'except',
    'intersect',
    'update',
    'into',
    'values',
    'use',
  ]);
  for (let index = tokens.length - 1; index >= 0; index -= 1) {
    if (clauses.has(tokens[index])) return tokens[index];
  }
  return '';
};

export const buildCompletionItems = (
  protocol: string | undefined,
  metadata: API.WebDataMetadataNode[],
  selectedNode: API.WebDataMetadataNode | undefined,
  context: CompletionContext,
  tr: (zh: string, en: string) => string,
): CompletionItem[] => {
  const normalizedProtocol = String(protocol || '').toLowerCase();
  const query = normalizeCompletionQuery(context.query);
  const items: CompletionItem[] = [];

  completionKeywordItems(normalizedProtocol, tr, context).forEach((item) =>
    items.push(item),
  );
  metadataCompletionItems(
    normalizedProtocol,
    metadata,
    selectedNode,
    context,
  ).forEach((item) => items.push(item));

  const deduped = new Map<string, CompletionItem>();
  items.forEach((item) => {
    const key = `${item.type}:${item.insertText}`;
    const existing = deduped.get(key);
    if (!existing || item.priority > existing.priority) {
      deduped.set(key, item);
    }
  });

  return Array.from(deduped.values())
    .filter((item) => completionItemMatches(item, query))
    .sort((a, b) => {
      const aScore = query ? completionItemMatchScore(a, query) : 0;
      const bScore = query ? completionItemMatchScore(b, query) : 0;
      if (aScore !== bScore) return aScore - bScore;
      if (a.priority !== b.priority) return b.priority - a.priority;
      return a.label.localeCompare(b.label);
    });
};

export const shouldChainCompletionAfterApply = (
  protocol: string | undefined,
  value: string,
  cursor: number,
) => {
  const normalizedProtocol = String(protocol || '').toLowerCase();
  const context = buildCompletionContext(value, cursor);
  const intent = completionIntentForProtocol(normalizedProtocol, context);
  if (isSQLProtocol(normalizedProtocol)) {
    return [
      'select-list',
      'from-keyword',
      'sql-clause',
      'table',
      'field',
      'database',
      'schema',
    ].includes(intent);
  }
  if (normalizedProtocol === 'redis') {
    return ['redis-key', 'redis-argument'].includes(intent);
  }
  if (normalizedProtocol === 'mongodb') {
    return ['mongo-collection', 'mongo-stage', 'database'].includes(intent);
  }
  return false;
};

export const shouldOpenCompletionForContext = (
  protocol: string | undefined,
  context: CompletionContext,
) => {
  if (context.query.trim()) return false;
  const previousWord = context.previousWord;
  const normalizedProtocol = String(protocol || '').toLowerCase();
  if (isSQLProtocol(normalizedProtocol)) {
    return (
      context.intent !== 'command' || ['select', '*'].includes(previousWord)
    );
  }
  const intent = completionIntentForProtocol(normalizedProtocol, context);
  if (normalizedProtocol === 'redis') {
    return ['redis-key', 'redis-argument'].includes(intent);
  }
  if (normalizedProtocol === 'mongodb') {
    return ['mongo-collection', 'mongo-stage'].includes(intent);
  }
  return false;
};

export const normalizeCompletionQuery = (value: string) =>
  value
    .trim()
    .replace(/^[`"']+|[`"']+$/g, '')
    .toLowerCase();

export const completionItemMatches = (item: CompletionItem, query: string) => {
  if (!query) return true;
  return Number.isFinite(completionItemMatchScore(item, query));
};

export const completionItemMatchScore = (
  item: CompletionItem,
  query: string,
) => {
  if (!query) return 0;
  const primaryCandidates = [item.label, item.insertText]
    .map(normalizeCompletionQuery)
    .filter(Boolean);
  const detailCandidates = [item.detail || '']
    .map(normalizeCompletionQuery)
    .filter(Boolean);
  const acronymCandidates = [completionAcronym(item.label)]
    .map(normalizeCompletionQuery)
    .filter(Boolean);
  let best = Number.POSITIVE_INFINITY;
  primaryCandidates.forEach((candidate) => {
    if (candidate === query) best = Math.min(best, 0);
    else if (candidate.startsWith(query)) {
      best = Math.min(best, 100 + candidate.length / 100);
    } else if (candidate.includes(query)) {
      best = Math.min(best, 200 + candidate.indexOf(query) / 100);
    }
    const fuzzyScore = fuzzyCompletionScore(query, candidate);
    if (Number.isFinite(fuzzyScore)) {
      best = Math.min(best, 400 + fuzzyScore / 100);
    }
  });
  detailCandidates.forEach((candidate) => {
    if (candidate.startsWith(query)) {
      best = Math.min(best, 250 + candidate.length / 100);
    } else if (candidate.includes(query)) {
      best = Math.min(best, 300 + candidate.indexOf(query) / 100);
    }
  });
  acronymCandidates.forEach((candidate) => {
    if (candidate === query) best = Math.min(best, 350);
    else if (candidate.startsWith(query)) best = Math.min(best, 360);
  });
  return best;
};

export const completionAcronym = (value: string) =>
  value
    .split(/[\s._-]+/)
    .map((part) => part[0] || '')
    .join('');

export const fuzzyCompletionScore = (query: string, candidate: string) => {
  let position = -1;
  let spread = 0;
  for (const char of query) {
    const next = candidate.indexOf(char, position + 1);
    if (next < 0) return Number.POSITIVE_INFINITY;
    if (position >= 0) spread += next - position - 1;
    position = next;
  }
  return spread + candidate.length / 20;
};

export const sqlKeywordInsertText = (label: string) => {
  if (label === ',') return ', ';
  if (label === '*') return '* ';
  return `${label} `;
};

export const completionKeywordItems = (
  protocol: string,
  tr: (zh: string, en: string) => string,
  context: CompletionContext,
): CompletionItem[] => {
  const keyword = (label: string, insertText = label, priority = 20) => ({
    key: `keyword-${label}`,
    label,
    insertText,
    detail: tr('命令', 'Command'),
    type: 'keyword' as const,
    priority,
  });
  const keywordItems = (
    labels: string[],
    priorityBase = 90,
  ): CompletionItem[] =>
    labels.map((label, index) =>
      keyword(label, sqlKeywordInsertText(label), priorityBase - index),
    );
  const snippet = (
    label: string,
    insertText: string,
    priority: number,
    detail = tr('片段', 'Snippet'),
  ): CompletionItem => ({
    key: `snippet-${label}`,
    label,
    insertText,
    detail,
    type: 'snippet',
    priority,
  });
  if (protocol === 'redis') {
    const intent = completionIntentForProtocol(protocol, context);
    if (intent === 'redis-argument') {
      return redisArgumentKeywords.map((label, index) =>
        keyword(label, `${label} `, 80 - index),
      );
    }
    if (intent !== 'command') return [];
    return redisCommandKeywords.map((label, index) =>
      keyword(label, label === 'SCAN' ? 'SCAN 0 ' : `${label} `, 100 - index),
    );
  }
  if (protocol === 'mongodb') {
    const intent = completionIntentForProtocol(protocol, context);
    if (intent === 'mongo-stage') {
      return mongoStageSnippets.map((item) =>
        snippet(item.label, JSON.stringify(item.value, null, 2), item.priority),
      );
    }
    if (intent !== 'command') return [];
    return mongoCommandSnippets.map((item) =>
      snippet(item.label, JSON.stringify(item.value, null, 2), item.priority),
    );
  }
  const intent = completionIntentForProtocol(protocol, context);
  const sqlItems: CompletionItem[] = sqlCommandKeywords.map((label, index) =>
    keyword(label, `${label} `, 90 - index),
  );
  if (intent === 'from-keyword') {
    return keywordItems(sqlSelectFollowKeywords);
  }
  if (intent === 'sql-clause') {
    return keywordItems(sqlClauseKeywords);
  }
  if (intent === 'select-list') {
    return [
      ...sqlSelectSnippets.map((item, index) =>
        snippet(
          item.label,
          item.insertText,
          100 - index,
          tr(item.detailZh, item.detailEn),
        ),
      ),
      ...keywordItems(sqlSelectListKeywords, 88),
    ];
  }
  if (intent === 'field') {
    return [
      ...keywordItems(sqlFieldKeywords, 84),
      ...keywordItems(sqlExpressionTailKeywords, 82),
    ];
  }
  return intent === 'command' ? sqlItems : [];
};

export const metadataCompletionItems = (
  protocol: string,
  metadata: API.WebDataMetadataNode[],
  selectedNode: API.WebDataMetadataNode | undefined,
  context: CompletionContext,
): CompletionItem[] => {
  const items: CompletionItem[] = [];
  const selectedScope = selectedMetadataScope(selectedNode);
  const intent = completionIntentForProtocol(protocol, context);
  const tableContext = intent === 'table';
  const fieldContext = ['select-list', 'field'].includes(intent);
  const databaseContext = intent === 'database';
  const schemaContext = intent === 'schema';

  const walk = (nodes: API.WebDataMetadataNode[]) => {
    nodes.forEach((node) => {
      if (
        protocol === 'redis' &&
        intent === 'redis-key' &&
        node.type === 'key'
      ) {
        const key = node.meta?.key || node.title;
        items.push({
          key: `redis-key-${node.key}`,
          label: key,
          insertText: quoteRedisArg(key),
          detail: node.value || 'key',
          type: 'key',
          priority: 36,
        });
      }
      if (
        protocol === 'mongodb' &&
        databaseContext &&
        node.type === 'database'
      ) {
        const name = node.meta?.database || node.title;
        items.push({
          key: `mongo-db-${node.key}`,
          label: name,
          insertText: name,
          detail: 'database',
          type: 'object',
          priority: metadataScopeBoost(node, selectedScope) ? 38 : 24,
        });
      }
      if (
        protocol === 'mongodb' &&
        intent === 'mongo-collection' &&
        node.type === 'collection'
      ) {
        const name = node.meta?.name || node.title;
        items.push({
          key: `mongo-coll-${node.key}`,
          label: name,
          insertText: name,
          detail: node.meta?.database
            ? `${node.meta.database} · collection`
            : 'collection',
          type: 'object',
          priority: 36 + metadataScopeBoost(node, selectedScope),
        });
      }
      if (isSQLProtocol(protocol)) {
        if (databaseContext && node.type === 'database') {
          const name = node.meta?.database || node.title;
          items.push({
            key: `sql-db-${node.key}`,
            label: name,
            insertText: name,
            detail: 'database',
            type: 'object',
            priority:
              (databaseContext ? 50 : 22) +
              metadataScopeBoost(node, selectedScope),
          });
        }
        if (schemaContext && node.type === 'schema') {
          const name = node.meta?.schema || node.title;
          items.push({
            key: `sql-schema-${node.key}`,
            label: name,
            insertText: name,
            detail: 'schema',
            type: 'object',
            priority:
              (schemaContext ? 50 : 24) +
              metadataScopeBoost(node, selectedScope),
          });
        }
        if (tableContext && node.type === 'table') {
          const insertText = sqlObjectCompletionInsert(protocol, node);
          items.push({
            key: `sql-table-${node.key}`,
            label: insertText,
            insertText: `${insertText} `,
            detail: node.meta?.database
              ? `${node.meta.database} · table`
              : node.meta?.schema
              ? `${node.meta.schema} · table`
              : 'table',
            type: 'object',
            priority:
              (tableContext ? 48 : 32) +
              metadataScopeBoost(node, selectedScope),
          });
        }
        if (fieldContext && node.type === 'column') {
          const column = node.title;
          const table = node.meta?.name;
          items.push({
            key: `sql-column-${node.key}`,
            label: column,
            insertText: `${column} `,
            detail: table ? `${table} · column` : 'column',
            type: 'field',
            priority:
              (fieldContext ? 46 : 24) +
              metadataScopeBoost(node, selectedScope),
          });
        }
      }
      if (node.children?.length) walk(node.children);
    });
  };
  walk(metadata);
  return items;
};

export const selectedMetadataScope = (node?: API.WebDataMetadataNode) => ({
  database:
    node?.meta?.database || (node?.type === 'database' ? node.title : ''),
  schema: node?.meta?.schema || (node?.type === 'schema' ? node.title : ''),
  object:
    node?.meta?.name ||
    (['table', 'collection'].includes(node?.type || '')
      ? node?.title || ''
      : ''),
});

export const metadataScopeBoost = (
  node: API.WebDataMetadataNode,
  selectedScope: ReturnType<typeof selectedMetadataScope>,
) => {
  let boost = 0;
  if (
    selectedScope.database &&
    (node.meta?.database || (node.type === 'database' ? node.title : '')) ===
      selectedScope.database
  ) {
    boost += 10;
  }
  if (
    selectedScope.schema &&
    (node.meta?.schema || (node.type === 'schema' ? node.title : '')) ===
      selectedScope.schema
  ) {
    boost += 10;
  }
  if (selectedScope.object && node.meta?.name === selectedScope.object) {
    boost += 12;
  }
  return boost;
};

export const sqlObjectCompletionInsert = (
  protocol: string,
  node: API.WebDataMetadataNode,
) => {
  const name = node.meta?.name || node.title;
  if (protocol === 'postgresql' && node.meta?.schema) {
    return `${node.meta.schema}.${name}`;
  }
  if (protocol === 'mysql' && node.meta?.database) {
    return `${node.meta.database}.${name}`;
  }
  return name;
};

export const completionTypeIcon = (item: CompletionItem) => {
  if (item.type === 'object') {
    return /database/i.test(item.detail || '') ? (
      <DatabaseOutlined />
    ) : (
      <TableOutlined />
    );
  }
  if (item.type === 'key') return <KeyOutlined />;
  if (item.type === 'field') return <span>T</span>;
  if (item.type === 'snippet') return <span>fx</span>;
  return <FileTextOutlined />;
};
