import { formatCellValue } from './metadata';

export const buildExplainStatement = (statement: string) => {
  const trimmed = stripTrailingSemicolons(statement.trim());
  if (/^explain\b/i.test(trimmed)) return `${trimmed};`;
  return `EXPLAIN ${trimmed};`;
};

export const stripTrailingSemicolons = (value: string) =>
  value.trim().replace(/;+$/g, '').trim();

export const rowsToCSV = (rows: Record<string, any>[], columns: string[]) => {
  const lines = [
    columns.map(csvEscape).join(','),
    ...rows.map((row) =>
      columns.map((column) => csvEscape(row[column])).join(','),
    ),
  ];
  return lines.join('\n');
};

export const csvEscape = (value: any) => {
  const text = formatCellValue(value);
  if (/[",\n\r]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`;
  }
  return text;
};

export const downloadTextFile = (
  filename: string,
  content: string,
  type: string,
) => {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

export const isDangerousStatement = (protocol?: string, statement?: string) => {
  const value = String(statement || '')
    .trim()
    .toLowerCase();
  const first = value.split(/\s+/)[0];
  if (protocol === 'redis') {
    return [
      'del',
      'unlink',
      'flushdb',
      'flushall',
      'config',
      'acl',
      'shutdown',
    ].includes(first);
  }
  if (protocol === 'mongodb') {
    return /\b(drop|delete|update|insert|create|renamecollection)\b/.test(
      value,
    );
  }
  return ![
    'select',
    'show',
    'describe',
    'desc',
    'explain',
    'with',
    'values',
  ].includes(first);
};
