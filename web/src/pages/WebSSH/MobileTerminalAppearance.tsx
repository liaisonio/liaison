import { Button, Field, Input, Modal } from '@/components/ui';
import { useI18n } from '@/i18n';

export function readMobileFontSize() {
  try {
    const value = Number(localStorage.getItem('liaison.ssh.mobile-font-size'));
    return Number.isInteger(value) && value >= 10 && value <= 20 ? value : 10;
  } catch { return 10; }
}

export default function MobileTerminalAppearance({ open, size, onChange, onClose }: {
  open: boolean; size: number; onChange: (size: number) => void; onClose: () => void;
}) {
  const { tr } = useI18n();
  return <Modal className="webssh-mobile-sheet" open={open} title={tr('终端显示', 'Terminal appearance')} onClose={onClose}
    footer={<><Button onClick={() => onChange(10)}>{tr('恢复默认', 'Reset')}</Button><Button variant="primary" onClick={onClose}>{tr('完成', 'Done')}</Button></>}>
    <div className="webssh-appearance-settings">
      <Field label={<>{tr('文字大小', 'Text size')}<output className="webssh-font-value">{size} px</output></>}
        hint={tr('立即生效，不会中断连接。', 'Applies immediately without disconnecting.')}>
        <Input className="webssh-font-range" aria-label={tr('文字大小', 'Text size')} type="range" min={10} max={20} step={1} value={size} onChange={e => onChange(Number(e.target.value))}/>
      </Field>
      <pre className="webssh-font-preview" style={{ fontSize: size }} aria-label={tr('字体预览', 'Font preview')}>{'user@host ~ % ls\nDocuments  projects\n0123456789  {} [] / ~'}</pre>
    </div>
  </Modal>;
}
