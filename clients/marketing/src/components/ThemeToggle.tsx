import { IconButton } from '@mui/material';
import { useThemeMode } from '@ironflyer/ui-web';

export function ThemeToggle() {
  const { mode, toggle } = useThemeMode();
  return (
    <IconButton
      onClick={toggle}
      size="small"
      aria-label={`Switch to ${mode === 'dark' ? 'light' : 'dark'} theme`}
      sx={{ border: 1, borderColor: 'divider', borderRadius: 1.5, color: 'text.secondary' }}
    >
      {mode === 'dark' ? (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
        </svg>
      ) : (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8z" />
        </svg>
      )}
    </IconButton>
  );
}
