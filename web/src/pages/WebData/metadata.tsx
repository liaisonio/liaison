import { Tooltip } from 'antd';
import type { ReactNode } from 'react';
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
  }));

export const renderNodeTitle = (node: API.WebDataMetadataNode) => {
  const icon =
    node.type === 'table' || node.type === 'collection'
      ? '▦'
      : node.type === 'column'
      ? '·'
      : node.type === 'key'
      ? '◆'
      : '';
  const label = `${icon ? `${icon} ` : ''}${node.title}${
    node.value ? ` (${node.value})` : ''
  }`;
  return (
    <Tooltip title={label}>
      <span className="webdata-tree-title">{label}</span>
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
    (protocol === 'mysql' || protocol === 'postgresql') &&
    node.type === 'column'
  ) {
    return {
      type: 'table',
      database: node.meta?.database,
      schema: node.meta?.schema,
      name: node.meta?.name,
    };
  }
  if (protocol === 'mysql' || protocol === 'postgresql') {
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
