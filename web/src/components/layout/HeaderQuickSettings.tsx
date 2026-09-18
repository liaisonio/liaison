import { useI18n } from '@/i18n';
import { useThemeMode } from '@/store/theme';
import { Check, Globe, Moon, Sun } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

export function HeaderQuickSettings({ languageOnly = false }: { languageOnly?: boolean }) {
  const { locale, setLocale, tr } = useI18n();
  const { resolved, setPreference } = useThemeMode();
  const [languageOpen, setLanguageOpen] = useState(false);
  const languageRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!languageOpen) return;
    const close = (event: MouseEvent) => {
      if (!languageRef.current?.contains(event.target as Node)) setLanguageOpen(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setLanguageOpen(false);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', escape);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', escape);
    };
  }, [languageOpen]);

  return (
    <div className="liaison-header-quick-settings">
      <div
        className="liaison-language-picker"
        ref={languageRef}
        onMouseEnter={() => setLanguageOpen(true)}
        onMouseLeave={() => setLanguageOpen(false)}
        onBlurCapture={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setLanguageOpen(false);
        }}
      >
        <button
          type="button"
          className="liaison-header-language-button"
          aria-label={tr('选择语言', 'Select language')}
          aria-expanded={languageOpen}
          onClick={() => setLanguageOpen(true)}
        >
          <Globe size={16} />
        </button>
        {languageOpen ? (
          <div className="liaison-language-menu">
            {(['en-US', 'zh-CN'] as const).map(value => <button key={value} type="button" aria-pressed={locale === value} onClick={() => { setLocale(value); setLanguageOpen(false); languageRef.current?.querySelector('button')?.focus(); }}><span>{value === 'en-US' ? 'English' : '简体中文'}</span>{locale === value && <Check size={14} />}</button>)}
          </div>
        ) : null}
      </div>

      {!languageOnly && <button
        type="button"
        className={`liaison-theme-toggle${resolved === 'dark' ? ' is-dark' : ''}`}
        role="switch"
        aria-label={tr('切换主题', 'Switch theme')}
        aria-checked={resolved === 'dark'}
        title={resolved === 'light' ? tr('切换至深色模式', 'Switch to dark mode') : tr('切换至浅色模式', 'Switch to light mode')}
        onClick={() => setPreference(resolved === 'light' ? 'dark' : 'light')}
      >
        <span>
          <Sun className="is-sun" size={12} />
          <Moon className="is-moon" size={12} />
        </span>
      </button>}
    </div>
  );
}
