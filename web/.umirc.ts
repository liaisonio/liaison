import { defineConfig } from '@umijs/max';

export default defineConfig({
  antd: {
    appConfig: {
      message: {
        maxCount: 3,
      },
    }, // 启用 App 组件以支持动态主题
  },
  access: {},
  model: {},
  initialState: {},
  request: {
    dataField: 'data',
  },
  esbuildMinifyIIFE: true,
  layout: {
    title: 'Liaison',
  },
  tailwindcss: {},
  proxy: {
    '/api': {
      target: 'https://49.232.250.11:443',
      changeOrigin: true,
      secure: false, // 如果是 HTTP，设置为 false
    },
  },
  routes: [
    {
      path: '/login',
      component: './Login',
      layout: false,
    },
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      name: 'Dashboard',
      path: '/dashboard',
      icon: 'DashboardOutlined',
      component: './Dashboard',
    },
    {
      name: '访问',
      path: '/proxy',
      icon: 'CloudServerOutlined',
      component: './Proxy',
    },
    {
      name: '设备/应用',
      path: '/resource',
      icon: 'AppstoreOutlined',
      routes: [
        {
          name: '设备',
          path: '/resource/device',
          component: './Device',
        },
        {
          name: '应用',
          path: '/resource/app',
          component: './App',
        },
      ],
    },
    {
      name: '连接器',
      path: '/connector',
      icon: 'ApiOutlined',
      component: './Connector',
    },
    {
      name: '设置',
      path: '/settings',
      icon: 'SettingOutlined',
      component: './Settings',
    },
  ],
  npmClient: 'pnpm',
});
