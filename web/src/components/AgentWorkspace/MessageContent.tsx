import { useI18n } from '@/i18n';
import { Check, ChevronRight, CircleAlert, Copy, Terminal } from 'lucide-react';
import { Children, isValidElement, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

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
  return <div className="agent-markdown"><ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml components={{
    pre({children}) {
      const child = Children.toArray(children)[0];
      if (!isValidElement<{children?: React.ReactNode; className?: string}>(child)) return <pre>{children}</pre>;
      return <CodeBlock language={child.props.className?.replace(/^language-/, '')} text={String(child.props.children ?? '').replace(/\n$/, '')}/>;
    },
    table({children}) { return <div className="agent-markdown-table"><table>{children}</table></div>; },
    a({href, children}) { return href ? <a href={href} target="_blank" rel="noopener noreferrer">{children}</a> : <span>{children}</span>; },
    // Model-supplied images must not initiate tracking requests.
    img({alt}) { return <span>{alt}</span>; },
  }}>{text}</ReactMarkdown></div>;
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
