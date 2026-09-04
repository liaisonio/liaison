import { useI18n } from '@/i18n';
import {
  ArrowRight,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  TriangleAlert,
  X,
} from 'lucide-react';
import {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  ReactNode,
  SelectHTMLAttributes,
  useEffect,
  useRef,
  useState,
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

export function DateRangeField({
  label,
  start,
  end,
  startPlaceholder,
  endPlaceholder,
  onStartChange,
  onEndChange,
  className = '',
}: {
  label: ReactNode;
  start: string;
  end: string;
  startPlaceholder: string;
  endPlaceholder: string;
  onStartChange: (value: string) => void;
  onEndChange: (value: string) => void;
  className?: string;
}) {
  const { tr } = useI18n();
  const rootRef = useRef<HTMLDivElement>(null);
  const parseValue = (value: string) => {
    const match = value.match(/^(\d{4})-(\d{2})-(\d{2})/);
    if (!match) return undefined;
    return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  };
  const dateKey = (date: Date) => [
    date.getFullYear(),
    String(date.getMonth() + 1).padStart(2, '0'),
    String(date.getDate()).padStart(2, '0'),
  ].join('-');
  const startDate = parseValue(start);
  const endDate = parseValue(end);
  const [open, setOpen] = useState(false);
  const [visibleMonth, setVisibleMonth] = useState(
    () => new Date((startDate || new Date()).getFullYear(), (startDate || new Date()).getMonth(), 1),
  );

  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [open]);

  const shiftMonth = (offset: number) => {
    setVisibleMonth((value) => new Date(value.getFullYear(), value.getMonth() + offset, 1));
  };
  const selectDate = (date: Date) => {
    if (!startDate || endDate || date.getTime() < startDate.getTime()) {
      onStartChange(`${dateKey(date)}T00:00`);
      onEndChange('');
      return;
    }
    onEndChange(`${dateKey(date)}T23:59`);
    setOpen(false);
  };
  const sameDate = (left?: Date, right?: Date) => Boolean(
    left && right && dateKey(left) === dateKey(right),
  );
  const monthDays = (month: Date) => {
    const first = new Date(month.getFullYear(), month.getMonth(), 1);
    const cursor = new Date(first);
    cursor.setDate(first.getDate() - ((first.getDay() + 6) % 7));
    return Array.from({ length: 42 }, (_, index) => {
      const date = new Date(cursor);
      date.setDate(cursor.getDate() + index);
      return date;
    });
  };
  const monthLabel = (month: Date) => `${month.getFullYear()}-${String(month.getMonth() + 1).padStart(2, '0')}`;
  const weekdays = tr(['一', '二', '三', '四', '五', '六', '日'].join(','), ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su'].join(',')).split(',');
  const renderMonth = (month: Date) => (
    <section className="liaison-calendar-month" key={monthLabel(month)}>
      <div className="liaison-calendar-weekdays">
        {weekdays.map((weekday) => <span key={weekday}>{weekday}</span>)}
      </div>
      <div className="liaison-calendar-days">
        {monthDays(month).map((date) => {
          const isStart = sameDate(date, startDate);
          const isEnd = sameDate(date, endDate);
          const inRange = Boolean(startDate && endDate && date > startDate && date < endDate);
          return (
            <button
              type="button"
              key={dateKey(date)}
              className={[
                date.getMonth() !== month.getMonth() ? 'is-outside' : '',
                isStart ? 'is-start' : '',
                isEnd ? 'is-end' : '',
                inRange ? 'is-in-range' : '',
              ].filter(Boolean).join(' ')}
              aria-label={dateKey(date)}
              aria-pressed={isStart || isEnd}
              onClick={() => selectDate(date)}
            >
              {date.getDate()}
            </button>
          );
        })}
      </div>
    </section>
  );
  const nextMonth = new Date(visibleMonth.getFullYear(), visibleMonth.getMonth() + 1, 1);

  return (
    <div ref={rootRef} className={`liaison-date-range${open ? ' is-open' : ''} ${className}`.trim()}>
      <span className="liaison-date-range-key">{label}</span>
      <button
        type="button"
        className="liaison-date-range-values"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => {
          if (!open && startDate) setVisibleMonth(new Date(startDate.getFullYear(), startDate.getMonth(), 1));
          setOpen((value) => !value);
        }}
      >
        <span className={`liaison-date-range-value${startDate ? ' has-value' : ''}`}>{startDate ? dateKey(startDate) : startPlaceholder}</span>
        <ArrowRight className="liaison-date-range-arrow" size={12} strokeWidth={1.7} aria-hidden />
        <span className={`liaison-date-range-value${endDate ? ' has-value' : ''}`}>{endDate ? dateKey(endDate) : endPlaceholder}</span>
        <CalendarDays className="liaison-date-range-calendar" size={14} strokeWidth={1.7} aria-hidden />
      </button>
      {open ? (
        <div className="liaison-date-range-popup" role="dialog" aria-label={tr('选择时间范围', 'Select date range')}>
          <header>
            <span className="liaison-calendar-nav">
              <button type="button" aria-label={tr('上一年', 'Previous year')} onClick={() => shiftMonth(-12)}><ChevronsLeft size={15} /></button>
              <button type="button" aria-label={tr('上个月', 'Previous month')} onClick={() => shiftMonth(-1)}><ChevronLeft size={15} /></button>
            </span>
            <strong>{monthLabel(visibleMonth)}</strong>
            <strong>{monthLabel(nextMonth)}</strong>
            <span className="liaison-calendar-nav">
              <button type="button" aria-label={tr('下个月', 'Next month')} onClick={() => shiftMonth(1)}><ChevronRight size={15} /></button>
              <button type="button" aria-label={tr('下一年', 'Next year')} onClick={() => shiftMonth(12)}><ChevronsRight size={15} /></button>
            </span>
          </header>
          <div className="liaison-calendar-grid">
            {renderMonth(visibleMonth)}
            {renderMonth(nextMonth)}
          </div>
          <footer>
            <span>{startDate ? dateKey(startDate) : startPlaceholder} <ArrowRight size={12} /> {endDate ? dateKey(endDate) : endPlaceholder}</span>
            <button type="button" onClick={() => { onStartChange(''); onEndChange(''); }}>{tr('清除', 'Clear')}</button>
          </footer>
        </div>
      ) : null}
    </div>
  );
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
  width = 500,
  closeOnMask = true,
  className = '',
}: {
  open: boolean;
  title: ReactNode;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  width?: number;
  closeOnMask?: boolean;
  className?: string;
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
      <section className={`liaison-modal ${className}`.trim()} style={{ width }}>
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

export function DangerConfirm({
  title,
  description,
}: {
  title: ReactNode;
  description?: ReactNode;
}) {
  return (
    <div className="native-confirm-copy">
      <span className="native-confirm-icon"><TriangleAlert size={17} /></span>
      <div>
        <strong>{title}</strong>
        {description ? <p>{description}</p> : null}
      </div>
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
  fixed?: 'left' | 'right';
  render: (row: T) => ReactNode;
};

export function Timestamp({ value }: { value?: string }) {
  const raw = String(value || '').trim();
  const display = raw.replace('T', ' ');
  return <time className="liaison-time-cell" dateTime={raw || undefined} title={display}>{display || '-'}</time>;
}

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
        <thead><tr>{columns.map((column) => <th key={column.key} className={column.fixed ? `is-fixed-${column.fixed}` : undefined} style={{ width: column.width }}>{column.title}</th>)}</tr></thead>
        <tbody>
          {rows.map((row) => <tr key={rowKey(row)}>{columns.map((column) => <td key={column.key} className={column.fixed ? `is-fixed-${column.fixed}` : undefined}>{column.render(row)}</td>)}</tr>)}
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
  const { tr } = useI18n();
  const pages = Math.max(1, Math.ceil(total / pageSize));
  const pageItems: Array<number | 'ellipsis'> = (() => {
    if (pages <= 10) return Array.from({ length: pages }, (_, index) => index + 1);
    if (page <= 6) return [...Array.from({ length: 10 }, (_, index) => index + 1), 'ellipsis', pages];
    if (page >= pages - 5) return [1, 'ellipsis', ...Array.from({ length: 10 }, (_, index) => pages - 9 + index)];
    return [
      1,
      'ellipsis',
      ...Array.from({ length: 8 }, (_, index) => page - 3 + index),
      'ellipsis',
      pages,
    ];
  })();

  return (
    <div className="liaison-pager">
      <span>{tr(`共 ${total} 条`, `${total} items`)}</span>
      <nav className="liaison-pager-navigation" aria-label="Pagination">
        <button className="liaison-pager-arrow" type="button" aria-label="Previous page" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>‹</button>
        <div className="liaison-pager-pages">
          {pageItems.map((item, index) => item === 'ellipsis' ? (
            <span className="liaison-pager-ellipsis" key={`ellipsis-${index}`} aria-hidden>…</span>
          ) : (
            <button
              type="button"
              key={item}
              className={item === page ? 'is-current' : undefined}
              aria-current={item === page ? 'page' : undefined}
              aria-label={`Page ${item}`}
              onClick={() => onPageChange(item)}
            >
              {item}
            </button>
          ))}
        </div>
        <button className="liaison-pager-arrow" type="button" aria-label="Next page" disabled={page >= pages} onClick={() => onPageChange(page + 1)}>›</button>
      </nav>
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
