import { Tooltip } from '@/components/ui/complex';
import {
  Braces,
  Columns3,
  Database,
  KeyRound,
  Layers3,
  List,
  Table2,
  Type,
  Waves,
} from 'lucide-react';
import type { ReactNode } from 'react';
import { isSQLProtocol } from './protocol';
import type { MetadataSummary, MetadataTreeNode } from './types';

export const filterMetadataNodes = (
  nodes: API.WebDataMetadataNode[],
  query: string,
): API.WebDataMetadataNode[] => {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return nodes;
  return nodes
    .map((node) => {
      const children = node.children
        ? filterMetadataNodes(node.children, normalized)
        : undefined;
      const metaText = Object.values(node.meta || {}).join(' ');
      const matched = `${node.title} ${node.value || ''} ${metaText}`
        .toLowerCase()
        .includes(normalized);
      if (!matched && !children?.length) return undefined;
      return { ...node, children };
    })
    .filter(Boolean) as API.WebDataMetadataNode[];
};

export const summarizeMetadata = (
  nodes: API.WebDataMetadataNode[],
): MetadataSummary => {
  const summary: MetadataSummary = {
    total: 0,
    databases: 0,
    schemas: 0,
    tables: 0,
    columns: 0,
    keys: 0,
    collections: 0,
  };
  const walk = (items: API.WebDataMetadataNode[]) => {
    items.forEach((node) => {
      summary.total += 1;
      if (node.type === 'database') summary.databases += 1;
      if (node.type === 'schema') summary.schemas += 1;
      if (node.type === 'table') summary.tables += 1;
      if (node.type === 'column') summary.columns += 1;
      if (node.type === 'key') summary.keys += 1;
      if (node.type === 'collection') summary.collections += 1;
      if (node.children?.length) walk(node.children);
    });
  };
  walk(nodes);
  return summary;
};

export const buildRedisMetadataView = (
  nodes: API.WebDataMetadataNode[],
  database = 0,
): API.WebDataMetadataNode[] => {
  const keys: API.WebDataMetadataNode[] = [];
  const collectKeys = (items: API.WebDataMetadataNode[]) => {
    items.forEach((node) => {
      if (node.type === 'key') keys.push(node);
      if (node.children?.length) collectKeys(node.children);
    });
  };
  collectKeys(nodes);

  const grouped = new Map<string, API.WebDataMetadataNode[]>();
  keys.forEach((node) => {
    const type = String(node.value || 'key').split(/\s+/)[0].toLowerCase();
    const label = {
      string: 'String',
      hash: 'Hash',
      list: 'List',
      set: 'Set',
      zset: 'Sorted Set',
      stream: 'Stream',
    }[type] || 'Other';
    const ttl = String(node.value || '').match(/ttl=(.+)$/)?.[1];
    const item = { ...node, value: ttl ? `TTL ${ttl}` : undefined };
    grouped.set(label, [...(grouped.get(label) || []), item]);
  });

  const typeOrder = ['String', 'Hash', 'List', 'Set', 'Sorted Set', 'Stream', 'Other'];
  const children = typeOrder
    .filter((label) => grouped.has(label))
    .map((label) => ({
      key: `redis-db-${database}-${label.toLowerCase().replace(/\s+/g, '-')}`,
      title: label,
      type: 'group',
      children: (grouped.get(label) || []).sort((a, b) =>
        a.title.localeCompare(b.title),
      ),
    }));

  return [{
    key: `redis-db-${database}`,
    title: `DB ${database}`,
    type: 'database',
    meta: { database: String(database) },
    children,
  }];
};

export const summarizeResult = (result?: API.WebDataExecuteResult) => {
  const rows = result?.rows?.length || 0;
  const columns = result?.columns?.length
    ? result.columns.length
    : unionRowKeys(result?.rows || []).length;
  return { rows, columns };
};

export const mapMetadataTree = (
  nodes: API.WebDataMetadataNode[],
): MetadataTreeNode[] =>
  nodes.map((node) => ({
    key: node.key,
    title: renderNodeTitle(node),
    source: node,
    children: node.children ? mapMetadataTree(node.children) : undefined,
    isLeaf: node.has_children
      ? false
      : node.children
        ? node.children.length === 0
        : true,
  }));

