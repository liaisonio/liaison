import { MoreHorizontal, MoreVertical } from 'lucide-react';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import './ActionMenu.less';

export type ActionMenuItem = {
  label: string;
  onClick: () => void;
  danger?: boolean;
  disabled?: boolean;
};
export default function ActionMenu({
  label,
  items,
  vertical = false,
  icon,
  disabled = false,
  placement = 'bottom',
}: {
  label: string;
  items: ActionMenuItem[];
  vertical?: boolean;
  icon?: ReactNode;
  disabled?: boolean;
  placement?: 'top' | 'bottom';
}) {
  const trigger = useRef<HTMLButtonElement>(null),
    menu = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ top: number; left: number }>();
  const close = (restore = false) => {
    setPosition(undefined);
    if (restore) trigger.current?.focus({ preventScroll: true });
  };
  useEffect(() => {
    if (!position) return;
    menu.current
      ?.querySelector<HTMLButtonElement>('button:not(:disabled)')
      ?.focus({ preventScroll: true });
    const outside = (e: PointerEvent) => {
      if (
        !menu.current?.contains(e.target as Node) &&
        !trigger.current?.contains(e.target as Node)
      )
        close();
    };
    const scroll = (e: Event) => {
      if (!menu.current?.contains(e.target as Node)) close();
    };
    document.addEventListener('pointerdown', outside);
    window.addEventListener('resize', scroll);
    window.addEventListener('scroll', scroll, true);
    return () => {
      document.removeEventListener('pointerdown', outside);
      window.removeEventListener('resize', scroll);
      window.removeEventListener('scroll', scroll, true);
    };
  }, [position]);
  const show = () => {
    const r = trigger.current?.getBoundingClientRect();
    if (r)
      setPosition({
        left: Math.max(8, Math.min(placement === 'top' ? r.left : r.right - 176, innerWidth - 184)),
        top: placement === 'top' ? Math.max(40, r.top - 6) : Math.max(
          8,
          Math.min(r.bottom + 6, innerHeight - items.length * 36 - 24),
        ),
      });
  };
  return (
    <>
      <button
        type="button"
        className="liaison-action-menu-trigger"
        ref={trigger}
        disabled={disabled}
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={!!position}
        onClick={() => (position ? close(true) : show())}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown') {
            e.preventDefault();
            show();
          }
        }}
      >
        {icon ?? (vertical ? <MoreVertical size={16} /> : <MoreHorizontal size={16} />)}
      </button>
      {position &&
        createPortal(
          <div
            className="liaison-action-menu"
            role="menu"
            aria-label={label}
            ref={menu}
            style={placement === 'top' ? {...position, transform: 'translateY(-100%)', maxHeight: position.top - 8, maxWidth: 'calc(100vw - 16px)'} : position}
            onBlur={(e) => {
              if (!e.currentTarget.contains(e.relatedTarget)) close();
            }}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.preventDefault();
                close(true);
              }
              if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                e.preventDefault();
                const buttons = Array.from(
                  menu.current?.querySelectorAll<HTMLButtonElement>(
                    'button:not(:disabled)',
                  ) || [],
                );
                const i = buttons.indexOf(
                  document.activeElement as HTMLButtonElement,
                );
                buttons[
                  (i + (e.key === 'ArrowDown' ? 1 : -1) + buttons.length) %
                    buttons.length
                ]?.focus({ preventScroll: true });
              }
            }}
          >
            {items.map((item) => (
              <button
                key={item.label}
                role="menuitem"
                type="button"
                className={item.danger ? 'is-danger' : ''}
                disabled={item.disabled}
                onClick={() => {
                  close(true);
                  item.onClick();
                }}
              >
                {item.label}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </>
  );
}
