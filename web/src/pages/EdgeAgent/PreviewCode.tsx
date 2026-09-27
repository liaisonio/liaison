import { common, createLowlight } from 'lowlight';
import { useMemo, type ReactNode } from 'react';

const highlighter = createLowlight(common);
type Node = {
  type: string;
  value?: string;
  properties?: { className?: unknown };
  children?: Node[];
};
export default function PreviewCode({
  text,
  path,
  line,
  endLine,
}: {
  text: string;
  path: string;
  line?: number;
  endLine?: number;
}) {
  const rows = useMemo(() => {
    const extension = path.split('.').pop()?.toLowerCase() || '';
    const language =
      (
        {
          tsx: 'typescript',
          jsx: 'javascript',
          mjs: 'javascript',
          cjs: 'javascript',
          vue: 'xml',
          html: 'xml',
          yml: 'yaml',
          sh: 'bash',
          md: 'markdown',
        } as Record<string, string>
      )[extension] || extension;
    const lines: ReactNode[][] = [[]];
    let key = 0;
    function walk(node: Node, classes: string[] = []) {
      if (node.type === 'text') {
        (node.value || '').split('\n').forEach((part, index) => {
          if (index) lines.push([]);
          if (part)
            lines.at(-1)!.push(
              <span key={key++} className={classes.join(' ')}>
                {part}
              </span>,
            );
        });
      } else
        for (const child of node.children || [])
          walk(child, [
            ...classes,
            ...(Array.isArray(node.properties?.className)
              ? (node.properties.className as string[])
              : []),
          ]);
    }
    // Bound highlighting work; larger previews remain readable as plain text.
    try {
      walk(
        text.length <= 200_000 && highlighter.registered(language)
          ? highlighter.highlight(language, text)
          : { type: 'text', value: text },
      );
    } catch {
      lines.length = 1;
      lines[0] = [];
      walk({ type: 'text', value: text });
    }
    return lines;
  }, [text, path]);
  return (
    <pre className="agent-source-code" tabIndex={0}>
      <code>
        {rows.map((content, index) => (
          <span
            key={index}
            data-line={index + 1}
            className={`agent-source-line${
              line && index + 1 >= line && index + 1 <= (endLine || line)
                ? ' is-target'
                : ''
            }`}
          >
            <span className="agent-source-number" aria-hidden="true">
              {index + 1}
            </span>
            <span>{content.length ? content : '\n'}</span>
          </span>
        ))}
      </code>
    </pre>
  );
}
