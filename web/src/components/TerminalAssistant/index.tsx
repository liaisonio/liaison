import { useEffect, useRef, useState } from 'react';
import { Sparkles, ArrowUpRight, X } from 'lucide-react';
import { request } from '@/api/client';
import { useI18n } from '@/i18n';
import './index.less';

const printable = (character: string) => {
  const code = character.codePointAt(0)!;
  return code >= 32 && (code < 127 || code > 159);
};

export default function TerminalAssistant({ handleId, onInsert }: { handleId: string; onInsert: (text: string) => void }) {
  const { tr } = useI18n();
  const [text, setText] = useState('');
  const [ghost, setGhost] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [enabled, setEnabled] = useState(true);
  const input = useRef<HTMLInputElement>(null);
  const revision = useRef(0);
  const editor = useRef(crypto.randomUUID());
  const controller = useRef<AbortController>();
  const cancel = () => { revision.current++; controller.current?.abort(); setGhost(''); setBusy(false); };
  useEffect(() => {
    const editorID = editor.current;
    return () => {
      controller.current?.abort();
      void request('/api/v1/assistance/suggestions', { method: 'DELETE', data: { handle_id: handleId, editor_id: editorID }, skipErrorHandler: true }).catch(() => undefined);
    };
  }, [handleId]);
  const suggest = async () => {
    cancel();
    if (!enabled || !text.trim()) return;
    // 第一版只做行尾补全，避免覆盖用户已有字符或选择内容。
    input.current?.focus();
    input.current?.setSelectionRange(text.length, text.length);
    const current = revision.current;
    const abort = new AbortController(); controller.current = abort;
    setBusy(true); setError('');
    try {
      const response = await request<API.Response<{text: string; revision: number}>>('/api/v1/assistance/suggestions', {
        method: 'POST', signal: abort.signal, skipErrorHandler: true,
        data: { handle_id: handleId, editor_id: editor.current, revision: current, text, cursor: new TextEncoder().encode(text).length },
      });
      if (abort.signal.aborted || current !== revision.current) return;
      const candidate = response.data?.text || '';
      if (![...candidate].every(printable)) throw new Error(tr('候选包含不可用字符', 'Suggestion contains unsupported characters'));
      setGhost(candidate);
      if (!candidate) setError(tr('暂无合适建议', 'No suggestion available'));
    } catch (e: any) {
      if (!abort.signal.aborted) setError(e?.response?.status === 503 ? tr('请先配置并启用 Agent 模型', 'Configure and enable an Agent model first') : tr('补全暂不可用，请重试', 'Suggestion unavailable. Please retry.'));
    } finally { if (current === revision.current) setBusy(false); }
  };
  const accept = () => { if (!ghost) return; setText(text + ghost); cancel(); };
  return <section className="terminal-assistant" aria-label={tr('命令辅助', 'Command assistance')}>
    <div className="terminal-assistant-meta"><span><Sparkles size={13} />{tr('命令辅助', 'Command assistance')}</span>
      <button type="button" onClick={() => { cancel(); setEnabled(!enabled); }}>{enabled ? tr('手动提示', 'Manual suggestions') : tr('提示已关闭', 'Suggestions off')}</button>
      <small>{tr('仅发送本栏草稿，不读取终端输出', 'Only this draft is shared, not terminal output')}</small>
    </div>
    <div className="terminal-assistant-row">
      <span className="terminal-assistant-prompt">$</span>
      <div className="terminal-assistant-editor">
        <div className="terminal-assistant-preview" aria-hidden="true"><span>{text}</span><i>{ghost}</i></div>
        <input ref={input} aria-label={tr('命令草稿', 'Command draft')} value={text} spellCheck={false} autoComplete="off"
          placeholder={tr('输入命令草稿，点击提示补全…', 'Draft a command, then request a suggestion…')}
          onChange={e => { cancel(); setText([...e.target.value].filter(printable).join('')); setError(''); }}
          onScroll={e => { const preview = e.currentTarget.previousElementSibling; if (preview) preview.scrollLeft = e.currentTarget.scrollLeft; }}
          onClick={cancel}
          onKeyDown={e => {
            if (e.nativeEvent.isComposing) return;
            if (e.key === 'Tab' && ghost) { e.preventDefault(); accept(); }
            else if (e.key === 'Escape') cancel();
            else if (e.key.startsWith('Arrow') || e.key === 'Home' || e.key === 'End') cancel();
            else if (e.key === 'Enter') e.preventDefault();
          }} />
      </div>
      {ghost && <button type="button" onClick={accept} title={tr('接受补全，不执行', 'Accept without executing')}>Tab</button>}
      {ghost && <button type="button" onClick={cancel} aria-label={tr('忽略建议', 'Dismiss suggestion')}><X size={14}/></button>}
      <button type="button" disabled={!enabled || !text.trim() || busy} onClick={() => void suggest()}><Sparkles size={14}/>{busy ? tr('生成中…', 'Suggesting…') : tr('提示', 'Suggest')}</button>
      <button type="button" disabled={!text.trim() || busy || Boolean(ghost)} onClick={() => { onInsert(text); setText(''); cancel(); }} title={tr('仅在 Shell 提示符处使用；不会发送回车', 'Use only at a shell prompt; no Enter is sent')}><ArrowUpRight size={14}/>{tr('填入终端', 'Insert into terminal')}</button>
    </div>
    <div className="terminal-assistant-help" role="status">{error || (ghost ? tr('Tab 接受 · Esc 忽略 · 不自动执行', 'Tab to accept · Esc to dismiss · Never auto-executes') : tr('确认终端位于 Shell 提示符后再填入；回车执行仍由你控制。', 'Insert only at a shell prompt. You control Enter in the terminal.'))}</div>
  </section>;
}