export const replaceMetadataChildren = (
  nodes: API.WebDataMetadataNode[],
  key: string,
  children: API.WebDataMetadataNode[],
): API.WebDataMetadataNode[] =>
  nodes.map((node) => {
    if (node.key === key) {
      return { ...node, children, has_children: children.length > 0 };
    }
    if (!node.children?.length) return node;
    return {
      ...node,
      children: replaceMetadataChildren(node.children, key, children),
    };
  });

export const buildMetadataChildParams = (
  protocol: string,
  node: API.WebDataMetadataNode,
): API.WebDataMetadataParams | undefined => {
  if (node.type !== 'table') return undefined;
  if (protocol === 'mysql' || protocol === 'mariadb') {
    return {
      type: 'table',
      database: node.meta?.database,
      name: node.meta?.name || node.title,
    };
  }
  if (protocol === 'postgresql' || protocol === 'sqlserver') {
    return {
      type: 'table',
      schema: node.meta?.schema,
      name: node.meta?.name || node.title,
    };
  }
  return undefined;
};

export const renderNodeTitle = (node: API.WebDataMetadataNode) => {
  const redisGroupIcon = node.type === 'group'
    ? ({
        String: Type,
        Hash: Braces,
        List,
        Set: Layers3,
        'Sorted Set': Layers3,
        Stream: Waves,
      }[node.title])
    : undefined;
  const Icon = redisGroupIcon || (node.type === 'database'
    ? Database
    : node.type === 'table'
      ? Table2
      : node.type === 'collection'
        ? Braces
        : node.type === 'key'
          ? KeyRound
          : Columns3);
  const distinctValue =
    node.value && node.value.trim().toLowerCase() !== node.title.trim().toLowerCase()
      ? node.value
      : '';
  const label = `${node.title}${
    distinctValue ? ` (${distinctValue})` : ''
  }`;
  return (
    <Tooltip title={label}>
      <span className="webdata-tree-title"><Icon size={13} />{label}</span>
    </Tooltip>
  );
};

export const isInspectableNode = (node: API.WebDataMetadataNode) =>
  ['table', 'collection', 'key', 'column'].includes(node.type);

export const buildObjectParams = (
  protocol: string,
  node: API.WebDataMetadataNode,
): API.WebDataObjectParams | undefined => {
  if (
    isSQLProtocol(protocol) &&
    node.type === 'column'
  ) {
    return {
      type: 'table',
      database: node.meta?.database,
      schema: node.meta?.schema,
      name: node.meta?.name,
    };
  }
  if (isSQLProtocol(protocol)) {
    if (node.type !== 'table') return undefined;
    return {
      type: 'table',
      database: node.meta?.database,
      schema: node.meta?.schema,
      name: node.meta?.name || node.title,
    };
  }
  if (protocol === 'redis' && node.type === 'key') {
    return { type: 'key', key: node.meta?.key || node.title };
  }
  if (protocol === 'mongodb' && node.type === 'collection') {
    return {
      type: 'collection',
      database: node.meta?.database,
      name: node.meta?.name || node.title,
    };
  }
  return undefined;
};

export const buildResultColumns = (
  result: API.WebDataExecuteResult,
  renderCell?: (
    value: any,
    column: string,
    row: Record<string, any>,
  ) => ReactNode,
) => {
  const columns = result.columns?.length
    ? result.columns
    : unionRowKeys(result.rows || []);
  return columns.map((column) => ({
    title: column,
    dataIndex: column,
    key: column,
    ellipsis: true,
    render: (value: any, row: Record<string, any>) =>
      renderCell ? (
        renderCell(value, column, row)
      ) : (
        <Tooltip title={formatCellValue(value)}>
          <span className="webdata-cell">{formatCellValue(value)}</span>
        </Tooltip>
      ),
  }));
};

export const buildGenericColumns = (rows: Record<string, any>[]) =>
  unionRowKeys(rows).map((column) => ({
    title: column,
    dataIndex: column,
    key: column,
    ellipsis: true,
    render: (value: any) => (
      <Tooltip title={formatCellValue(value)}>
        <span className="webdata-cell">{formatCellValue(value)}</span>
      </Tooltip>
    ),
  }));

export const unionRowKeys = (rows: Record<string, any>[]) => {
  const seen = new Set<string>();
  rows.forEach((row) => {
    Object.keys(row || {}).forEach((key) => seen.add(key));
  });
  return Array.from(seen);
};

export const formatCellValue = (value: any) => {
  if (value === null || value === undefined) return 'NULL';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
};

export const editableCellValue = (value: any) => {
  if (value === null || value === undefined) return '';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
};

export const formatAuditTime = (value?: string) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};
