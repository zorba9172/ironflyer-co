import { useState } from 'react';
import { Avatar, Box, Divider, ListItemIcon, Menu, MenuItem, Switch, Typography } from '@mui/material';
import { useAuth } from '@ironflyer/data';
import { useThemeMode } from '@ironflyer/ui-web';
import { toast } from '@ironflyer/ui-web/fx';

const icon = (d: string) => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d={d} /></svg>
);

// Avatar → dropdown with account info, theme, preferences, and sign out.
export function AccountMenu({ size = 28 }: { size?: number }) {
  const { user, signOut } = useAuth();
  const { mode, toggle } = useThemeMode();
  const [anchor, setAnchor] = useState<null | HTMLElement>(null);
  const label = user?.email ?? 'Guest';

  return (
    <>
      <Avatar
        onClick={(e) => setAnchor(e.currentTarget)}
        sx={{ width: size, height: size, fontSize: size * 0.45, bgcolor: 'action.selected', color: 'text.primary', cursor: 'pointer' }}
      >
        {label[0]?.toUpperCase()}
      </Avatar>

      <Menu
        anchorEl={anchor}
        open={!!anchor}
        onClose={() => setAnchor(null)}
        transformOrigin={{ horizontal: 'right', vertical: 'top' }}
        anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
        slotProps={{ paper: { sx: { width: 248, mt: 1, border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}
      >
        <Box sx={{ px: 2, py: 1 }}>
          <Typography sx={{ fontWeight: 600, fontSize: '0.9rem' }} noWrap>{label}</Typography>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled' })}>{user ? `${user.plan ?? 'free'} plan` : 'offline preview'}</Typography>
        </Box>
        <Divider />

        <MenuItem onClick={toggle}>
          <ListItemIcon sx={{ color: 'text.secondary' }}>{mode === 'dark' ? icon('M12 2v2M12 20v2M4.9 4.9l1.4 1.4M2 12h2M20 12h2M5 19l1.4-1.4M12 8a4 4 0 100 8 4 4 0 000-8z') : icon('M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z')}</ListItemIcon>
          <Typography sx={{ flex: 1, fontSize: '0.9rem' }}>{mode === 'dark' ? 'Light theme' : 'Dark theme'}</Typography>
          <Switch size="small" checked={mode === 'light'} />
        </MenuItem>

        <MenuItem onClick={() => { setAnchor(null); toast('Account preferences are coming soon.', 'info'); }}>
          <ListItemIcon sx={{ color: 'text.secondary' }}>{icon('M12 15a3 3 0 100-6 3 3 0 000 6zM19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-2.91.66 2 2 0 11-3.98 0 1.65 1.65 0 00-2.91-.66l-.06.06a2 2 0 11-2.83-2.83l.06-.06A1.65 1.65 0 004.6 15a2 2 0 110-4 1.65 1.65 0 001.51-1 1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H12a2 2 0 110 0z')}</ListItemIcon>
          <Typography sx={{ fontSize: '0.9rem' }}>Account preferences</Typography>
        </MenuItem>

        <MenuItem onClick={() => { setAnchor(null); toast('Billing & wallet are coming soon.', 'info'); }}>
          <ListItemIcon sx={{ color: 'text.secondary' }}>{icon('M3 10h18M3 6h18v12H3zM7 15h2')}</ListItemIcon>
          <Typography sx={{ fontSize: '0.9rem' }}>Billing & wallet</Typography>
        </MenuItem>

        <Divider />
        {user && (
          <MenuItem onClick={() => { setAnchor(null); void signOut(); }}>
            <ListItemIcon sx={{ color: 'error.main' }}>{icon('M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4M16 17l5-5-5-5M21 12H9')}</ListItemIcon>
            <Typography sx={{ fontSize: '0.9rem', color: 'error.main' }}>Sign out</Typography>
          </MenuItem>
        )}
      </Menu>
    </>
  );
}
