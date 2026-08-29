import { X } from 'lucide-react';
import {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  ReactNode,
  SelectHTMLAttributes,
  useEffect,
} from 'react';

export function Button({
  variant = 'secondary',
  className = '',
  type = 'button',
  loading = false,
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  loading?: boolean;
}) {
  return (
    <button
      type={type}
      className={`liaison-button is-${variant} ${className}`.trim()}
      aria-busy={loading || undefined}
      disabled={loading || props.disabled}
      {...props}
    >
      {loading ? <span className="ui-spinner" aria-hidden /> : null}
      {children}
    </button>
  );
}

export function Field({
  label,
  hint,
  children,
  required,
}: {
  label: ReactNode;
  hint?: ReactNode;
  children: ReactNode;
  required?: boolean;
}) {
  return (
    <label className="liaison-field">
      <span className="liaison-field-label">
        {required ? <i aria-hidden>*</i> : null}
        {label}
      </span>
      {children}
      {hint ? <small>{hint}</small> : null}
    </label>
  );
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={`liaison-input ${props.className || ''}`.trim()} {...props} />;
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={`liaison-input liaison-select ${props.className || ''}`.trim()} {...props} />;
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: { label: ReactNode; value: T }[];
  onChange: (value: T) => void;
}) {
  return (
    <div className="liaison-segmented">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          className={option.value === value ? 'is-active' : ''}
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

export function Modal({
  open,
  title,
  onClose,
  children,
  footer,
  width = 560,
  closeOnMask = true,
}: {
  open: boolean;
  title: ReactNode;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  width?: number;
  closeOnMask?: boolean;
}) {
  useEffect(() => {
    if (!open) return;
    const close = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', close);
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', close);
      document.body.style.overflow = previousOverflow;
    };
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="liaison-modal-root" role="dialog" aria-modal="true">
      <button
        type="button"
        className="liaison-modal-mask"
        aria-label="Close dialog"
        onClick={closeOnMask ? onClose : undefined}
      />
      <section className="liaison-modal" style={{ width }}>
        <header>
          <h2>{title}</h2>
          <button type="button" onClick={onClose} aria-label="Close">
            <X size={19} />
          </button>
        </header>
        <div className="liaison-modal-body">{children}</div>
        {footer ? <footer>{footer}</footer> : null}
      </section>
    </div>
  );
}

export function Notice({
  tone = 'info',
  className = '',
  children,
}: {
  tone?: 'info' | 'danger' | 'success' | 'warning';
  className?: string;
  children: ReactNode;
}) {
  return <div className={`liaison-notice is-${tone} ${className}`.trim()}>{children}</div>;
}

export type Column<T> = {
  key: string;
  title: ReactNode;
  width?: number | string;
  render: (row: T) => ReactNode;
};

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  loading,
  emptyText,
}: {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string | number;
  loading?: boolean;
  emptyText: ReactNode;
}) {
  return (
    <div className="liaison-table-scroll">
      <table className="liaison-table">
        <thead><tr>{columns.map((column) => <th key={column.key} style={{ width: column.width }}>{column.title}</th>)}</tr></thead>
        <tbody>
          {rows.map((row) => <tr key={rowKey(row)}>{columns.map((column) => <td key={column.key}>{column.render(row)}</td>)}</tr>)}
          {!loading && rows.length === 0 ? <tr><td className="liaison-table-empty" colSpan={columns.length}>{emptyText}</td></tr> : null}
          {loading ? <tr><td className="liaison-table-empty" colSpan={columns.length}>…</td></tr> : null}
        </tbody>
      </table>
    </div>
  );
}

export function Pager({
  page,
  pageSize,
  total,
  onPageChange,
}: {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  return (
    <div className="liaison-pager">
      <span>{total} items</span>
      <Button variant="ghost" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>‹</Button>
      <b>{page}</b>
      <Button variant="ghost" disabled={page >= pages} onClick={() => onPageChange(page + 1)}>›</Button>
    </div>
  );
}

export function StatusPill({ tone = 'neutral', children }: { tone?: 'neutral' | 'success' | 'danger' | 'info'; children: ReactNode }) {
  return <span className={`liaison-status is-${tone}`}>{children}</span>;
}

export function Drawer({
  open,
  title,
  onClose,
  children,
}: {
  open: boolean;
  title: ReactNode;
  onClose: () => void;
  children: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const close = (event: KeyboardEvent) => event.key === 'Escape' && onClose();
    document.addEventListener('keydown', close);
    return () => document.removeEventListener('keydown', close);
  }, [open, onClose]);
  if (!open) return null;
  return <div className="liaison-drawer-root"><button className="liaison-drawer-mask" onClick={onClose} aria-label="Close" /><aside className="liaison-drawer"><header><h2>{title}</h2><button onClick={onClose}><X size={19} /></button></header><div>{children}</div></aside></div>;
}
