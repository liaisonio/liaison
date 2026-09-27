import { FileLinkContext } from '@/components/AgentWorkspace/FileLinkContext';
import { MessageContent } from '@/components/AgentWorkspace/MessageContent';
import { Button, Modal, Notice } from '@/components/ui';
import { useI18n } from '@/i18n';
import { edgeAgent, type AgentSnapshot } from '@/services/edgeAgent';
import {
  ArrowLeft,
  Copy,
  Download,
  File,
  Folder,
  Maximize2,
  Minimize2,
} from 'lucide-react';
import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import { useAgentFiles, type AgentFile } from './Files';
import './code-preview.less';
import { documentLink, previewTarget } from './documentLink';

const PreviewCode = lazy(() => import('./PreviewCode'));
type Reply = AgentSnapshot & {
  files?: AgentFile[];
  file?: AgentFile;
  transfer_id?: string;
  file_data?: string;
  file_offset?: number;
  file_done?: boolean;
};
const previewLimit = 1024 * 1024;
export function CodePreview({
  href,
  project,
  edge,
  access,
  session,
  available,
  onClose,
}: {
  href: string;
  project: string;
  edge: number;
  access: string;
  session: string;
  available: boolean;
  onClose: () => void;
}) {
  const { tr } = useI18n(),
    [location, setLocation] = useState(href),
    [revision, setRevision] = useState(0);
  const [text, setText] = useState<string>(),
    [file, setFile] = useState<AgentFile>(),
    [entries, setEntries] = useState<AgentFile[]>(),
    [error, setError] = useState(''),
    [note, setNote] = useState(''),
    [loading, setLoading] = useState(true),
    [copied, setCopied] = useState(false);
  const [wide, setWide] = useState(false),
    [width, setWidth] = useState(640);
  const body = useRef<HTMLDivElement>(null),
    drag = useRef<{ x: number; width: number }>();
  const files = useAgentFiles(edge, access, session),
    target = previewTarget(location, project);
  const [history, setHistory] = useState<string[]>([]);
  const [anchorRevision, setAnchorRevision] = useState(0);
  const navigate = useCallback(
    (next: string) => {
      if (next === location) { setAnchorRevision(value => value + 1); return; }
      setHistory((previous) => [...previous, location]);
      setLocation(next);
    },
    [location],
  );
  const openDocumentLink = useCallback(
    (href: string) => {
      const next = documentLink(href, target?.path || '', project);
      if (next === undefined) {
        setNote(
          tr(
            '此链接不在当前项目内或格式不受支持。',
            'This link is outside the project or uses an unsupported format.',
          ),
        );
        return;
      }
      setNote('');
      navigate(next);
    },
    [target?.path, project, navigate, tr],
  );
  const markdown = /(?:\.md|\.markdown|(?:^|\/)readme)$/i.test(
    target?.path || '',
  );
  const [sourceView, setSourceView] = useState(Boolean(target?.line));
  useEffect(() => setSourceView(Boolean(target?.line)), [location]);
  const canRender = markdown && text !== undefined && text.length <= 200000;
  useEffect(() => {
    if (loading || sourceView || !canRender) return;
    const container = body.current?.querySelector<HTMLElement>(
      '.agent-document-preview',
    );
    if (!container) return;
    const heading = target?.anchor
      ? Array.from(container.querySelectorAll<HTMLElement>('[id]')).find(
          (el) => el.id === target.anchor,
        )
      : undefined;
    container.scrollTop = heading
      ? heading.getBoundingClientRect().top -
        container.getBoundingClientRect().top +
        container.scrollTop -
        16
      : 0;
    if (target?.anchor && !heading)
      setNote(
        tr(
          '未找到该标题，已显示文档开头。',
          'Heading not found. Showing the start of the document.',
        ),
      );
  }, [loading, sourceView, canRender, target?.anchor, text, anchorRevision]);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const modal = body.current?.closest<HTMLElement>('.liaison-modal');
    modal
      ?.querySelector<HTMLButtonElement>('header button')
      ?.focus({ preventScroll: true });
    const trap = (event: KeyboardEvent) => {
      if (event.key !== 'Tab' || !modal) return;
      const controls = Array.from(
        modal.querySelectorAll<HTMLElement>(
          'button:not(:disabled),a[href],summary,[tabindex="0"]',
        ),
      ).filter((el) => el.getClientRects().length);
      const first = controls[0],
        last = controls.at(-1);
      if (!first) return;
      if (
        event.shiftKey &&
        (document.activeElement === first ||
          !modal.contains(document.activeElement))
      ) {
        event.preventDefault();
        last?.focus({ preventScroll: true });
      } else if (
        !event.shiftKey &&
        (document.activeElement === last ||
          !modal.contains(document.activeElement))
      ) {
        event.preventDefault();
        first.focus({ preventScroll: true });
      }
    };
    document.addEventListener('keydown', trap);
    return () => {
      document.removeEventListener('keydown', trap);
      previous?.isConnected && previous.focus({ preventScroll: true });
    };
  }, []);
  useEffect(() => {
    const abort = new AbortController();
    let transfer = '';
    const request = async (
      action: string,
      file: Record<string, unknown>,
      signal?: AbortSignal,
    ) =>
      (await edgeAgent(
        edge,
        action,
        { access_id: access, session_id: session, file },
        signal,
      )) as Reply;
    setLoading(true);
    setText(undefined);
    setFile(undefined);
    setEntries(undefined);
    setError('');
    setNote('');
    setCopied(false);
    async function load() {
      try {
        if (!target) {
          setError(
            tr(
              '此路径不在当前项目内，无法预览。',
              'This path is outside the current project and cannot be previewed.',
            ),
          );
          return;
        }
        if (!available) {
          setError(
            tr(
              '文件暂不可读，请检查连接器状态，必要时恢复会话后重试。',
              'Files are unavailable. Check the connector and resume the conversation if needed.',
            ),
          );
          return;
        }
        let result = await request(
          'file_read',
          { path: target.path },
          abort.signal,
        );
        if (result.status !== 'ok') {
          if (result.status !== 'invalid_request')
            throw new Error('unavailable');
          result = await request(
            'file_list',
            { path: target.path },
            abort.signal,
          );
          if (result.status !== 'ok') throw new Error('unavailable');
          if (!abort.signal.aborted) {
            setEntries(result.files || []);
            if (result.truncated)
              setNote(
                tr('仅显示前 500 项。', 'Showing the first 500 entries.'),
              );
          }
          return;
        }
        const chunks: Uint8Array[] = [];
        let offset = 0;
        for (;;) {
          transfer = result.transfer_id || transfer;
          if (abort.signal.aborted) return;
          if (result.status !== 'ok' || !transfer)
            throw new Error('unavailable');
          if (result.file) {
            setFile(result.file);
            if (result.file.size > previewLimit) {
              setNote(
                tr(
                  '文件超过 1 MB，请下载查看。',
                  'This file exceeds 1 MB. Download to view it.',
                ),
              );
              return;
            }
          }
          const bytes = Uint8Array.from(atob(result.file_data || ''), (c) =>
            c.charCodeAt(0),
          );
          if (
            result.file_offset !== offset + bytes.length ||
            offset + bytes.length > previewLimit ||
            (!bytes.length && !result.file_done)
          )
            throw new Error('invalid response');
          chunks.push(bytes);
          offset += bytes.length;
          if (result.file_done) {
            transfer = '';
            break;
          }
          result = await request(
            'file_read',
            { transfer_id: transfer, offset },
            abort.signal,
          );
        }
        const bytes = new Uint8Array(offset);
        let cursor = 0;
        for (const part of chunks) {
          bytes.set(part, cursor);
          cursor += part.length;
        }
        let content: string;
        try {
          content = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
          if (content.includes('\0')) throw new Error('binary');
        } catch {
          setNote(
            tr(
              '此文件不是 UTF-8 文本，请下载查看。',
              'This is not a UTF-8 text file. Download to view it.',
            ),
          );
          return;
        }
        if (content.split('\n').length > 10000) {
          setNote(
            tr(
              '文件超过 10,000 行，请下载查看。',
              'This file exceeds 10,000 lines. Download to view it.',
            ),
          );
          return;
        }
        setText(content);
      } catch {
        if (!abort.signal.aborted)
          setError(
            tr(
              '无法读取文件。文件可能已移动、无访问权限或连接器已离线。',
              'Cannot read the file. It may have moved, access may be denied, or the connector may be offline.',
            ),
          );
      } finally {
        if (transfer)
          void request('file_cancel', { transfer_id: transfer }).catch(
            () => {},
          );
        if (!abort.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => abort.abort();
  }, [target?.path, project, edge, access, session, available, revision]);
  useEffect(() => {
    const el = body.current;
    if (!el || text === undefined || !target?.line) return;
    const scroll = () => {
      const source = el.querySelector<HTMLElement>('.agent-source-code'),
        line = el.querySelector<HTMLElement>('.agent-source-line.is-target');
      if (source && line)
        source.scrollTop +=
          line.getBoundingClientRect().top -
          source.getBoundingClientRect().top -
          source.clientHeight / 2;
    };
    const observer = new MutationObserver(scroll);
    observer.observe(el, { childList: true, subtree: true });
    scroll();
    return () => observer.disconnect();
  }, [text, location]);
  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
    } catch {
      setNote(
        tr(
          '复制失败，请手动选择文本。',
          'Copy failed. Select and copy the text manually.',
        ),
      );
    }
  };
  const changeWidth = (value: number) =>
    setWidth(Math.max(360, Math.min(window.innerWidth - 32, value)));
  return (
    <Modal
      open
      title={
        markdown
          ? tr('文件预览', 'File preview')
          : tr('代码预览', 'Code preview')
      }
      onClose={onClose}
      width={wide ? window.innerWidth : width}
      className={`agent-code-preview-panel${wide ? ' is-wide' : ''}`}
    >
      {!wide && (
        <div
          role="separator"
          tabIndex={0}
          aria-label={tr('调整代码预览宽度', 'Resize code preview')}
          aria-orientation="vertical"
          aria-valuenow={width}
          className="agent-code-preview-resize"
          onPointerDown={(event) => {
            if (event.button !== 0) return;
            event.preventDefault();
            event.currentTarget.setPointerCapture(event.pointerId);
            drag.current = { x: event.clientX, width };
          }}
          onPointerMove={(event) => {
            if (drag.current)
              changeWidth(drag.current.width + drag.current.x - event.clientX);
          }}
          onPointerUp={() => {
            drag.current = undefined;
          }}
          onPointerCancel={() => {
            drag.current = undefined;
          }}
          onLostPointerCapture={() => {
            drag.current = undefined;
          }}
          onKeyDown={(event) => {
            if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
              event.preventDefault();
              changeWidth(width + (event.key === 'ArrowLeft' ? 40 : -40));
            }
          }}
        />
      )}
      <div ref={body} className="agent-code-preview-body">
        <div className="agent-code-preview-toolbar">
          <code title={target?.path || location}>
            {target?.path || location}
          </code>
          <small>{tr('当前文件内容', 'Current file contents')}</small>
          <div>
            {!!history.length && (
              <Button
                aria-label={tr('返回上个文件', 'Back to previous file')}
                onClick={() => {
                  setNote('');
                  setLocation(history[history.length - 1]);
                  setHistory((previous) => previous.slice(0, -1));
                }}
              >
                <ArrowLeft size={14} />
              </Button>
            )}
            {canRender && (
              <>
                <Button
                  aria-pressed={!sourceView}
                  onClick={() => setSourceView(false)}
                >
                  {tr('预览', 'Preview')}
                </Button>
                <Button
                  aria-pressed={sourceView}
                  onClick={() => setSourceView(true)}
                >
                  {tr('源码', 'Source')}
                </Button>
              </>
            )}
            <Button
              aria-label={tr('复制路径', 'Copy path')}
              title={tr('复制路径', 'Copy path')}
              onClick={() => void copy(target?.path || location)}
            >
              <Copy size={14} />
            </Button>
            {text !== undefined && (
              <Button onClick={() => void copy(text)}>
                {markdown
                  ? tr('复制源码', 'Copy source')
                  : tr('复制代码', 'Copy code')}
              </Button>
            )}
            {file && (
              <Button
                disabled={files.busy}
                onClick={() => void files.download(file)}
              >
                <Download size={14} />
                {tr('下载', 'Download')}
              </Button>
            )}
            <Button
              aria-label={
                wide
                  ? tr('还原预览', 'Restore preview')
                  : tr('放大预览', 'Expand preview')
              }
              title={
                wide
                  ? tr('还原预览', 'Restore preview')
                  : tr('放大预览', 'Expand preview')
              }
              onClick={() => setWide((v) => !v)}
            >
              {wide ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
            </Button>
          </div>
        </div>
        {copied && <small role="status">{tr('已复制', 'Copied')}</small>}
        {files.feedback}
        {loading ? (
          <p role="status">{tr('正在读取文件…', 'Reading file…')}</p>
        ) : error ? (
          <Notice tone="danger">
            {error}
            <Button onClick={() => setRevision((v) => v + 1)}>
              {tr('重试', 'Retry')}
            </Button>
          </Notice>
        ) : (
          <>
            {note && <Notice>{note}</Notice>}
            {markdown && text !== undefined && !canRender && (
              <Notice>
                {tr(
                  '文档较大，已切换为源码显示。',
                  'This document is large and is displayed as source.',
                )}
              </Notice>
            )}
            {text !== undefined && (
              <>
                {canRender && !sourceView ? (
                  <div className="agent-document-preview" key={location}>
                    <FileLinkContext.Provider value={openDocumentLink}>
                      <MessageContent text={text} onAnchor={openDocumentLink} />
                    </FileLinkContext.Provider>
                  </div>
                ) : (
                  <Suspense
                    fallback={
                      <p role="status">
                        {tr('正在加载代码…', 'Loading code…')}
                      </p>
                    }
                  >
                    <PreviewCode
                      text={text}
                      path={target?.path || ''}
                      line={target?.line}
                      endLine={target?.endLine}
                    />
                  </Suspense>
                )}
                {target?.line && target.line > text.split('\n').length && (
                  <Notice>
                    {tr(
                      '指定行已超出当前文件范围。',
                      'The requested line is beyond the current file.',
                    )}
                  </Notice>
                )}
              </>
            )}
            {entries && (
              <div className="edge-agent-directory-list">
                {target?.path !== '.' && (
                  <Button
                    onClick={() =>
                      navigate(
                        target!.path.split('/').slice(0, -1).join('/') || '.',
                      )
                    }
                  >
                    {tr('上一级', 'Parent folder')}
                  </Button>
                )}
                {!entries.length && (
                  <p>{tr('此目录为空', 'This folder is empty')}</p>
                )}
                {entries.map((entry) => (
                  <Button
                    variant="ghost"
                    key={entry.path}
                    onClick={() => navigate(entry.path)}
                  >
                    {entry.directory ? (
                      <Folder size={15} />
                    ) : (
                      <File size={15} />
                    )}
                    <span>{entry.name}</span>
                  </Button>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </Modal>
  );
}
