import { request } from '@/api/client';
import { Button, Field, Input } from '@/components/ui';
import { useI18n } from '@/i18n';
import { useFeature } from '@/store/permissions';
import { getToken } from '@/store/session';
import {
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  ChevronRight,
  Download,
  FileText,
  Folder,
  Info,
  Link as LinkIcon,
  RefreshCw,
  Search,
  Upload,
  X,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import './Files.less';

export type FileSession = {
  id: string;
  home: string;
  can_upload: boolean;
  expires_at: string;
};
type Session = FileSession;
type Entry = {
  name: string;
  size: number;
  mode: string;
  directory: boolean;
  symlink: boolean;
  modified_at: string;
};
const limit = 64 * 1024 * 1024;
const join = (dir: string, name: string) => `${dir.replace(/\/$/, '')}/${name}`;

export default function Files({
  proxyId,
  username: initialUsername,
  saved,
  onClose,
  standalone = false,
  initialSession,
  apiBase,
  closeSessionURL,
}: {
  proxyId: number;
  username: string;
  saved: boolean;
  onClose: () => void;
  standalone?: boolean;
  initialSession?: FileSession;
  apiBase?: string;
  closeSessionURL?: string;
}) {
  const { tr } = useI18n();
  const [username, setUsername] = useState(initialUsername);
  const uploadAllowed = useFeature('webssh.files.upload');
  const [session, setSession] = useState<Session | undefined>(initialSession);
  const sessionRef = useRef<Session | undefined>(initialSession);
  const [password, setPassword] = useState('');
  const [saveCredential, setSaveCredential] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const [directory, setDirectory] = useState('');
  const [draft, setDraft] = useState('');
  const [entries, setEntries] = useState<Entry[]>([]);
  const [selected, setSelected] = useState('');
  const [filter, setFilter] = useState('');
  const [editingPath, setEditingPath] = useState(false);
  const [infoOpen, setInfoOpen] = useState(false);
  const history = useRef<string[]>([]);
  const historyIndex = useRef(-1);
  const [tree, setTree] = useState<Record<string, string[]>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [treeErrors, setTreeErrors] = useState<Record<string, string>>({});
  const [treeLoading, setTreeLoading] = useState('');
  const [treeVisible, setTreeVisible] = useState(() => window.innerWidth > 600);
  const [treeWidth, setTreeWidth] = useState(220);
  const normalize = (p: string) => {
    const parts: string[] = [];
    for (const part of (p.startsWith('/')
      ? p
      : join(directory || '/', p)
    ).split('/')) {
      if (part === '..') parts.pop();
      else if (part && part !== '.') parts.push(part);
    }
    return '/' + parts.join('/');
  };
  const remember = (p: string, rows: Entry[], reveal: boolean) => {
    const ancestors = p
      .split('/')
      .filter(Boolean)
      .map((_, i, all) => '/' + all.slice(0, i + 1).join('/'));
    setTree((old) => {
      const next = {
        ...old,
        [p]: rows
          .filter((e) => e.directory && !e.symlink)
          .map((e) => join(p, e.name))
          .sort(),
      };
      let parent = '/';
      for (const child of ancestors) {
        next[parent] = Array.from(
          new Set([...(next[parent] || []), child]),
        ).sort();
        parent = child;
      }
      return next;
    });
    if (reveal)
      setExpanded((old) => ({
        ...old,
        '/': true,
        ...Object.fromEntries(ancestors.map((a) => [a, true])),
      }));
    setTreeErrors((old) => ({ ...old, [p]: '' }));
  };
  // Synthetic ancestor paths are not loaded directory listings.
  const loadedPaths = useRef(new Set<string>());
  const [preview, setPreview] = useState<{ name: string; text: string }>();
  const [progress, setProgress] = useState<number>();
  const alive = useRef(true);
  const controller = useRef<AbortController>();
  const transfer = useRef<XMLHttpRequest>();
  const uploadInput = useRef<HTMLInputElement>(null);
  const reason = (value: string) =>
    ({
      SFTP_UNAUTHORIZED: tr(
        '登录已失效，请重新登录。',
        'Your sign-in has expired.',
      ),
      SFTP_FORBIDDEN: tr(
        '没有文件操作权限，或当前访问已禁用。',
        'File permission denied or access disabled.',
      ),
      SFTP_NOT_FOUND: tr(
        '文件或会话不存在，请重新打开文件视图。',
        'File or session not found. Reopen the file view.',
      ),
      SFTP_BUSY: tr(
        '当前会话有操作未完成，请稍后重试。',
        'An operation is still running. Retry shortly.',
      ),
      SFTP_INVALID: tr(
        '路径无效，或不是普通文件。',
        'Invalid path or not a regular file.',
      ),
      SFTP_TOO_LARGE: tr(
        '超过大小限制：预览 256 KiB，传输 64 MiB。',
        'Size limit exceeded: preview 256 KiB, transfer 64 MiB.',
      ),
      SFTP_BINARY: tr(
        '此文件不是 UTF-8 文本，可下载查看。',
        'Not UTF-8 text. Download to view it.',
      ),
      SFTP_EXISTS: tr(
        '同名文件已存在，不会覆盖。请修改文件名后再上传。',
        'A file with this name exists and will not be overwritten.',
      ),
      SFTP_UPLOAD_UNSUPPORTED: tr(
        '服务器不支持安全的不覆盖上传。',
        'Server does not support safe no-overwrite uploads.',
      ),
      SFTP_TIMEOUT: tr(
        '操作已取消或超时，请重试。',
        'Operation canceled or timed out. Please retry.',
      ),
    }[value] ||
    tr(
      '文件操作失败。请检查目标服务、路径、文件大小与权限，或重新连接。',
      'File operation failed. Check the service, path, file size and permissions, or reconnect.',
    ));
  const showError = (error: unknown) => {
    if (!alive.current) return;
    const e = error as Error;
    if (e.name === 'AbortError') {
      setNotice(tr('已取消。', 'Canceled.'));
      return;
    }
    setNotice(reason(e.message));
  };
  const base = (s: Session) =>
    apiBase || `/api/v1/webssh/files/sessions/${encodeURIComponent(s.id)}`;
  const load = async (
    s: Session,
    p: string,
    signal: AbortSignal,
    index?: number,
  ) => {
    const res = await request<API.Response<Entry[]>>(`${base(s)}/list`, {
      params: { path: p },
      signal,
    });
    if (alive.current) {
      loadedPaths.current.add(p);
      remember(p, res.data || [], true);
      setEntries(
        (res.data || []).sort(
          (a, b) =>
            Number(b.directory) - Number(a.directory) ||
            a.name.localeCompare(b.name),
        ),
      );
      setDirectory(p);
      if (index !== undefined) historyIndex.current = index;
      else if (history.current[historyIndex.current] !== p) {
        history.current = history.current
          .slice(0, historyIndex.current + 1)
          .concat(p);
        historyIndex.current = history.current.length - 1;
      }
      setSelected('');
      setFilter('');
      setEditingPath(false);
      setDraft(p);
      setPreview(undefined);
    }
  };
  const run = async (fn: (signal: AbortSignal) => Promise<void>) => {
    if (controller.current) return;
    const c = new AbortController();
    controller.current = c;
    setBusy(true);
    setNotice('');
    try {
      await fn(c.signal);
    } catch (e) {
      showError(e);
    } finally {
      if (alive.current) {
        setBusy(false);
        setProgress(undefined);
      }
      controller.current = undefined;
      transfer.current = undefined;
    }
  };
  const connect = () =>
    run(async (signal) => {
      const res = await request<API.Response<Session>>(
        `/api/v1/webssh/proxies/${proxyId}/files/sessions`,
        {
          method: 'POST',
          data: {
            username,
            password,
            use_saved_credential: saved && !password,
            save_credential: saveCredential,
          },
          signal,
        },
      );
      if (!res.data) throw new Error('SFTP_FAILED');
      if (!alive.current) {
        void request(base(res.data), { method: 'DELETE' }).catch(
          () => undefined,
        );
        return;
      }
      sessionRef.current = res.data;
      setSession(res.data);
      setPassword('');
      await load(res.data, res.data.home, signal);
    });
  useEffect(() => {
    alive.current = true;
    const start =
      initialSession || saved
        ? window.setTimeout(() => {
            if (initialSession)
              void run((signal) =>
                load(initialSession, initialSession.home, signal),
              );
            else void connect();
          }, 0)
        : undefined;
    return () => {
      window.clearTimeout(start);
      alive.current = false;
      controller.current?.abort();
      transfer.current?.abort();
      const s = sessionRef.current;
      if (s)
        void request(closeSessionURL || base(s), {
          method: 'DELETE',
          skipErrorHandler: true,
        }).catch(() => undefined);
    };
  }, []);
  const move = (p: string, index?: number) => {
    if (session)
      void run(async (signal) => {
        const target = normalize(p);
        try {
          await load(session, target, signal, index);
        } catch (e) {
          if (alive.current)
            setTreeErrors((old) => ({
              ...old,
              [target]: reason((e as Error).message),
            }));
          throw e;
        }
      });
  };
  const expand = (p: string) => {
    if (busy || !session) return;
    if (expanded[p] && !treeErrors[p]) {
      setExpanded((old) => ({ ...old, [p]: false }));
      return;
    }
    setExpanded((old) => ({ ...old, [p]: true }));
    if (loadedPaths.current.has(p) && !treeErrors[p]) return;
    void run(async (signal) => {
      setTreeLoading(p);
      try {
        const res = await request<API.Response<Entry[]>>(
          `${base(session)}/list`,
          { params: { path: p }, signal },
        );
        if (alive.current) {
          loadedPaths.current.add(p);
          remember(p, res.data || [], false);
        }
      } catch (e) {
        if (alive.current)
          setTreeErrors((old) => ({
            ...old,
            [p]: reason((e as Error).message),
          }));
      } finally {
        if (alive.current) setTreeLoading('');
      }
    });
  };
  const directoryNode = (p: string, depth = 0): React.ReactNode => (
    <li key={p}>
      <div
        className={`webssh-directory-row${
          directory === p ? ' is-selected' : ''
        }`}
        style={{ paddingLeft: 8 + depth * 12 }}
      >
        <button
          type="button"
          className="webssh-directory-toggle"
          disabled={busy}
          aria-label={`${
            expanded[p] ? tr('收起', 'Collapse') : tr('展开', 'Expand')
          } ${p}`}
          aria-expanded={!!expanded[p]}
          onClick={() => expand(p)}
        >
          <ChevronRight
            size={14}
            style={{ transform: expanded[p] ? 'rotate(90deg)' : undefined }}
          />
        </button>
        <button
          type="button"
          className="webssh-directory-label"
          disabled={busy}
          aria-current={directory === p ? 'location' : undefined}
          title={p}
          onClick={() => move(p)}
        >
          <Folder size={15} />
          <span>{p === '/' ? '/' : p.split('/').pop()}</span>
        </button>
      </div>
      {treeLoading === p && (
        <div className="webssh-directory-message" role="status">
          {tr('加载中…', 'Loading…')}
        </div>
      )}
      {treeErrors[p] && (
        <div className="webssh-directory-message" role="alert">
          {treeErrors[p]}{' '}
          <Button variant="ghost" disabled={busy} onClick={() => expand(p)}>
            {tr('重试', 'Retry')}
          </Button>
        </div>
      )}
      {expanded[p] && (
        <ul>
          {(tree[p] || []).map((child) => directoryNode(child, depth + 1))}
        </ul>
      )}
    </li>
  );
  const rawTransfer = (method: 'GET' | 'PUT', url: string, body?: File) =>
    new Promise<Blob>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      transfer.current = xhr;
      xhr.open(method, url);
      xhr.responseType = 'blob';
      xhr.timeout = 5 * 60 * 1000;
      xhr.setRequestHeader('Authorization', `Bearer ${getToken() || ''}`);
      if (method === 'PUT')
        xhr.setRequestHeader('Content-Type', 'application/octet-stream');
      const update = (e: ProgressEvent) => {
        if (alive.current)
          setProgress(
            e.lengthComputable
              ? Math.round((e.loaded / e.total) * 100)
              : undefined,
          );
      };
      xhr.onprogress = update;
      xhr.upload.onprogress = update;
      xhr.onerror = () => reject(new Error('SFTP_FAILED'));
      xhr.ontimeout = () => reject(new Error('SFTP_TIMEOUT'));
      xhr.onabort = () => reject(new DOMException('Canceled', 'AbortError'));
      xhr.onload = async () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(xhr.response);
          return;
        }
        try {
          reject(
            new Error(
              JSON.parse(await (xhr.response as Blob).text()).reason ||
                'SFTP_FAILED',
            ),
          );
        } catch {
          reject(new Error('SFTP_FAILED'));
        }
      };
      xhr.send(body);
    });
  const download = (entry: Entry) =>
    void run(async () => {
      if (!session) return;
      const blob = await rawTransfer(
        'GET',
        `${base(session)}/download?path=${encodeURIComponent(
          join(directory, entry.name),
        )}`,
      );
      if (!alive.current) return;
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = entry.name;
      a.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    });
  const upload = (file?: File) => {
    if (!file || !session) return;
    if (file.size > limit) {
      setNotice(reason('SFTP_TOO_LARGE'));
      return;
    }
    void run(async (signal) => {
      await rawTransfer(
        'PUT',
        `${base(session)}/upload?path=${encodeURIComponent(
          join(directory, file.name),
        )}`,
        file,
      );
      await load(session, directory, signal);
      if (alive.current) setNotice(tr('上传完成。', 'Upload complete.'));
    });
  };
  const chosen = entries.find((e) => e.name === selected);
  const visibleEntries = entries.filter((e) =>
    e.name.toLocaleLowerCase().includes(filter.toLocaleLowerCase()),
  );
  const openEntry = (e: Entry) => {
    if (busy || e.symlink || !session) return;
    setSelected(e.name);
    if (e.directory) {
      move(join(directory, e.name));
      return;
    }
    void run(async (signal) => {
      const r = await request<API.Response<{ text: string }>>(
        `${base(session)}/preview`,
        { params: { path: join(directory, e.name) }, signal },
      );
      if (alive.current) setPreview({ name: e.name, text: r.data?.text || '' });
    });
  };
  const iconButton = (
    label: string,
    icon: React.ReactNode,
    action: () => void,
    disabled = false,
  ) => (
    <Button
      variant="ghost"
      aria-label={label}
      title={label}
      onClick={action}
      disabled={disabled}
    >
      {icon}
    </Button>
  );
  return (
    <section
      className="webssh-files"
      aria-label={tr('文件浏览器', 'File browser')}
      data-testid="webssh-files"
    >
      {!session ? (
        <div className="webssh-files-connect">
          <Folder size={28} />
          <strong>{tr('连接文件浏览器', 'Connect file browser')}</strong>
          {standalone && !saved ? (
            <Field label={tr('SSH 用户名', 'SSH username')} required>
              <Input
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </Field>
          ) : (
            <p>
              {tr('当前 SSH 账号', 'Current SSH account')} · {username}
            </p>
          )}
          {!saved && (
            <Field label={tr('SSH 密码', 'SSH password')}>
              <Input
                autoComplete="current-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !busy) void connect();
                }}
              />
            </Field>
          )}
          {standalone && !saved && (
            <label className="liaison-check-row">
              <input
                type="checkbox"
                checked={saveCredential}
                onChange={(e) => setSaveCredential(e.target.checked)}
              />
              {tr('保存登录信息', 'Save credentials')}
            </label>
          )}
          <div>
            <Button onClick={onClose}>
              {standalone
                ? tr('返回访问', 'Back to access')
                : tr('返回终端', 'Back to terminal')}
            </Button>{' '}
            <Button
              variant="primary"
              disabled={busy || !username.trim() || (!saved && !password)}
              onClick={() => void connect()}
            >
              {busy ? tr('连接中…', 'Connecting…') : tr('连接', 'Connect')}
            </Button>
          </div>
        </div>
      ) : (
        <>
          <div className="webssh-files-toolbar">
            <Button
              variant="ghost"
              aria-label={tr('目录', 'Directories')}
              title={tr('目录', 'Directories')}
              aria-expanded={treeVisible}
              onClick={() => setTreeVisible((v) => !v)}
            >
              <Folder size={16} />
            </Button>
            <div className="webssh-files-navigation">
              {iconButton(
                tr('后退', 'Back'),
                <ArrowLeft size={16} />,
                () =>
                  move(
                    history.current[historyIndex.current - 1],
                    historyIndex.current - 1,
                  ),
                busy || historyIndex.current < 1,
              )}
              {iconButton(
                tr('前进', 'Forward'),
                <ArrowRight size={16} />,
                () =>
                  move(
                    history.current[historyIndex.current + 1],
                    historyIndex.current + 1,
                  ),
                busy || historyIndex.current >= history.current.length - 1,
              )}
              {iconButton(
                tr('上级目录', 'Parent directory'),
                <ArrowUp size={16} />,
                () => move(directory.replace(/\/[^/]*\/?$/, '') || '/'),
                busy || directory === '/',
              )}
              {iconButton(
                tr('刷新', 'Refresh'),
                <RefreshCw size={15} />,
                () => move(directory),
                busy,
              )}
            </div>
            <div className="webssh-files-address">
              {editingPath ? (
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    if (!busy) move(draft);
                  }}
                >
                  <Input
                    autoFocus
                    aria-label={tr('远程路径', 'Remote path')}
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    disabled={busy}
                    onKeyDown={(e) => {
                      if (e.key === 'Escape') {
                        setDraft(directory);
                        setEditingPath(false);
                      }
                    }}
                  />
                </form>
              ) : (
                <nav
                  aria-label={tr('当前路径', 'Current path')}
                  className="webssh-files-breadcrumb"
                >
                  <button disabled={busy} onClick={() => move('/')}>
                    /
                  </button>
                  {directory
                    .split('/')
                    .filter(Boolean)
                    .map((part, i, all) => (
                      <span key={i}>
                        <ChevronRight size={12} />
                        <button
                          disabled={busy}
                          onClick={() =>
                            move('/' + all.slice(0, i + 1).join('/'))
                          }
                        >
                          {part}
                        </button>
                      </span>
                    ))}
                </nav>
              )}
              {iconButton(
                tr('编辑路径', 'Edit path'),
                <span className="webssh-files-path-icon">/</span>,
                () => {
                  setDraft(directory);
                  setEditingPath((v) => !v);
                },
                busy,
              )}
            </div>
            <label className="webssh-files-filter">
              <Search size={14} />
              <input
                aria-label={tr('筛选当前目录', 'Filter this folder')}
                placeholder={tr('筛选当前目录', 'Filter this folder')}
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
              />
            </label>
            {session.can_upload && uploadAllowed && (
              <>
                <input
                  ref={uploadInput}
                  type="file"
                  hidden
                  onChange={(e) => {
                    upload(e.target.files?.[0]);
                    e.target.value = '';
                  }}
                />
                <Button
                  disabled={busy}
                  onClick={() => uploadInput.current?.click()}
                >
                  <Upload size={15} />
                  {tr('上传', 'Upload')}
                </Button>
              </>
            )}
          </div>
          <div
            className={`webssh-files-layout${treeVisible ? ' has-tree' : ''}${
              preview ? ' has-preview' : ''
            }`}
            style={
              { '--file-tree-width': `${treeWidth}px` } as React.CSSProperties
            }
          >
            {treeVisible && (
              <>
                <nav
                  className="webssh-directory-tree"
                  aria-label={tr('目录树', 'Directory tree')}
                >
                  <div className="webssh-files-section-label">
                    {tr('目录', 'Directories')}
                  </div>
                  <ul>{directoryNode('/')}</ul>
                </nav>
                <div
                  className="webssh-directory-resize"
                  role="separator"
                  aria-label={tr('目录树宽度', 'Directory tree width')}
                  aria-orientation="vertical"
                  aria-valuemin={160}
                  aria-valuemax={340}
                  aria-valuenow={treeWidth}
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
                      e.preventDefault();
                      setTreeWidth((w) =>
                        Math.max(
                          160,
                          Math.min(340, w + (e.key === 'ArrowLeft' ? -16 : 16)),
                        ),
                      );
                    }
                  }}
                  onPointerDown={(e) =>
                    e.currentTarget.setPointerCapture(e.pointerId)
                  }
                  onPointerMove={(e) => {
                    if (e.currentTarget.hasPointerCapture(e.pointerId)) {
                      const bounds =
                        e.currentTarget.parentElement!.getBoundingClientRect();
                      setTreeWidth(
                        Math.max(160, Math.min(340, e.clientX - bounds.left)),
                      );
                    }
                  }}
                  onPointerUp={(e) =>
                    e.currentTarget.releasePointerCapture(e.pointerId)
                  }
                />
              </>
            )}
            <div className="webssh-files-content">
              <div className="webssh-files-selection">
                <span>{chosen ? chosen.name : tr('文件', 'Files')}</span>
                <div>
                  {chosen && !chosen.symlink && (
                    <>
                      {iconButton(
                        chosen.directory
                          ? tr('打开', 'Open')
                          : tr('预览', 'Preview'),
                        chosen.directory ? (
                          <Folder size={15} />
                        ) : (
                          <FileText size={15} />
                        ),
                        () => openEntry(chosen),
                        busy,
                      )}
                      {!chosen.directory &&
                        iconButton(
                          tr('下载', 'Download'),
                          <Download size={15} />,
                          () => download(chosen),
                          busy || chosen.size > limit,
                        )}
                    </>
                  )}
                </div>
              </div>
              <div className="webssh-files-list" aria-busy={busy}>
                <table>
                  <thead>
                    <tr>
                      <th>{tr('名称', 'Name')}</th>
                      <th>{tr('大小', 'Size')}</th>
                      <th>{tr('修改时间', 'Modified')}</th>
                      <th>{tr('权限', 'Permissions')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleEntries.map((e) => (
                      <tr
                        key={e.name}
                        className={selected === e.name ? 'is-selected' : ''}
                        onClick={() => setSelected(e.name)}
                        onDoubleClick={() => openEntry(e)}
                      >
                        <td>
                          <button
                            className="webssh-file-name"
                            aria-pressed={selected === e.name}
                            title={e.name}
                            onClick={() => setSelected(e.name)}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter') {
                                event.preventDefault();
                                openEntry(e);
                              }
                            }}
                          >
                            {e.symlink ? (
                              <LinkIcon size={16} />
                            ) : e.directory ? (
                              <Folder size={16} />
                            ) : (
                              <FileText size={16} />
                            )}
                            <span>{e.name}</span>
                          </button>
                        </td>
                        <td>
                          {e.directory
                            ? '—'
                            : e.size < 1024
                            ? `${e.size} B`
                            : `${(e.size / 1024).toFixed(1)} KiB`}
                        </td>
                        <td>
                          {e.modified_at
                            ? new Date(e.modified_at).toLocaleString(
                                tr('zh-CN', 'en-US'),
                                {
                                  year: 'numeric',
                                  month: '2-digit',
                                  day: '2-digit',
                                  hour: '2-digit',
                                  minute: '2-digit',
                                },
                              )
                            : '—'}
                        </td>
                        <td>
                          <code>{e.mode}</code>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {visibleEntries.length === 0 && !busy && (
                  <div className="webssh-files-empty">
                    <Folder size={30} />
                    <p>
                      {filter
                        ? tr('没有匹配的文件', 'No matching files')
                        : tr('目录为空', 'This folder is empty')}
                    </p>
                  </div>
                )}
              </div>
            </div>
            {preview && (
              <aside
                className="webssh-file-preview"
                aria-label={tr('文件预览', 'File preview')}
              >
                <header>
                  <strong title={preview.name}>{preview.name}</strong>
                  {iconButton(
                    tr('关闭预览', 'Close preview'),
                    <X size={16} />,
                    () => setPreview(undefined),
                  )}
                </header>
                <pre>{preview.text || tr('空文件', 'Empty file')}</pre>
              </aside>
            )}
          </div>
        </>
      )}
      <footer className="webssh-files-status">
        <span>
          {visibleEntries.length} {tr('项', 'items')}
          {selected ? ` · ${tr('已选 1 项', '1 selected')}` : ''}
        </span>
        <div className="webssh-files-status-message" role="status">
          {busy ? (
            <>
              <span>
                {progress === undefined
                  ? tr('处理中…', 'Working…')
                  : `${progress}%`}
              </span>
              <Button
                variant="ghost"
                onClick={() => {
                  controller.current?.abort();
                  transfer.current?.abort();
                }}
              >
                {tr('取消', 'Cancel')}
              </Button>
            </>
          ) : (
            <span title={notice}>{notice || username}</span>
          )}
        </div>
        {iconButton(tr('详情', 'Details'), <Info size={15} />, () =>
          setInfoOpen((v) => !v),
        )}
        {infoOpen && (
          <div className="webssh-files-details">
            <strong>{tr('文件浏览器', 'File browser')}</strong>
            <p>
              {tr(
                '双击打开 · Enter 打开 · 单击选择',
                'Double-click or Enter to open · Click to select',
              )}
            </p>
            <p>
              {tr(
                '预览 256 KiB · 传输 64 MiB · 不覆盖同名文件',
                'Preview 256 KiB · Transfers 64 MiB · No overwrites',
              )}
            </p>
            <code>{session?.id}</code>
          </div>
        )}
      </footer>
    </section>
  );
}
