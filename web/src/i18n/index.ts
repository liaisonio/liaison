import { useCallback, useEffect, useMemo, useState } from 'react';

export type LiaisonLocale = 'zh-CN' | 'en-US';

const LOCALE_STORAGE_KEY = 'liaison-locale';
const LOCALE_CHANGE_EVENT = 'liaison-locale-change';

export const getLocale = (): LiaisonLocale => {
  const locale = localStorage.getItem(LOCALE_STORAGE_KEY);
  return locale === 'en-US' ? 'en-US' : 'zh-CN';
};

export const setLocale = (locale: LiaisonLocale) => {
  localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  window.dispatchEvent(new CustomEvent<LiaisonLocale>(LOCALE_CHANGE_EVENT, { detail: locale }));
};

export const tr = (zh: string, en: string) => (getLocale() === 'en-US' ? en : zh);

export const useI18n = () => {
  const [locale, setLocaleState] = useState<LiaisonLocale>(getLocale());

  useEffect(() => {
    const listener = (event: Event) => {
      const customEvent = event as CustomEvent<LiaisonLocale>;
      setLocaleState(customEvent.detail || getLocale());
    };
    window.addEventListener(LOCALE_CHANGE_EVENT, listener as EventListener);
    return () => window.removeEventListener(LOCALE_CHANGE_EVENT, listener as EventListener);
  }, []);

  const toggleLocale = useCallback(() => {
    setLocale(locale === 'zh-CN' ? 'en-US' : 'zh-CN');
  }, [locale]);

  const translator = useMemo(() => {
    return (zh: string, en: string) => (locale === 'en-US' ? en : zh);
  }, [locale]);

  return {
    locale,
    tr: translator,
    toggleLocale,
    setLocale,
  };
};
