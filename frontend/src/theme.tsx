import { createContext, type ReactNode, useContext, useEffect, useMemo, useState } from 'react';

export type ThemePreference = 'system' | 'dark' | 'light';
export type EffectiveTheme = 'dark' | 'light';

const themeStorageKey = 'nodebridge.theme';

const themeNames: Record<ThemePreference, string> = {
  system: 'system',
  dark: 'dark',
  light: 'light',
};

function isThemePreference(value: string | null): value is ThemePreference {
  return value === 'system' || value === 'dark' || value === 'light';
}

function readThemePreference(): ThemePreference {
  if (typeof window === 'undefined') return 'system';
  try {
    const saved = window.localStorage.getItem(themeStorageKey);
    return isThemePreference(saved) ? saved : 'system';
  } catch {
    return 'system';
  }
}

function systemTheme(): EffectiveTheme {
  if (typeof window === 'undefined') return 'dark';
  return window.matchMedia?.('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
}

function resolveTheme(preference: ThemePreference): EffectiveTheme {
  return preference === 'system' ? systemTheme() : preference;
}

type ThemeContextValue = {
  effectiveTheme: EffectiveTheme;
  theme: ThemePreference;
  themeNames: Record<ThemePreference, string>;
  setTheme: (theme: ThemePreference) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemePreference>(readThemePreference);
  const [effectiveTheme, setEffectiveTheme] = useState<EffectiveTheme>(() => resolveTheme(readThemePreference()));

  useEffect(() => {
    const media = window.matchMedia?.('(prefers-color-scheme: light)');
    const update = () => setEffectiveTheme(resolveTheme(theme));
    update();
    if (theme === 'system' && media) {
      media.addEventListener('change', update);
      return () => media.removeEventListener('change', update);
    }
    return undefined;
  }, [theme]);

  useEffect(() => {
    document.documentElement.dataset.theme = effectiveTheme;
    document.documentElement.style.colorScheme = effectiveTheme;
  }, [effectiveTheme]);

  function setTheme(nextTheme: ThemePreference) {
    setThemeState(nextTheme);
    try {
      window.localStorage.setItem(themeStorageKey, nextTheme);
    } catch {
      // Local storage is best effort in embedded webviews.
    }
  }

  const value = useMemo<ThemeContextValue>(
    () => ({ effectiveTheme, theme, themeNames, setTheme }),
    [effectiveTheme, theme],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error('ThemeProvider is required');
  }
  return context;
}
