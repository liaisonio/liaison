import { useI18n } from '@/i18n';
import { useThemeMode } from '@/store/theme';
import { App as AntdApp, ConfigProvider, theme } from 'antd';
import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import type { ReactNode } from 'react';

export function AppThemeProvider({ children }: { children: ReactNode }) {
  const { locale } = useI18n();
  const { resolved } = useThemeMode();
  const dark = resolved === 'dark';

  return (
    <ConfigProvider
      locale={locale === 'en-US' ? enUS : zhCN}
      theme={{
        algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: dark ? '#f4f4f5' : '#111827',
          colorPrimaryHover: dark ? '#d4d4d8' : '#273449',
          colorTextLightSolid: dark ? '#0b0b11' : '#ffffff',
          colorInfo: dark ? '#30a6d0' : '#0e7490',
          colorSuccess: '#10b981',
          colorWarning: '#f59e0b',
          colorError: '#ef4444',
          colorBgBase: dark ? '#0b0b11' : '#f4f5f8',
          colorBgContainer: dark ? '#171720' : '#ffffff',
          colorBorder: dark ? '#282832' : '#d1d5db',
          controlHeight: 32,
          borderRadius: 6,
          borderRadiusLG: 10,
          boxShadowSecondary: dark
            ? '0 16px 48px rgba(0, 0, 0, 0.36)'
            : '0 16px 48px rgba(31, 42, 68, 0.12)',
          fontFamily:
            '"IBM Plex Sans", "Segoe UI Variable", "PingFang SC", sans-serif',
        },
        components: {
          Button: { controlHeight: 32, fontWeight: 600 },
          Card: { headerFontSize: 14 },
          Table: {
            headerBg: dark ? '#111117' : '#f8fafc',
            rowHoverBg: dark ? '#202029' : '#f8fafc',
            borderColor: dark ? '#282832' : '#e2e8f0',
          },
          Modal: { contentBg: dark ? '#171720' : '#ffffff' },
          Tabs: { itemSelectedColor: dark ? '#f4f4f5' : '#111827' },
        },
      }}
    >
      <AntdApp message={{ maxCount: 3 }}>{children}</AntdApp>
    </ConfigProvider>
  );
}
