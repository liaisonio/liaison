import { LiaisonLogo } from '@/components/LiaisonLogo';
import { useI18n } from '@/i18n';
import { getCurrentUser } from '@/services/api';
import { useSession } from '@/store/session';
import { useCallback, useEffect, useState } from 'react';
import { Link, Outlet, useLocation } from 'react-router-dom';
import { HeaderQuickSettings } from './HeaderQuickSettings';
import { HeaderUser } from './HeaderUser';
import { Sidebar } from './Sidebar';

export function AppLayout() {
  const location = useLocation();
  const { tr } = useI18n();
  const initialState = useSession((state) => state.initialState);
  const token = useSession((state) => state.token);
  const setInitialState = useSession((state) => state.setInitialState);
  const clear = useSession((state) => state.clear);
  const [loading, setLoading] = useState(!initialState.currentUser);
  const [connectionRetry, setConnectionRetry] = useState(0);

  const pages: Record<string, { title: string; description: string }> = {
    '/dashboard': {
      title: tr('总览', 'Overview'),
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
    '/audit': {
      title: tr('日志与审计', 'Logs & Audit'),
      description: tr(
        '统一查看访问记录与关键操作留痕',
        'Review access records and critical operations',
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
  const page = pages[location.pathname];

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
    document.title = `${page?.title || 'Liaison'} · Liaison`;
  }, [page?.title]);

  return (
    <div className="liaison-app-frame">
      <header className="liaison-global-header">
        <Link to="/dashboard" className="liaison-brand-link">
          <span className="liaison-brand-mark">
            <LiaisonLogo size={30} />
          </span>
          <span className="liaison-brand-copy">
            <strong>Liaison</strong>
            <small>Zero Trust Access</small>
          </span>
        </Link>
        <div className="liaison-global-actions">
          <HeaderQuickSettings />
          <HeaderUser />
        </div>
      </header>
      <div className="liaison-shell">
        <Sidebar />
        <main className="liaison-workspace">
          {page && (
            <header className="liaison-page-header">
              <div className="liaison-page-header-copy">
                <h1>{page.title}</h1>
                <p>{page.description}</p>
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
