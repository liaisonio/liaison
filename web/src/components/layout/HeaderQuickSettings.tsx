import { useI18n } from '@/i18n';
import { useThemeMode } from '@/store/theme';
import { Languages, Moon, Sun } from 'lucide-react';

export function HeaderQuickSettings() {
  const { locale, setLocale, tr } = useI18n();
  const { resolved, setPreference } = useThemeMode();

  return (
    <div className="liaison-header-quick-settings">
      <div
        className="liaison-header-switch"
        aria-label={tr('切换语言', 'Switch language')}
      >
        <Languages size={13} />
        <button
          type="button"
          className={locale === 'zh-CN' ? 'is-active' : undefined}
          aria-pressed={locale === 'zh-CN'}
          onClick={() => setLocale('zh-CN')}
        >
          中文
        </button>
        <button
          type="button"
          className={locale === 'en-US' ? 'is-active' : undefined}
          aria-pressed={locale === 'en-US'}
          onClick={() => setLocale('en-US')}
        >
          EN
        </button>
      </div>

      <div
        className="liaison-header-switch liaison-theme-switch"
        aria-label={tr('切换主题', 'Switch theme')}
      >
        <button
          type="button"
          className={resolved === 'light' ? 'is-active' : undefined}
          aria-pressed={resolved === 'light'}
          title={tr('浅色模式', 'Light mode')}
          onClick={() => setPreference('light')}
        >
          <Sun size={13} />
          <span>Light</span>
        </button>
        <button
          type="button"
          className={resolved === 'dark' ? 'is-active' : undefined}
          aria-pressed={resolved === 'dark'}
          title={tr('深色模式', 'Dark mode')}
          onClick={() => setPreference('dark')}
        >
          <Moon size={13} />
          <span>Dark</span>
        </button>
      </div>
    </div>
  );
}
