import { useCallback, useEffect, useState } from 'react';

export type ThemePreference = 'system' | 'light' | 'dark';
export type ResolvedTheme = 'light' | 'dark';

export type AccentPreset = {
  id: string;
  zh: string;
  en: string;
  rgb: string;
  foreground: string;
  hex: string;
};

export const ACCENT_PRESETS: AccentPreset[] = [
  { id: 'brand-purple', zh: '品牌紫', en: 'Brand purple', rgb: '140 109 240', foreground: '255 255 255', hex: '#8C6DF0' },
  { id: 'rose', zh: '玫粉', en: 'Rose', rgb: '241 91 199', foreground: '255 255 255', hex: '#F15BC7' },
  { id: 'royal-blue', zh: '海蓝', en: 'Royal blue', rgb: '82 105 244', foreground: '255 255 255', hex: '#5269F4' },
  { id: 'cyan', zh: '青色', en: 'Cyan', rgb: '48 166 208', foreground: '255 255 255', hex: '#30A6D0' },
  { id: 'teal', zh: '蓝绿', en: 'Teal', rgb: '32 172 176', foreground: '255 255 255', hex: '#20ACB0' },
  { id: 'emerald', zh: '翡翠', en: 'Emerald', rgb: '16 185 129', foreground: '255 255 255', hex: '#10B981' },
];

const KEY = 'liaison-theme-preference';
const EVENT = 'liaison-theme-change';
const ACCENT_KEY = 'liaison-accent-preference';
const ACCENT_EVENT = 'liaison-accent-change';

export function getAccentPreference() {
  const value = localStorage.getItem(ACCENT_KEY);
  return ACCENT_PRESETS.some((preset) => preset.id === value)
    ? value as string
    : ACCENT_PRESETS[0].id;
}

export function applyAccent(id: string) {
  const preset = ACCENT_PRESETS.find((item) => item.id === id) || ACCENT_PRESETS[0];
  document.documentElement.style.setProperty('--accent', preset.rgb);
  document.documentElement.style.setProperty('--accent-ink', preset.foreground);
  document.documentElement.dataset.accent = preset.id;
}

export function setAccentPreference(id: string) {
  if (!ACCENT_PRESETS.some((preset) => preset.id === id)) return;
  localStorage.setItem(ACCENT_KEY, id);
  applyAccent(id);
  window.dispatchEvent(new CustomEvent<string>(ACCENT_EVENT, { detail: id }));
}

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
  applyAccent(getAccentPreference());
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (getThemePreference() === 'system') applyTheme(resolveTheme('system'));
  });
}

export function useAccentColor() {
  const [accentId, setAccentId] = useState(getAccentPreference);

  useEffect(() => {
    const listener = (event: Event) => {
      setAccentId((event as CustomEvent<string>).detail || getAccentPreference());
    };
    window.addEventListener(ACCENT_EVENT, listener);
    return () => window.removeEventListener(ACCENT_EVENT, listener);
  }, []);

  return { accentId, setAccentId: setAccentPreference };
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
