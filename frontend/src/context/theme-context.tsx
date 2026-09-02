import { createContext, useContext, useEffect, useState } from 'react';

type Theme = 'dark' | 'light' | 'system';

type ThemeProviderProps = {
  children: React.ReactNode;
  defaultTheme?: Theme;
  storageKey?: string;
};

type ThemeProviderState = {
  theme: Theme;
  setTheme: (theme: Theme) => void;
};

const initialState: ThemeProviderState = {
  theme: 'system',
  setTheme: () => null,
};

const ThemeProviderContext = createContext<ThemeProviderState>(initialState);

// Color scheme variants were removed in the UI refactor; strip any legacy
// scheme class left in localStorage-driven state from older sessions.
const LEGACY_COLOR_SCHEME_CLASSES = ['blue', 'green', 'purple', 'orange', 'red', 'black', 'cream', 'claude', 'starry'];

export function ThemeProvider({ children, defaultTheme = 'system', storageKey = 'axonhub-ui-theme', ...props }: ThemeProviderProps) {
  const [theme, _setTheme] = useState<Theme>(() => (localStorage.getItem(storageKey) as Theme) || defaultTheme);

  useEffect(() => {
    const root = window.document.documentElement;
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    const applyTheme = (theme: Theme) => {
      root.classList.remove('light', 'dark', ...LEGACY_COLOR_SCHEME_CLASSES);

      const systemTheme = mediaQuery.matches ? 'dark' : 'light';
      const effectiveTheme = theme === 'system' ? systemTheme : theme;

      // Add theme class with transition
      root.style.transition = 'background-color 0.3s ease, color 0.3s ease';
      root.classList.add(effectiveTheme);

      // Remove transition after animation completes
      setTimeout(() => {
        root.style.transition = '';
      }, 300);
    };

    const handleChange = () => {
      if (theme === 'system') {
        applyTheme('system');
      }
    };

    applyTheme(theme);

    mediaQuery.addEventListener('change', handleChange);

    return () => mediaQuery.removeEventListener('change', handleChange);
  }, [theme]);

  const setTheme = (theme: Theme) => {
    localStorage.setItem(storageKey, theme);
    _setTheme(theme);
  };

  const value = {
    theme,
    setTheme,
  };

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export const useTheme = () => {
  const context = useContext(ThemeProviderContext);

  if (context === undefined) throw new Error('useTheme must be used within a ThemeProvider');

  return context;
};
