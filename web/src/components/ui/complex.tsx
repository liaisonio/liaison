import { ChevronDown, ChevronRight, X } from 'lucide-react';
import React, {
  Children,
  cloneElement,
  createContext,
  forwardRef,
  isValidElement,
  ReactNode,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Button as NativeButton, Modal as NativeModal, Notice } from '.';

type Values = Record<string, any>;
type FormApi = {
  values: Values;
  setFieldsValue: (values: Values) => void;
  setFieldValue: (name: string, value: any) => void;
  getFieldValue: (name: string) => any;
  validateFields: () => Promise<Values>;
  resetFields: () => void;
  submit: () => void;
  subscribe: (listener: () => void) => () => void;
  onSubmit?: (values: Values) => void;
};

export const message = (() => {
  const show = (text: ReactNode, tone: string) => {
    const node = document.createElement('div');
    node.className = `liaison-toast is-${tone}`;
    node.textContent = typeof text === 'string' ? text : String(text ?? '');
    document.body.appendChild(node);
    requestAnimationFrame(() => node.classList.add('is-visible'));
    window.setTimeout(() => {
      node.classList.remove('is-visible');
      window.setTimeout(() => node.remove(), 180);
    }, 2400);
  };
  return {
    success: (text: ReactNode) => show(text, 'success'),
    error: (text: ReactNode) => show(text, 'danger'),
    warning: (text: ReactNode) => show(text, 'warning'),
    info: (text: ReactNode) => show(text, 'info'),
  };
})();

const FormContext = createContext<FormApi | null>(null);

