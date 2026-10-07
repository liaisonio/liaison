import { useEffect, useState } from 'react';
import { ClipboardPaste, Copy, Keyboard, ChevronsUpDown, ArrowUp, ArrowDown, ArrowLeft, ArrowRight } from 'lucide-react';
import ActionMenu from '@/components/ui/ActionMenu';
import { Button, Field, Modal, Notice } from '@/components/ui';
import { useI18n } from '@/i18n';

export default function MobileTerminalControls({ connected, selection, ctrl, onCtrl, onKey, onKeyboard, onPaste }: {
  connected: boolean; selection: string; ctrl: boolean; onCtrl: () => void;
  onKey: (data: string) => void; onKeyboard: () => void; onPaste: (data: string) => void;
}) {
  const { tr } = useI18n();
  const [pasteOpen, setPasteOpen] = useState(false);
  const [text, setText] = useState('');
  const [notice, setNotice] = useState('');
  useEffect(() => { if (!connected) { setPasteOpen(false); setText(''); } }, [connected]);
  useEffect(() => { if (!notice) return; const timer = window.setTimeout(() => setNotice(''), 3000); return () => clearTimeout(timer); }, [notice]);
  const close = () => { setPasteOpen(false); setText(''); };
  return <>
    <div className="webssh-mobile-keys" role="toolbar" aria-label={tr('终端快捷键', 'Terminal shortcuts')}>
      <Button className="webssh-keyboard-toggle" variant="ghost" disabled={!connected} aria-label={tr('键盘', 'Keyboard')} title={tr('显示或收起键盘', 'Show or hide keyboard')} onPointerDown={e => e.preventDefault()} onClick={onKeyboard}><Keyboard size={19}/></Button>
      <div className="webssh-key-scroll">
      <Button disabled={!connected} aria-pressed={ctrl} onPointerDown={e => e.preventDefault()} onClick={onCtrl}>Ctrl</Button>
      {[
        ['Esc', '\x1b', tr('退出键', 'Escape')], ['Tab', '\t', tr('制表键', 'Tab')],
        ['↑', '\x1b[A', tr('向上', 'Up')], ['↓', '\x1b[B', tr('向下', 'Down')],
        ['←', '\x1b[D', tr('向左', 'Left')], ['→', '\x1b[C', tr('向右', 'Right')],
      ].map(([label, data, title]) => <Button key={label} disabled={!connected} aria-label={title} onPointerDown={e => e.preventDefault()} onClick={() => onKey(data)}>{label === '↑' ? <ArrowUp size={16}/> : label === '↓' ? <ArrowDown size={16}/> : label === '←' ? <ArrowLeft size={16}/> : label === '→' ? <ArrowRight size={16}/> : label}</Button>)}
      <Button disabled={!selection} aria-label={tr('复制', 'Copy')} title={tr('复制选中内容', 'Copy selection')} onClick={async () => { try { await navigator.clipboard.writeText(selection); setNotice(tr('已复制选中内容', 'Selection copied')); } catch { setNotice(tr('复制失败，请使用系统复制菜单。', 'Copy failed. Use the system copy menu.')); } }}><Copy size={16}/></Button>
      </div>
      <div className="webssh-key-tools"><Button variant="ghost" disabled={!connected} aria-label={tr('粘贴', 'Paste')} title={tr('粘贴', 'Paste')} onClick={() => { setText(''); setPasteOpen(true); }}><ClipboardPaste size={18}/></Button>
      <ActionMenu menuClassName="webssh-touch-menu webssh-mobile-key-menu" disabled={!connected} placement="top" icon={<ChevronsUpDown size={18}/>} label={tr('更多按键', 'More keys')} items={[['Home', '\x1b[H'], ['End', '\x1b[F'], ['Page Up', '\x1b[5~'], ['Page Down', '\x1b[6~'], ['Delete', '\x1b[3~'], ['Ctrl+C', '\x03'], ['Ctrl+D', '\x04']].map(([label, data]) => ({ label, onClick: () => onKey(data) }))}/></div>
    </div>
    {notice && <div className="webssh-mobile-notice" role="status">{notice}</div>}
    <Modal className="webssh-mobile-sheet" open={pasteOpen} title={tr('粘贴到终端', 'Paste into terminal')} onClose={close} footer={<><Button onClick={close}>{tr('取消', 'Cancel')}</Button><Button variant="primary" disabled={!connected || !text} onClick={() => { onPaste(text); close(); }}>{tr('插入', 'Insert')}</Button></>}>
      <Field label={tr('粘贴内容', 'Text to paste')} hint={tr('确认内容后插入终端，不额外发送回车。', 'Review before inserting. No extra Enter key is sent.')}>
        <textarea className="liaison-input webssh-paste-input" autoFocus rows={5} value={text} autoCapitalize="none" autoCorrect="off" spellCheck={false} onChange={e => setText(e.target.value)} />
      </Field>
      {!connected && <Notice>{tr('连接已断开', 'Connection closed')}</Notice>}
    </Modal>
  </>;
}
