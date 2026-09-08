import { useEffect, useRef, useState } from 'react';
import { Sparkles, X } from 'lucide-react';
import { request } from '@/api/client';
import { useI18n } from '@/i18n';
import './index.less';

// Separate, ephemeral completion context. Never sends chat history or executes a query.
export default function DataAssistance({ handleId, text, onApply }: { handleId: string; text: string; onApply: (text: string) => void }) {
  const { tr } = useI18n();
  const [candidate, setCandidate] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const revision = useRef(0);
  const editor = useRef(crypto.randomUUID());
  const controller = useRef<AbortController>();
  const latest = useRef(text);
  latest.current = text;
  const cancel = () => { revision.current++; controller.current?.abort(); setCandidate(''); setBusy(false); };
  useEffect(() => { cancel(); setError(''); }, [text, handleId]);
  useEffect(() => {
    const id = editor.current;
    return () => {
      controller.current?.abort();
      void request('/api/v1/assistance/suggestions', { method: 'DELETE', data: { handle_id: handleId, editor_id: id }, skipErrorHandler: true }).catch(() => undefined);
    };
  }, [handleId]);
  const suggest = async () => {
    cancel();
    if (!text.trim()) return;
    const snapshot = text;
    const current = revision.current;
    const abort = new AbortController(); controller.current = abort;
    setBusy(true); setError('');
    try {
      const response = await request<API.Response<{ text: string; revision: number }>>('/api/v1/assistance/suggestions', {
        method: 'POST', signal: abort.signal, skipErrorHandler: true,
        data: { handle_id: handleId, editor_id: editor.current, revision: current, text: snapshot, cursor: new TextEncoder().encode(snapshot).length },
      });
      if (abort.signal.aborted || current !== revision.current || latest.current !== snapshot) return;
      const value = response.data?.text || '';
      if ([...value].some(character => { const code = character.codePointAt(0)!; return (code < 32 && code !== 9 && code !== 10) || (code >= 127 && code <= 159); })) throw new Error('Invalid suggestion');
      setCandidate(value);
      if (!value) setError(tr('暂无合适建议', 'No suggestion available'));
    } catch {
      if (!abort.signal.aborted) setError(tr('补全暂不可用，请检查模型配置或重试', 'Suggestion unavailable. Check model settings or retry.'));
    } finally { if (current === revision.current) setBusy(false); }
  };
  return <section className="data-assistance" aria-label={tr('查询辅助', 'Query assistance')}>
    <div className="data-assistance-toolbar">
      <button type="button" disabled={busy || !text.trim()} onClick={() => void suggest()}><Sparkles size={13} />{busy ? tr('生成中…', 'Suggesting…') : tr('补全草稿', 'Complete draft')}</button>
      <small>{tr('仅发送编辑器草稿，不自动执行', 'Shares only the editor draft. Never auto-executes.')}</small>
      {(busy || candidate) && <button type="button" aria-label={tr('忽略建议', 'Dismiss suggestion')} onClick={cancel}><X size={13} /></button>}
    </div>
    {candidate && <div className="data-assistance-candidate"><pre>{candidate}</pre><button type="button" onClick={() => { onApply(text + candidate); cancel(); }}>{tr('应用到编辑器', 'Apply to editor')}</button></div>}
    {error && <small role="status">{error}</small>}
  </section>;
}