function createFormApi(): FormApi {
  const listeners = new Set<() => void>();
  const initial: Values = {};
  const api: FormApi = {
    values: { ...initial },
    setFieldsValue(values) {
      api.values = { ...api.values, ...values };
      listeners.forEach((listener) => listener());
    },
    setFieldValue(name, value) {
      api.setFieldsValue({ [name]: value });
    },
    getFieldValue(name) {
      return api.values[name];
    },
    async validateFields() {
      return { ...api.values };
    },
    resetFields() {
      api.values = {};
      listeners.forEach((listener) => listener());
    },
    submit() {
      api.onSubmit?.({ ...api.values });
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
  return api;
}

function FormRoot({ form, onFinish, children, className = '' }: any) {
  const internalForm = useMemo(createFormApi, []);
  const api = form || internalForm;
  api.onSubmit = onFinish;
  return (
    <FormContext.Provider value={api}>
      <form className={`liaison-form ${className}`.trim()} onSubmit={(event) => { event.preventDefault(); api.submit(); }}>
        {children}
      </form>
    </FormContext.Provider>
  );
}

function FormItem({ name, label, extra, children, valuePropName = 'value', rules = [], className = '' }: any) {
  const form = useContext(FormContext);
  const [, render] = useState(0);
  useEffect(() => form?.subscribe(() => render((value) => value + 1)), [form]);
  const child = Children.only(children) as React.ReactElement<any>;
  const required = rules.some((rule: any) => rule?.required);
  const value = name ? form?.getFieldValue(name) : undefined;
  const controlled = name && isValidElement(child)
    ? cloneElement<any>(child, {
        [valuePropName]: valuePropName === 'checked' ? Boolean(value) : value ?? '',
        onChange: (event: any) => {
          const next = event?.target
            ? valuePropName === 'checked'
              ? event.target.checked
              : event.target.value
            : event;
          form?.setFieldValue(name, next);
          (child as React.ReactElement<any>).props.onChange?.(event);
        },
      })
    : child;
  return <label className={`liaison-field ${className}`.trim()}><span className="liaison-field-label">{required ? <i>*</i> : null}{label}</span>{controlled}{extra ? <small>{extra}</small> : null}</label>;
}

export const Form: any = Object.assign(FormRoot, {
  Item: FormItem,
  useForm: () => {
    const ref = useRef<FormApi>();
    if (!ref.current) ref.current = createFormApi();
    return [ref.current];
  },
});

export function Button({ type, danger, icon, loading, children, ...props }: any) {
  const htmlType = type === 'submit' ? 'submit' : props.htmlType || 'button';
  const variant = danger ? 'danger' : type === 'primary' ? 'primary' : type === 'link' ? 'ghost' : 'secondary';
  return <NativeButton {...props} type={htmlType} variant={variant} loading={loading}>{icon}{children}</NativeButton>;
}

function TextInput({ prefix, onPressEnter, size: _size, autoSize: _autoSize, ...props }: any) {
  const input = <input {...props} className={`liaison-input ${props.className || ''}`.trim()} onKeyDown={(event) => { props.onKeyDown?.(event); if (event.key === 'Enter') onPressEnter?.(event); }} />;
  return prefix ? <span className="liaison-input-wrap">{prefix}{input}</span> : input;
}
function PasswordInput(props: any) { return <TextInput {...props} type="password" />; }
const TextAreaInput = forwardRef<HTMLTextAreaElement, any>(
  ({ autoSize: _autoSize, onPressEnter, ...props }, ref) => (
    <textarea
      {...props}
      ref={ref}
      className={`liaison-input liaison-textarea ${props.className || ''}`.trim()}
      onKeyDown={(event) => {
        props.onKeyDown?.(event);
        if (event.key === 'Enter') onPressEnter?.(event);
      }}
    />
  ),
);
TextAreaInput.displayName = 'TextAreaInput';
export const Input: any = Object.assign(TextInput, { Password: PasswordInput, TextArea: TextAreaInput });
export const InputNumber = ({ min, max, precision: _precision, onChange, ...props }: any) => <TextInput {...props} type="number" min={min} max={max} onChange={(event: React.ChangeEvent<HTMLInputElement>) => onChange?.(event.target.value === '' ? undefined : Number(event.target.value))} />;

export function Select({ options = [], allowClear, ...props }: any) {
  return <select {...props} className={`liaison-input liaison-select ${props.className || ''}`.trim()} onChange={(event) => props.onChange?.(event.target.value)}><option value="">{allowClear ? '—' : props.placeholder || '—'}</option>{options.map((item: any) => <option key={String(item.value)} value={item.value}>{item.label}</option>)}</select>;
}

export const Switch = ({ checked, onChange, ...props }: any) => <button {...props} type="button" role="switch" aria-checked={checked} className={`liaison-switch ${checked ? 'is-on' : ''}`} onClick={() => onChange?.(!checked)}><i /></button>;
export const Spin = ({ children, spinning = true }: any) => children ? <div className={`liaison-spin-wrap ${spinning ? 'is-spinning' : ''}`}>{children}{spinning ? <span className="ui-spinner" /> : null}</div> : <span className="ui-spinner" />;
export const Alert = ({ type = 'info', message: content, className = '' }: any) => <Notice className={className} tone={type === 'error' ? 'danger' : type}>{content}</Notice>;
export const Tag = ({ children, color = '' }: any) => <span className={`liaison-status ${color ? `is-${color}` : ''}`}>{children}</span>;
export const Tooltip = ({ title, children }: any) => isValidElement(children) ? cloneElement(children as React.ReactElement<any>, { title: typeof title === 'string' ? title : undefined }) : children;

export function Space({ children, direction, wrap, className = '' }: any) { return <div className={`liaison-space ${direction === 'vertical' ? 'is-vertical' : ''} ${wrap ? 'is-wrap' : ''} ${className}`.trim()}>{children}</div>; }
export const Empty: any = ({ description, children }: any) => <div className="liaison-empty"><span>{description || 'No data'}</span>{children}</div>;
Empty.PRESENTED_IMAGE_SIMPLE = true;

export function Table({ columns = [], dataSource = [], rowKey, className = '', expandable }: any) {
  const [expandedKeys, setExpandedKeys] = useState<Set<string>>(new Set());
  const keyOf = (row: any, index: number) => typeof rowKey === 'function' ? rowKey(row, index) : row[rowKey || 'key'] ?? index;
  return <div className={`liaison-table-scroll ${className}`.trim()}><table className="liaison-table"><thead><tr>{expandable ? <th aria-label="Expand" /> : null}{columns.map((column: any, index: number) => <th key={column.key || column.dataIndex || index} style={{ width: column.width }}>{column.title}</th>)}</tr></thead><tbody>{dataSource.map((row: any, rowIndex: number) => { const rawKey = keyOf(row, rowIndex); const key = String(rawKey); const canExpand = expandable?.rowExpandable ? expandable.rowExpandable(row) : Boolean(expandable); const expanded = expandedKeys.has(key); return <React.Fragment key={key}><tr>{expandable ? <td><button type="button" className="liaison-row-expand" disabled={!canExpand} onClick={() => setExpandedKeys((current) => { const next = new Set(current); expanded ? next.delete(key) : next.add(key); return next; })}>{expanded ? '−' : '+'}</button></td> : null}{columns.map((column: any, colIndex: number) => { const value = row[column.dataIndex]; return <td key={column.key || column.dataIndex || colIndex}>{column.render ? column.render(value, row, rowIndex) : String(value ?? '')}</td>; })}</tr>{expanded ? <tr><td colSpan={columns.length + 1} className="liaison-expanded-row">{expandable.expandedRowRender(row)}</td></tr> : null}</React.Fragment>; })}</tbody></table></div>;
}

function ModalRoot({ open, title, onCancel, onOk, confirmLoading, okText = 'OK', cancelText = 'Cancel', footer, children, width, className }: any) {
  const modalFooter = footer === null ? undefined : footer || <div className="liaison-modal-actions"><NativeButton onClick={onCancel}>{cancelText}</NativeButton><NativeButton variant="primary" loading={confirmLoading} onClick={onOk}>{okText}</NativeButton></div>;
  return <NativeModal open={open} title={title} onClose={onCancel} footer={modalFooter} width={width} className={className}>{children}</NativeModal>;
}
export const Modal: any = Object.assign(ModalRoot, {
  confirm: ({ title, content, onOk }: any) => {
    const copy = [title, content].filter(Boolean).join('\n\n');
    if (window.confirm(copy)) return onOk?.();
    return undefined;
  },
});

export function Drawer({ open, title, onClose, footer, children, width = 560, className = '', extra }: any) {
  if (!open) return null;
  return <div className={`liaison-drawer-root ${className}`.trim()}><button className="liaison-drawer-mask" onClick={onClose} aria-label="Close" /><aside className="liaison-drawer" style={{ width }}><header><div className="liaison-drawer-heading"><h2>{title}</h2>{extra ? <div className="liaison-drawer-extra">{extra}</div> : null}</div><button onClick={onClose} aria-label="Close"><X size={19} /></button></header><div className="liaison-drawer-body">{children}</div>{footer ? <footer>{footer}</footer> : null}</aside></div>;
}

export function Collapse({ items = [] }: any) { return <div className="liaison-collapse">{items.map((item: any) => <details key={item.key}><summary>{item.label}<ChevronDown size={16} /></summary><div>{item.children}</div></details>)}</div>; }
export function Tabs({ items = [] }: any) { const [active, setActive] = useState(items[0]?.key); const item = items.find((entry: any) => entry.key === active) || items[0]; return <div className="liaison-tabs"><nav>{items.map((entry: any) => <button key={entry.key} className={entry.key === item?.key ? 'is-active' : ''} onClick={() => setActive(entry.key)}>{entry.label}</button>)}</nav><div>{item?.children}</div></div>; }
export function Descriptions({ items = [] }: any) { return <dl className="liaison-descriptions">{items.map((item: any) => <React.Fragment key={item.key}><dt>{item.label}</dt><dd>{item.children}</dd></React.Fragment>)}</dl>; }
export function Tree({
  treeData = [],
  onSelect,
  selectedKeys = [],
  defaultExpandAll = false,
  defaultExpandedKeys = [],
}: any) {
  const allExpandableKeys = useMemo(() => {
    const keys: string[] = [];
    const walk = (nodes: any[]) => nodes.forEach((node) => {
      if (node.children?.length) {
        keys.push(String(node.key));
        walk(node.children);
      }
    });
    walk(treeData);
    return keys;
  }, [treeData]);
  const [expandedKeys, setExpandedKeys] = useState<Set<string>>(
    () => new Set((defaultExpandAll ? allExpandableKeys : defaultExpandedKeys).map(String)),
  );
  const defaultsAppliedRef = useRef(false);

  useEffect(() => {
    if (defaultExpandAll) {
      setExpandedKeys(new Set(allExpandableKeys));
      defaultsAppliedRef.current = true;
    } else if (!defaultsAppliedRef.current && defaultExpandedKeys.length) {
      defaultsAppliedRef.current = true;
      setExpandedKeys(new Set(defaultExpandedKeys.map(String)));
    }
  }, [defaultExpandAll, defaultExpandedKeys, allExpandableKeys]);

  const toggle = (key: string) => setExpandedKeys((current) => {
    const next = new Set(current);
    next.has(key) ? next.delete(key) : next.add(key);
    return next;
  });
  const selected = new Set(selectedKeys.map(String));
  const render = (nodes: any[]) => (
    <ul>
      {nodes.map((node) => {
        const key = String(node.key);
        const expandable = Boolean(node.children?.length);
        const expanded = expandedKeys.has(key);
        return (
          <li key={key}>
            <div className={`liaison-tree-row${selected.has(key) ? ' is-selected' : ''}`}>
              {expandable ? (
                <button
                  type="button"
                  className="liaison-tree-toggle"
                  aria-label={expanded ? 'Collapse' : 'Expand'}
                  aria-expanded={expanded}
                  onClick={() => toggle(key)}
                >
                  {expanded ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
                </button>
              ) : <span className="liaison-tree-toggle-placeholder" />}
              <button
                type="button"
                className="liaison-tree-label"
                onClick={() => onSelect?.([node.key], { node })}
              >
                {node.title}
              </button>
            </div>
            {expandable && expanded ? render(node.children) : null}
          </li>
        );
      })}
    </ul>
  );
  return <div className="liaison-tree">{render(treeData)}</div>;
}

const Text = ({ children, strong, type, className = '' }: any) => <span className={`${strong ? 'is-strong' : ''} ${type === 'secondary' ? 'is-secondary' : ''} ${type === 'danger' ? 'is-danger-text' : ''} ${className}`.trim()}>{children}</span>;
const Title = ({ children, level = 2, className = '' }: any) => React.createElement(`h${Math.min(6, Math.max(1, level))}`, { className }, children);
export const Typography = { Text, Title };
