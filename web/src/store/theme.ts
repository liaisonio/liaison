import { useCallback, useEffect, useState } from 'react';

export type ThemePreference = 'system' | 'light' | 'dark';
export type ResolvedTheme = 'light' | 'dark';

const KEY = 'liaison-theme-preference';
const EVENT = 'liaison-theme-change';

export function getThemePreference(): ThemePreference {
  const value = localStorage.getItem(KEY);
  return value === 'light' || value === 'dark' || value === 'system'
    ? value
    : 'system';
}

export function resolveTheme(preference: ThemePreference): ResolvedTheme {
  if (preference !== 'system') return preference;
  return window.matchMedia('(prefers-color-scheme: dark)').matches
    ? 'dark'
    : 'light';
}

export function applyTheme(theme: ResolvedTheme) {
  const root = document.documentElement;
  root.classList.toggle('dark', theme === 'dark');
  root.classList.toggle('light', theme === 'light');
  root.dataset.theme = theme;
  root.style.colorScheme = theme;
}

export function setThemePreference(preference: ThemePreference) {
  localStorage.setItem(KEY, preference);
  applyTheme(resolveTheme(preference));
  window.dispatchEvent(
    new CustomEvent<ThemePreference>(EVENT, { detail: preference }),
  );
}

export function applyThemeOnBoot() {
  applyTheme(resolveTheme(getThemePreference()));
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (getThemePreference() === 'system') applyTheme(resolveTheme('system'));
  });
}

export function useThemeMode() {
  const [preference, setPreferenceState] =
    useState<ThemePreference>(getThemePreference);

  useEffect(() => {
    const listener = (event: Event) => {
      const detail = (event as CustomEvent<ThemePreference>).detail;
      setPreferenceState(detail || getThemePreference());
    };
    window.addEventListener(EVENT, listener);
    return () => window.removeEventListener(EVENT, listener);
  }, []);

  const setPreference = useCallback(setThemePreference, []);
  const cycle = useCallback(() => {
    const order: ThemePreference[] = ['system', 'light', 'dark'];
    setThemePreference(order[(order.indexOf(preference) + 1) % order.length]);
  }, [preference]);

  return {
    preference,
    resolved: resolveTheme(preference),
    setPreference,
    cycle,
  };
}

