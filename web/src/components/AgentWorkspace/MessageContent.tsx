import { useI18n } from '@/i18n';
import { Check, ChevronRight, CircleAlert, Copy, Terminal } from 'lucide-react';
import { Fragment, useState } from 'react';

// Render a small, safe Markdown subset as React nodes. Never interpret model HTML.
function inline(text: string) {
  return text.split(/(`[^`\n]+`|\*\*[^*\n]+\*\*)/g).map((part, index) =>
    part.startsWith('`') && part.endsWith('`') ? <code key={index}>{part.slice(1, -1)}</code>
      : part.startsWith('**') && part.endsWith('**') ? <strong key={index}>{part.slice(2, -2)}</strong>
        : <Fragment key={index}>{part}</Fragment>);
}

export function CodeBlock({ text, language = '' }: { text: string; language?: string }) {
  const { tr } = useI18n();
  const [copied, setCopied] = useState(false);
  return <div className="agent-code">
    <div className="agent-code-toolbar"><span>{language || tr('输出', 'Output')}</span><button type="button" aria-label={tr('复制', 'Copy')} onClick={() => {
      void navigator.clipboard.writeText(text).then(() => setCopied(true)).catch(() => setCopied(false));
    }}>{copied ? <Check size={13} /> : <Copy size={13} />}</button></div>
    <pre><code>{text}</code></pre>
  </div>;
}

export function MessageContent({ text }: { text: string }) {
  const blocks = text.split(/```/g);
  return <div className="agent-markdown">{blocks.map((block, index) => {
    if (index % 2) {
      const newline = block.indexOf('\n');
      return <CodeBlock key={index} language={newline >= 0 ? block.slice(0, newline).trim() : ''} text={(newline >= 0 ? block.slice(newline + 1) : block).replace(/\n$/, '')} />;
    }
    return block.split(/\n\s*\n/).filter(part => part.trim()).map((part, partIndex) => {
      const lines = part.trim().split('\n');
      if (lines.every(line => /^\s*[-*]\s/.test(line))) return <ul key={`${index}-${partIndex}`}>{lines.map((line, i) => <li key={i}>{inline(line.replace(/^\s*[-*]\s+/, ''))}</li>)}</ul>;
      if (lines.every(line => /^\s*\d+[.)]\s/.test(line))) return <ol key={`${index}-${partIndex}`}>{lines.map((line, i) => <li key={i}>{inline(line.replace(/^\s*\d+[.)]\s+/, ''))}</li>)}</ol>;
      return <p key={`${index}-${partIndex}`}>{lines.map((line, i) => <Fragment key={i}>{i > 0 && <br />}{/^#{1,6}\s/.test(line) ? <strong>{inline(line.replace(/^#{1,6}\s+/, ''))}</strong> : inline(line)}</Fragment>)}</p>;
    });
  })}</div>;
}

export function ToolMessage({ name, content, command }: { name: string; content: string; command?: string }) {
  const { tr } = useI18n();
  let output = content;
  let failed = false;
  let timedOut = false;
  let rows: Record<string, unknown>[] | undefined;
  let columns: string[] = [];
  try {
    const result = JSON.parse(content);
    const value = result.Content ?? result.content ?? result;
    if (Array.isArray(value?.rows) && value.rows.every((row: unknown) => row && typeof row === 'object' && !Array.isArray(row))) {
      rows = value.rows;
      columns = Array.isArray(value.columns) ? value.columns.filter((column: unknown) => typeof column === 'string') : Object.keys(rows?.[0] || {});
    }
    timedOut = value?.code === 'tool_timeout';
    failed = Boolean(result.IsError ?? result.is_error) || (typeof value?.exit_code === 'number' && value.exit_code !== 0);
    output = typeof value === 'string' ? value : value?.output ?? JSON.stringify(value, null, 2);
  } catch { /* Older or plain-text tool results remain readable. */ }
  const labels: Record<string, string> = {
    'terminal.read': tr('读取终端', 'Read terminal'), 'terminal.execute': tr('执行命令', 'Run command'),
    'core.tool_search': tr('查找工具', 'Find tools'), 'core.tool_describe': tr('加载工具', 'Load tools'),
    'data.schema': tr('查看数据结构', 'Inspect schema'), 'data.query': tr('执行查询', 'Run query'),
  };
  return <details className={`agent-tool-result${failed ? ' is-error' : ''}`}>
    <summary><ChevronRight size={13} />{failed ? <CircleAlert size={14} /> : <Terminal size={14} />}<span>{labels[name] || name}</span><small>{timedOut ? tr('超时', 'Timed out') : failed ? tr('未完成', 'Failed') : tr('完成', 'Done')}</small></summary>
    {command && <code className="agent-tool-command">{command}</code>}
    {timedOut ? <p>{tr('命令超过执行时限，远端完成状态未知。不会自动重试，请缩小范围后再确认执行。', 'The command exceeded its time limit. Remote completion is unknown. It will not retry automatically; narrow the scope before approving another command.')}</p> : rows?.length && !failed ? <>
      <div className="agent-data-table"><table><thead><tr>{columns.map(column => <th key={column}>{column}</th>)}</tr></thead><tbody>{rows.slice(0,25).map((row,index)=><tr key={index}>{columns.map(column=><td key={column}>{typeof row[column] === 'object' ? JSON.stringify(row[column]) : String(row[column] ?? 'NULL')}</td>)}</tr>)}</tbody></table></div>
      {rows.length > 25 && <p>{tr('预览前 25 行，完整返回值见下方', 'Previewing 25 rows. Full returned result below.')}</p>}
      <details><summary>{tr('原始结果', 'Raw result')}</summary><CodeBlock text={output} /></details>
    </> : <CodeBlock text={output || tr('无输出', 'No output')} />}
  </details>;
}
