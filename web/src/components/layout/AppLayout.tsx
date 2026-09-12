import { LiaisonLogo } from '@/components/LiaisonLogo';
import { useI18n } from '@/i18n';
import { getCurrentUser } from '@/services/api';
import { useSession } from '@/store/session';
import { useUi } from '@/store/ui';
import { ArrowLeft } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { Link, Outlet, useLocation } from 'react-router-dom';
import { HeaderQuickSettings } from './HeaderQuickSettings';
import { HeaderUser } from './HeaderUser';
import { Sidebar } from './Sidebar';

export function AppLayout() {
  const location = useLocation();
  const isHome = location.pathname === '/' || location.pathname.startsWith('/agent/sessions/');
  const { tr } = useI18n();
  const initialState = useSession((state) => state.initialState);
  const token = useSession((state) => state.token);
  const setInitialState = useSession((state) => state.setInitialState);
  const clear = useSession((state) => state.clear);
  const sidebarCollapsed = useUi((state) => state.sidebarCollapsed);
  const [loading, setLoading] = useState(!initialState.currentUser);
  const [connectionRetry, setConnectionRetry] = useState(0);

  const pages: Record<string, { title: string; description: string }> = {
    '/dashboard': {
      title: tr('仪表盘', 'Dashboard'),
      description: tr(
        '设备、应用与零信任访问状态',
        'Devices, applications, and zero-trust access status',
      ),
    },
    '/proxy': {
      title: tr('访问', 'Access'),
      description: tr(
        '统一管理应用入口与访问策略',
        'Manage application entry points and access policies',
      ),
    },
    '/resource/device': {
      title: tr('设备', 'Devices'),
      description: tr(
        '查看接入网络的终端与运行状态',
        'Review connected endpoints and their status',
      ),
    },
    '/resource/app': {
      title: tr('应用', 'Applications'),
      description: tr(
        '按协议管理私有应用与服务',
        'Manage private applications and services by protocol',
      ),
    },
    '/connector': {
      title: tr('连接器', 'Connectors'),
      description: tr(
        '管理私有网络中的安全连接节点',
        'Manage secure connection nodes in private networks',
      ),
    },
    '/logs/management': {
      title: tr('管理日志', 'Management logs'),
      description: tr(
        '查看用户在控制台中的管理操作留痕',
        'Review user control-plane operations',
      ),
    },
    '/logs/audit': {
      title: tr('审计日志', 'Audit logs'),
      description: tr(
        '按协议查看访问会话中的操作记录',
        'Review access-session activity by protocol',
      ),
    },
    '/settings': {
      title: tr('设置', 'Settings'),
      description: tr(
        '管理产品功能与界面偏好',
        'Manage product features and interface preferences',
      ),
    },
    '/users': {
      title: tr('用户管理', 'Users'),
      description: tr(
        '查看账户信息与管理登录安全',
        'Review account information and manage sign-in security',
      ),
    },
  };
  const isAIAccessPage = /^\/ai\/\d+\/?$/.test(location.pathname);
  const page = isAIAccessPage ? { title: tr('LLM 协议', 'LLM protocol'), description: '' } : location.pathname.startsWith('/webdata/')
    ? {
        title: tr('数据访问', 'Data access'),
        description: tr('数据库连接与查询控制台', 'Database connection and query console'),
      }
    : pages[location.pathname];
  const webDataRoute = location.pathname.match(
    /^\/webdata\/(\d+)(?:\/connections\/(\d+))?(?:\/sessions\/[^/]+(?:\/agent\/[^/]+)?)?$/,
  );
  const isWebDataPage = Boolean(webDataRoute) || isAIAccessPage;
  const webDataReturn = (() => {
    if (isAIAccessPage) return { href: '/proxy', label: tr('返回访问', 'Back to access') };
    if (!webDataRoute) return undefined;
    if (webDataRoute[2]) {
      return {
        href: `/webdata/${webDataRoute[1]}${location.search}`,
        label: tr('返回连接', 'Back to connections'),
      };
    }
    const from = new URLSearchParams(location.search).get('from') || '';
    return {
      href: from.startsWith('/proxy') && !from.startsWith('//') ? from : '/proxy',
      label: tr('返回访问', 'Back to access'),
    };
  })();

  const fetchUserInfo = useCallback(async () => {
    const response = await getCurrentUser();
    if (response.code === 200) return response.data;
    if (response.code === 401) return undefined;
    throw new Error(response.message || 'Unable to load the current user');
  }, []);

  useEffect(() => {
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let cancelled = false;
    void setInitialState((state) => ({ ...state, fetchUserInfo }));
    if (initialState.currentUser) return;
    if (!token) {
      setLoading(false);
      return;
    }
    setLoading(true);
    fetchUserInfo()
      .then((currentUser) => {
        if (cancelled) return;
        if (!currentUser) {
          clear();
          setLoading(false);
          return;
        }
        void setInitialState((state) => ({
          ...state,
          currentUser,
          fetchUserInfo,
        }));
        setLoading(false);
      })
      .catch((error: { response?: { status?: number } }) => {
        if (cancelled) return;
        if (error?.response?.status === 401) {
          clear();
          setLoading(false);
          return;
        }
        retryTimer = setTimeout(
          () => setConnectionRetry((attempt) => attempt + 1),
          1500,
        );
      });
    return () => {
      cancelled = true;
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, [
    clear,
    connectionRetry,
    fetchUserInfo,
    initialState.currentUser,
    setInitialState,
    token,
  ]);

  useEffect(() => {
    document.title = `${location.pathname === '/' || location.pathname.startsWith('/agent/sessions/') ? tr('首页', 'Home') : page?.title || 'Liaison'} · Liaison`;
  }, [page?.title, location.pathname, tr]);

  return (
    <div className={`liaison-app-frame${isHome ? ' is-home' : ''}${sidebarCollapsed ? ' is-sidebar-collapsed' : ''}`}>
      <header className="liaison-global-header">
        <div className={`liaison-global-left${sidebarCollapsed ? ' is-collapsed' : ''}`}>
          <Link to="/" className="liaison-brand-link">
            <span className="liaison-brand-mark">
              <LiaisonLogo size={30} />
            </span>
            {!sidebarCollapsed && <span className="liaison-brand-copy">
              <strong>Liaison</strong>
              <small>ZERO TRUST APPLICATION ACCESS</small>
            </span>}
          </Link>
        </div>
        {!isHome && <div className="liaison-global-actions">
          <HeaderQuickSettings />
          <HeaderUser />
        </div>}
      </header>
      <div className="liaison-shell">
        <Sidebar />
        <main className="liaison-workspace">
          {page && (
            <header
              className={`liaison-page-header${isWebDataPage ? ' is-context-only' : ''}`}
            >
              <div className="liaison-page-header-copy">
                {webDataReturn && (
                  <Link className="liaison-page-back" to={webDataReturn.href}>
                    <ArrowLeft size={13} />
                    {webDataReturn.label}
                  </Link>
                )}
                {!isWebDataPage && (
                  <>
                    <h1>{page.title}</h1>
                    <p>{page.description}</p>
                  </>
                )}
              </div>
            </header>
          )}
          <div className="liaison-page-body">
            {loading ? (
              <div className="liaison-loading">
                <span />
                {tr('正在连接控制平面…', 'Connecting to control plane…')}
              </div>
            ) : (
              <Outlet />
            )}
          </div>
        </main>
      </div>
    </div>
  );
}
