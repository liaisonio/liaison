import {
  ACCESS_TYPES,
  ACCESS_TYPES_CHANGED_EVENT,
  isAccessType,
} from '@/constants/accessTypes';
import {
  APPLICATION_TYPES_CHANGED_EVENT,
  isApplicationType,
} from '@/constants/applicationTypes';
import { useI18n } from '@/i18n';
import { getApplicationList } from '@/services/api';
import { useUi } from '@/store/ui';
import { AppstoreOutlined, FileSearchOutlined } from '@ant-design/icons';
import type { LucideIcon, LucideProps } from 'lucide-react';
import {
  Boxes,
  Cable,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Gauge,
  Globe2,
  Settings,
  Users,
} from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { Link, NavLink, useLocation } from 'react-router-dom';

type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
  end?: boolean;
};

const CloudApplicationsIcon = ({ size = 17, className }: LucideProps) => (
  <AppstoreOutlined className={className} style={{ fontSize: Number(size) }} />
);

const CloudAuditIcon = ({ size = 17, className }: LucideProps) => (
  <FileSearchOutlined
    className={className}
    style={{ fontSize: Number(size) }}
  />
);

export function Sidebar() {
  const { tr } = useI18n();
  const collapsed = useUi((state) => state.sidebarCollapsed);
  const toggleSidebar = useUi((state) => state.toggleSidebar);
  const location = useLocation();
  const [accessOpen, setAccessOpen] = useState(location.pathname === '/proxy');
  const [availableAccessTypes, setAvailableAccessTypes] = useState<Set<string>>(
    new Set(),
  );

  const primary: NavItem[] = [
    { to: '/dashboard', label: tr('总览', 'Overview'), icon: Gauge },
    { to: '/resource/device', label: tr('设备', 'Devices'), icon: Boxes },
    { to: '/connector', label: tr('连接器', 'Connectors'), icon: Cable },
  ];
  const lowerNav: NavItem[] = [
    {
      to: '/audit',
      label: tr('日志与审计', 'Logs & Audit'),
      icon: CloudAuditIcon as LucideIcon,
    },
    { to: '/users', label: tr('用户管理', 'Users'), icon: Users },
    { to: '/settings', label: tr('设置', 'Settings'), icon: Settings },
  ];

  const accessType = new URLSearchParams(location.search).get('access_type');
  const activeAccessType = isAccessType(accessType) ? accessType : undefined;
  const visibleAccessTypes = ACCESS_TYPES.filter((type) =>
    availableAccessTypes.has(type.value),
  );

  const loadAccessTypes = useCallback(async () => {
    try {
      const response = await getApplicationList({ page_size: 1000 });
      const types = new Set<string>();
      for (const application of response.data?.applications || []) {
        const type = String(application.application_type || '').toLowerCase();
        if (!isApplicationType(type)) continue;
        types.add(type);
        if (type === 'ssh') types.add('webssh');
      }
      setAvailableAccessTypes(types);
    } catch {
      // Keep the last successful menu snapshot when the control plane is unavailable.
    }
  }, []);

  useEffect(() => {
    void loadAccessTypes();
  }, [loadAccessTypes, location.pathname]);

  useEffect(() => {
    const refresh = () => void loadAccessTypes();
    window.addEventListener(ACCESS_TYPES_CHANGED_EVENT, refresh);
    window.addEventListener(APPLICATION_TYPES_CHANGED_EVENT, refresh);
    return () => {
      window.removeEventListener(ACCESS_TYPES_CHANGED_EVENT, refresh);
      window.removeEventListener(APPLICATION_TYPES_CHANGED_EVENT, refresh);
    };
  }, [loadAccessTypes]);

  useEffect(() => {
    if (location.pathname === '/proxy') {
      setAccessOpen(true);
    }
  }, [location.pathname]);

  const renderItems = (items: NavItem[]) =>
    items.map(({ to, label, icon: Icon, end }) => (
      <NavLink
        key={to}
        to={to}
        end={end}
        title={collapsed ? label : undefined}
        className={({ isActive }) =>
          `liaison-nav-item${isActive ? ' is-active' : ''}`
        }
      >
        <Icon size={17} strokeWidth={1.8} />
        {!collapsed && <span>{label}</span>}
      </NavLink>
    ));

  return (
    <aside className={`liaison-sidebar${collapsed ? ' is-collapsed' : ''}`}>
      <button
        className="liaison-sidebar-collapse"
        onClick={toggleSidebar}
        aria-label={
          collapsed
            ? tr('展开侧栏', 'Expand sidebar')
            : tr('收起侧栏', 'Collapse sidebar')
        }
        title={
          collapsed
            ? tr('展开菜单', 'Expand menu')
            : tr('收起菜单', 'Collapse menu')
        }
      >
        {collapsed ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
      </button>

      <nav className="liaison-nav">
        <div className="liaison-nav-primary">
          {renderItems(primary.slice(0, 1))}
          {collapsed || visibleAccessTypes.length === 0 ? (
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
                  if (!accessOpen) void loadAccessTypes();
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
                  {visibleAccessTypes.map((type) => (
                    <Link
                      key={type.value}
                      to={`/proxy?access_type=${type.value}`}
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
          {renderItems(primary.slice(1))}
          <NavLink
            to="/resource/app"
            title={collapsed ? tr('应用', 'Applications') : undefined}
            className={({ isActive }) =>
              `liaison-nav-item${isActive ? ' is-active' : ''}`
            }
          >
            <CloudApplicationsIcon size={17} />
            {!collapsed && <span>{tr('应用', 'Applications')}</span>}
          </NavLink>
        </div>
      </nav>

      <div className="liaison-sidebar-bottom">
        <nav className="liaison-nav-lower" aria-label={tr('系统', 'System')}>
          {renderItems(lowerNav)}
        </nav>
      </div>
    </aside>
  );
}
