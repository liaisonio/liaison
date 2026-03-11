import { history, RequestConfig } from '@umijs/max';
import { ConfigProvider, Dropdown, message, Typography, Tooltip } from 'antd';
import { LogoutOutlined, SettingOutlined, BugOutlined, GlobalOutlined } from '@ant-design/icons';
import React from 'react';
import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import { getCurrentUser, logout } from '@/services/api';
import { APP_NAME, GITHUB_URL, getAvatarUrl } from '@/constants';
import { getLocale, setLocale, tr } from '@/i18n';
import './global.less';

const { Text, Link } = Typography;
const GITHUB_ISSUES_URL = 'https://github.com/singchia/liaison/issues/new';

const localizeMenuName = (path?: string, name?: string) => {
  const map: Record<string, string> = {
    '/dashboard': tr('Dashboard', 'Dashboard'),
    '/proxy': tr('访问', 'Entries'),
    '/resource': tr('设备/应用', 'Devices/Apps'),
    '/resource/device': tr('设备', 'Devices'),
    '/resource/app': tr('应用', 'Applications'),
    '/connector': tr('连接器', 'Edges'),
    '/settings': tr('设置', 'Settings'),
  };
  if (path && map[path]) {
    return map[path];
  }
  if (!name) {
    return name;
  }
  return name;
};

if (process.env.NODE_ENV === 'development') {
  const filterMessages = [
    'findDOMNode is deprecated',
    'Static function can not consume context',
  ];

  const shouldFilter = (args: any[]) => {
    const msg = args[0];
    if (typeof msg === 'string') {
      return filterMessages.some(filter => msg.includes(filter));
    }
    return false;
  };

  const originalWarn = console.warn;
  const originalError = console.error;

  console.warn = (...args) => {
    if (shouldFilter(args)) return;
    originalWarn.apply(console, args);
  };

  console.error = (...args) => {
    if (shouldFilter(args)) return;
    originalError.apply(console, args);
  };
}

export async function getInitialState(): Promise<{
  currentUser?: API.CurrentUser;
  fetchUserInfo?: () => Promise<API.CurrentUser | undefined>;
}> {
  const fetchUserInfo = async () => {
    try {
      const token = localStorage.getItem('token');
      if (!token) {
        return undefined;
      }
      const res = await getCurrentUser();
      // 兼容 dataField 解包与未解包两种返回
      const raw = (res as any)?.data !== undefined ? (res as any).data : res;
      if (!raw || typeof raw !== 'object') return undefined;
      if ((res as any)?.code !== undefined && (res as any).code !== 200) return undefined;
      const avatar = (raw as any).avatar ?? (raw as any).Avatar ?? '';
      return { ...raw, avatar } as API.CurrentUser;
    } catch (error) {
      return undefined;
    }
  };

  const { location } = history;
  if (location.pathname !== '/login') {
    const currentUser = await fetchUserInfo();
    if (!currentUser) {
      history.push('/login');
    }
    return {
      fetchUserInfo,
      currentUser,
    };
  }
  return {
    fetchUserInfo,
  };
}

const handleLogout = async () => {
  try {
    await logout();
  } catch (e) {}
  localStorage.removeItem('token');
  message.success(tr('已退出登录', 'Logged out'));
  history.push('/login');
};

