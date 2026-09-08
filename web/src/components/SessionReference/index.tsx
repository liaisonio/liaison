import { useEffect, useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { useI18n } from '@/i18n';
import './index.less';

// Connection handles are bearer credentials. Only expose their one-way digest.
export default function SessionReference({ id, handle, label }: { id?: string; handle?: string; label: string }) {
  const { tr } = useI18n();
  const [reference, setReference] = useState('');
  const [copied, setCopied] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);
  useEffect(() => {
    let active = true;
    setReference(id || ''); setCopied(false); setCopyFailed(false);
    if (!id && handle) void crypto.subtle.digest('SHA-256', new TextEncoder().encode(handle)).then(buffer => {
      if (active) setReference(`conn_${Array.from(new Uint8Array(buffer)).map(v => v.toString(16).padStart(2, '0')).join('').slice(0, 24)}`);
    }).catch(() => { /* Never fall back to displaying the bearer handle. */ });
    return () => { active = false; };
  }, [id, handle]);
  if (!reference) return null;
  return <span className="session-reference">
    <button type="button" title={`${label}: ${reference}`} aria-label={`${tr('复制', 'Copy')} ${label}: ${reference}`}
      onClick={() => { void navigator.clipboard.writeText(reference).then(() => { setCopied(true); setCopyFailed(false); }).catch(() => setCopyFailed(true)); }}>
      <span>{label}</span><code>{reference}</code>{copied ? <Check size={12} /> : <Copy size={12} />}
    </button>
    {copyFailed && <small>{tr('复制失败，请选中 ID 手动复制', 'Copy failed. Select the ID to copy manually.')}</small>}
  </span>;
}
