import {ACCESS_GROUPS,accessGroup} from '@/constants/accessGroups';
import { AuditLogIcon } from '@/components/icons/AuditLogIcon';
import { ApplicationIcon } from '@/components/icons/ApplicationIcon';
import { useI18n } from '@/i18n';
import { useUi } from '@/store/ui';
import { useFeature } from '@/store/permissions';
import type { LucideIcon } from 'lucide-react';
import {
  Boxes,
  Cable,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Gauge,
  House,
  Globe2,
  Settings,
  Users,
} from 'lucide-react';
import { useEffect, useState } from 'react';
import { Link, NavLink, useLocation } from 'react-router-dom';

type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
  end?: boolean;
};

export function Sidebar() {
  const homeAI = useFeature('ai.home.use');
  const audit = useFeature('audit.read');
  const { tr } = useI18n();
  const collapsed = useUi((state) => state.sidebarCollapsed);
  const toggleSidebar = useUi((state) => state.toggleSidebar);
  const location = useLocation();
  const [accessOpen, setAccessOpen] = useState(location.pathname === '/proxy');
  const [logsOpen, setLogsOpen] = useState(location.pathname.startsWith('/logs/'));

  const primary: NavItem[] = [
    { to: '/', label: tr('首页', 'Home'), icon: House, end: true },
    { to: '/dashboard', label: tr('仪表盘', 'Dashboard'), icon: Gauge },
    { to: '/connector', label: tr('连接器', 'Connectors'), icon: Cable },
    { to: '/resource/device', label: tr('设备', 'Devices'), icon: Boxes },
  ];
  const lowerNav: NavItem[] = [
    { to: '/users', label: tr('用户管理', 'Users'), icon: Users },
    { to: '/settings', label: tr('设置', 'Settings'), icon: Settings },
  ];

  const activeAccessType = accessGroup(new URLSearchParams(location.search))?.value;

  useEffect(() => {
    if (location.pathname === '/proxy') {
      setAccessOpen(true);
    }
  }, [location.pathname]);

  useEffect(() => {
    if (location.pathname.startsWith('/logs/')) setLogsOpen(true);
  }, [location.pathname]);

  const renderItems = (items: NavItem[]) =>
    items.filter(item => item.to !== '/' || homeAI).map(({ to, label, icon: Icon, end }) => (
      <NavLink
        key={to}
        to={to}
        end={end}
        title={collapsed ? label : undefined}
        className={({ isActive }) =>
          `liaison-nav-item${isActive || (to === '/' && location.pathname.startsWith('/agent/sessions/')) ? ' is-active' : ''}`
        }
      >
        <Icon size={17} strokeWidth={1.8} />
        {!collapsed && <span>{label}</span>}
      </NavLink>
    ));

  return (
    <aside className={`liaison-sidebar${collapsed ? ' is-collapsed' : ''}`}>
      <button
        type="button"
        className="liaison-sidebar-boundary-toggle"
        onClick={toggleSidebar}
        aria-label={collapsed ? tr('展开侧栏', 'Expand sidebar') : tr('收起侧栏', 'Collapse sidebar')}
        title={collapsed ? tr('展开菜单', 'Expand menu') : tr('收起菜单', 'Collapse menu')}
      >
        {collapsed ? <ChevronRight size={13} /> : <ChevronLeft size={13} />}
      </button>
      <nav className="liaison-nav">
        <div className="liaison-nav-primary">
          {renderItems(primary.slice(0, 2))}
          {collapsed ? (
            <NavLink
              to="/proxy"
              title={tr('访问', 'Access')}
              className={({ isActive }) =>
                `liaison-nav-item${isActive ? ' is-active' : ''}`
              }
            >
              <Globe2 size={17} strokeWidth={1.8} />
              {!collapsed && <span>{tr('访问', 'Access')}</span>}
            </NavLink>
          ) : (
            <div
              className={`liaison-nav-expandable${
                location.pathname === '/proxy' ? ' is-active' : ''
              }`}
            >
              <button
                type="button"
                className="liaison-nav-item liaison-nav-toggle"
                onClick={() => {
                  setAccessOpen((open) => !open);
                }}
                aria-expanded={accessOpen}
              >
                <Globe2 size={17} strokeWidth={1.8} />
                <span>{tr('访问', 'Access')}</span>
                <ChevronDown
                  className={`liaison-nav-chevron${
                    accessOpen ? ' is-open' : ''
                  }`}
                  size={15}
                />
              </button>
              {accessOpen && (
                <div className="liaison-nav-children">
                  <Link
                    to="/proxy"
                    className={`liaison-nav-child${
                      location.pathname === '/proxy' && !activeAccessType
                        ? ' is-active'
                        : ''
                    }`}
                  >
                    <span>{tr('全部访问', 'All access')}</span>
                  </Link>
                  {ACCESS_GROUPS.map((type) => (
                    <Link
                      key={type.value}
                      to={`/proxy?category=${type.value}`}
                      className={`liaison-nav-child${
                        location.pathname === '/proxy' &&
                        activeAccessType === type.value
                          ? ' is-active'
                          : ''
                      }`}
                    >
                      <span>{type.label}</span>
                    </Link>
                  ))}
                </div>
              )}
            </div>
          )}
          {renderItems(primary.slice(2))}
          <NavLink
            to="/resource/app"
            title={collapsed ? tr('应用', 'Applications') : undefined}
            className={({ isActive }) =>
              `liaison-nav-item${isActive ? ' is-active' : ''}`
            }
          >
            <ApplicationIcon size={17} />
            {!collapsed && <span>{tr('应用', 'Applications')}</span>}
          </NavLink>
        </div>
      </nav>

      <div className="liaison-sidebar-bottom">
        <nav className="liaison-nav-lower" aria-label={tr('系统', 'System')}>
          {audit && (collapsed ? (
            <NavLink to="/logs/management" title={tr('日志与审计', 'Logs & Audit')} className={({ isActive }) => `liaison-nav-item${isActive || location.pathname.startsWith('/logs/') ? ' is-active' : ''}`}>
              <AuditLogIcon size={17} />
            </NavLink>
          ) : (
            <div className={`liaison-nav-expandable${location.pathname.startsWith('/logs/') ? ' is-active' : ''}`}>
              <button type="button" className="liaison-nav-item liaison-nav-toggle" onClick={() => setLogsOpen((open) => !open)} aria-expanded={logsOpen}>
                <AuditLogIcon size={17} />
                <span>{tr('日志与审计', 'Logs & Audit')}</span>
                <ChevronDown className={`liaison-nav-chevron${logsOpen ? ' is-open' : ''}`} size={15} />
              </button>
              {logsOpen ? <div className="liaison-nav-children liaison-nav-log-children">
                <Link to="/logs/management" className={`liaison-nav-child${location.pathname === '/logs/management' ? ' is-active' : ''}`}><span>{tr('管理日志', 'Management logs')}</span></Link>
                <Link to="/logs/audit" className={`liaison-nav-child${location.pathname === '/logs/audit' ? ' is-active' : ''}`}><span>{tr('审计日志', 'Audit logs')}</span></Link>
              </div> : null}
            </div>
          ))}
          {renderItems(lowerNav)}
        </nav>
      </div>
    </aside>
  );
}