// 布局配置
export const layout = ({ initialState }: any) => {
  const currentLocale = getLocale();
  const antdLocale = currentLocale === 'en-US' ? enUS : zhCN;
  const dropdownMenuItems = [
    {
      key: 'settings',
      icon: React.createElement(SettingOutlined),
      label: tr('个人设置', 'Settings'),
      onClick: () => history.push('/settings'),
    },
    {
      key: 'logout',
      icon: React.createElement(LogoutOutlined),
      label: tr('退出登录', 'Logout'),
      onClick: handleLogout,
    },
    {
      type: 'divider' as const,
    },
    {
      key: 'report',
      icon: React.createElement(BugOutlined),
      label: tr('报告问题', 'Report Issue'),
      onClick: () => {
        window.open(GITHUB_ISSUES_URL, '_blank');
      },
    },
  ];

  return {
    logo: React.createElement('img', {
      src: '/liaison.png',
      alt: 'Liaison',
      style: {
        height: 52,
      },
    }),
    menu: {
      locale: false,
    },
    menuDataRender: (menuData: any[]) => {
      const localizeItems = (items: any[]): any[] =>
        items.map((item) => ({
          ...item,
          name: localizeMenuName(item.path, item.name),
          children: item.children ? localizeItems(item.children) : undefined,
        }));
      return localizeItems(menuData);
    },
    actionsRender: () => [
      React.createElement(
        'div',
        {
          key: 'locale-switch',
          className: 'locale-switch',
          style: {
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
            padding: 3,
            border: '1px solid rgba(0, 0, 0, 0.1)',
            borderRadius: 999,
            background: 'rgba(250, 250, 250, 0.9)',
            boxShadow: '0 1px 2px rgba(0, 0, 0, 0.04)',
          },
        },
        React.createElement(GlobalOutlined, {
          className: 'locale-switch__icon',
          style: {
            fontSize: 12,
            color: 'rgba(0, 0, 0, 0.45)',
            margin: '0 2px 0 4px',
          },
        }),
        React.createElement(
          'span',
          {
            className: `locale-switch__item ${currentLocale === 'zh-CN' ? 'is-active' : ''}`,
            style: {
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: 22,
              minWidth: 36,
              padding: '0 8px',
              borderRadius: 999,
              fontSize: 12,
              fontWeight: 500,
              lineHeight: '22px',
              cursor: currentLocale === 'zh-CN' ? 'default' : 'pointer',
              color: currentLocale === 'zh-CN' ? '#1677ff' : 'rgba(0, 0, 0, 0.65)',
              background: currentLocale === 'zh-CN' ? '#e6f4ff' : 'transparent',
              boxShadow: currentLocale === 'zh-CN' ? 'inset 0 0 0 1px rgba(22, 119, 255, 0.22)' : 'none',
              userSelect: 'none',
            },
            onClick: () => {
              if (currentLocale === 'zh-CN') return;
              setLocale('zh-CN');
              window.location.reload();
            },
          },
          '中文',
        ),
        React.createElement(
          'span',
          {
            className: `locale-switch__item ${currentLocale === 'en-US' ? 'is-active' : ''}`,
            style: {
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: 22,
              minWidth: 36,
              padding: '0 8px',
              borderRadius: 999,
              fontSize: 12,
              fontWeight: 500,
              lineHeight: '22px',
              cursor: currentLocale === 'en-US' ? 'default' : 'pointer',
              color: currentLocale === 'en-US' ? '#1677ff' : 'rgba(0, 0, 0, 0.65)',
              background: currentLocale === 'en-US' ? '#e6f4ff' : 'transparent',
              boxShadow: currentLocale === 'en-US' ? 'inset 0 0 0 1px rgba(22, 119, 255, 0.22)' : 'none',
              userSelect: 'none',
            },
            onClick: () => {
              if (currentLocale === 'en-US') return;
              setLocale('en-US');
              window.location.reload();
            },
          },
          'EN',
        ),
      ),
    ],
    childrenRender: (children: React.ReactNode) =>
      React.createElement(
        ConfigProvider,
        {
          locale: antdLocale,
        },
        children,
      ),
    layout: 'mix',
    splitMenus: false,
    fixedHeader: true,
    fixSiderbar: true,
    navTheme: 'light',
    contentWidth: 'Fluid',
    colorPrimary: '#1677ff', // 更现代的蓝色
    siderWidth: 220,
    title: 'Liaison',
    token: {
      // 现代化的设计 token
      colorBgContainer: '#ffffff',
      colorBgElevated: '#fafafa',
      borderRadius: 8,
      wireframe: false,
      // 更好的间距
      sizeStep: 4,
      sizeUnit: 4,
    },
    avatarProps: {
      src: getAvatarUrl(initialState?.currentUser?.avatar),
      size: 'small',
      title: initialState?.currentUser?.name || 'Admin',
      render: (_props: any, avatarChildren: React.ReactNode) => {
        return React.createElement(
          Dropdown,
          { menu: { items: dropdownMenuItems } },
          avatarChildren
        );
      },
    },
    waterMarkProps: {
      content: 'Liaison',
      fontSize: 14,
      fontColor: 'rgba(0, 0, 0, 0.04)',
      gapX: 120,
      gapY: 120,
      rotate: -22,
      fontStyle: 'normal',
      fontWeight: 'normal',
    },
    footerRender: () => {
      return React.createElement(
        'div',
        {
          className: 'global-footer',
          style: {
            position: 'fixed',
            bottom: 0,
            left: 0,
            right: 0,
            textAlign: 'center',
            padding: '16px 24px',
            backgroundColor: 'transparent',
            zIndex: 100,
            color: 'rgba(0, 0, 0, 0.45)',
          },
        },
        React.createElement(
          'div',
          {
            style: {
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              gap: 8,
              flexWrap: 'wrap',
            },
          },
          React.createElement(Text, { type: 'secondary', style: { fontSize: 13 } }, `© 2026 ${APP_NAME}. All rights reserved.`),
          React.createElement(Text, { type: 'secondary', style: { fontSize: 13 } }, '备案号：浙ICP备2026007675号-1'),
          React.createElement(
            Link,
            {
              href: GITHUB_URL,
              target: '_blank',
              rel: 'noopener noreferrer',
              style: {
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                color: 'rgba(0, 0, 0, 0.45)',
                fontSize: 13,
                transition: 'color 0.3s',
                textDecoration: 'none',
              },
              onMouseEnter: (e: any) => {
                e.currentTarget.style.color = '#1677ff';
              },
              onMouseLeave: (e: any) => {
                e.currentTarget.style.color = 'rgba(0, 0, 0, 0.45)';
              },
            },
            React.createElement('img', {
              src: '/github.svg',
              alt: 'GitHub',
              style: {
                width: 18,
                height: 18,
                verticalAlign: 'middle',
              },
            }),
            React.createElement('span', null, 'Github')
          ),
          React.createElement(
            Tooltip,
            {
              title: React.createElement('img', {
                src: '/wechat.png',
                alt: tr('微信二维码', 'WeChat QR Code'),
                style: {
                  width: 200,
                  height: 200,
                  display: 'block',
                },
              }),
              placement: 'top',
              overlayStyle: {
                padding: 0,
              },
              overlayInnerStyle: {
                padding: 8,
                backgroundColor: '#fff',
                borderRadius: 8,
                boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
              },
            },
            React.createElement(
              'div',
              {
                style: {
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 6,
                  cursor: 'pointer',
                  color: 'rgba(0, 0, 0, 0.45)',
                  fontSize: 13,
                },
              },
              React.createElement('img', {
                src: '/wechat.svg',
                alt: 'WeChat',
                style: {
                  width: 18,
                  height: 18,
                  verticalAlign: 'middle',
                  opacity: 0.7,
                  transition: 'opacity 0.3s',
                },
                onMouseEnter: (e: any) => {
                  e.currentTarget.style.opacity = '1';
                },
                onMouseLeave: (e: any) => {
                  e.currentTarget.style.opacity = '0.7';
                },
              }),
              React.createElement('span', null, 'WeChat')
            )
          )
        )
      );
    },
  };
};

