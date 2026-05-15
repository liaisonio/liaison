import type { DataNode } from 'antd/es/tree';
import type { Key } from 'react';

export type MetadataTreeNode = DataNode & {
  source?: API.WebDataMetadataNode;
  children?: MetadataTreeNode[];
};

export type QuickAction =
  | 'sql-insert'
  | 'redis-set'
  | 'redis-member-add'
  | 'redis-member-update'
  | 'mongo-insert'
  | 'mongo-update'
  | undefined;
export type SQLRowAction = 'sql-update' | undefined;

export type WebDataColumnInfo = {
  name: string;
  type: string;
  generated: boolean;
  raw: Record<string, any>;
};

export type InlineCellEdit = {
  scope: string;
  rowKey: Key;
  column: string;
  value: string;
  row: Record<string, any>;
  mode: 'sql' | 'mongo';
};

export type CompletionItem = {
  key: string;
  label: string;
  insertText: string;
  detail?: string;
  type: 'keyword' | 'object' | 'field' | 'snippet' | 'key';
  priority: number;
};

export type CompletionIntent =
  | 'command'
  | 'select-list'
  | 'from-keyword'
  | 'sql-clause'
  | 'table'
  | 'field'
  | 'database'
  | 'schema'
  | 'redis-key'
  | 'redis-argument'
  | 'mongo-collection'
  | 'mongo-stage'
  | 'any';

export type CompletionContext = {
  start: number;
  end: number;
  query: string;
  previousWord: string;
  statementPrefix: string;
  intent: CompletionIntent;
};

export type CompletionPosition = {
  left: number;
  top: number;
  width: number;
  maxHeight: number;
};

export type ObjectFilterCondition = {
  id: string;
  field?: string;
  operator: string;
  value?: string;
};

export type ObjectFilterState = {
  conditions: ObjectFilterCondition[];
  limit: number;
};

export type LoadObjectDetailOptions = {
  updateStatement?: boolean;
  updateResult?: boolean;
  clearResult?: boolean;
};

export type ExecuteStatementOptions = {
  refreshMetadata?: boolean;
};

export type ExecuteGeneratedCommandOptions = ExecuteStatementOptions & {
  updateStatement?: boolean;
  afterSuccess?: () => void | Promise<void>;
};

export type ObjectFilterFieldKind =
  | 'text'
  | 'number'
  | 'date'
  | 'boolean'
  | 'object'
  | 'unknown';

export type ObjectFilterFieldInfo = {
  name: string;
  kind: ObjectFilterFieldKind;
  typeLabel?: string;
};

export type MetadataSummary = {
  total: number;
  databases: number;
  schemas: number;
  tables: number;
  columns: number;
  keys: number;
  collections: number;
};
