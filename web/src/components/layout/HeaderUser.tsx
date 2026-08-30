import { useI18n } from '@/i18n';
import { logout } from '@/services/api';
import {
  getAccountLabel,
  getAvatarIdentity,
  useAvatarStore,
} from '@/store/avatar';
import { useSession } from '@/store/session';
import { ChevronDown, LogOut, UserRound } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';

export function HeaderUser() {
  const { tr } = useI18n();
  const user = useSession((state) => state.initialState.currentUser);
  const identity = getAvatarIdentity(user);
  const localAvatar = useAvatarStore((state) => state.avatars[identity]);
  const clear = useSession((state) => state.clear);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const accountLabel = getAccountLabel(user) || tr('用户', 'User');
  const avatar = localAvatar || user?.avatar;

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

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // The local session must still be cleared when the server is unreachable.
    }
    clear();
  };

  return (
    <div className="liaison-header-user" ref={rootRef}>
      <button
        type="button"
        className="liaison-header-user-trigger"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        aria-label={tr('打开用户菜单', 'Open user menu')}
      >
        <span className={`liaison-avatar${avatar ? ' is-image' : ''}`}>
          {avatar ? (
            <img src={avatar} alt="" />
          ) : (
            accountLabel.slice(0, 1).toUpperCase()
          )}
        </span>
        <span className="liaison-header-user-copy">{accountLabel}</span>
        <ChevronDown
          className={open ? 'is-open' : undefined}
          size={13}
          strokeWidth={1.8}
        />
      </button>

      {open && (
        <div className="liaison-header-user-menu">
          <Link to="/users" onClick={() => setOpen(false)}>
            <UserRound size={15} />
            <span>{tr('用户管理', 'Users')}</span>
          </Link>
          <div className="liaison-header-user-divider" />
          <button type="button" className="is-danger" onClick={handleLogout}>
            <LogOut size={15} />
            <span>{tr('退出登录', 'Log out')}</span>
          </button>
        </div>
      )}
    </div>
  );
}