// 请求配置
export const request: RequestConfig = {
  timeout: 30000,
  requestInterceptors: [
    (config: any) => {
      const token = localStorage.getItem('token');
      if (token) {
        config.headers = {
          ...config.headers,
          Authorization: `Bearer ${token}`,
        };
      }
      return config;
    },
  ],
  responseInterceptors: [
    (response: any) => {
      const { data } = response;
      if (data && data.code === 401) {
        localStorage.removeItem('token');
        history.push('/login');
        return response;
      }
      return response;
    },
  ],
  errorConfig: {
    errorHandler: (error: any) => {
      const { response } = error;
      if (response?.status === 401 || response?.status === 403) {
        localStorage.removeItem('token');
        message.error(tr('登录已过期，请重新登录', 'Session expired, please login again'));
        history.push('/login');
      } else if (response?.status === 500) {
        // 检查是否是认证相关的500错误
        const errorMessage = error?.response?.data?.message || error?.message || '';
        if (errorMessage.includes('authentication') || errorMessage.includes('token') || errorMessage.includes('unauthorized')) {
          localStorage.removeItem('token');
          message.error(tr('登录已过期，请重新登录', 'Session expired, please login again'));
          history.push('/login');
        } else {
          message.error(tr('服务器错误', 'Server error'));
        }
      } else if (!response) {
        message.error(tr('网络异常', 'Network error'));
      }
    },
  },
};
