import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Avatar, Box, Divider, ListItemIcon, Menu, MenuItem, Switch, Typography } from '@mui/material';
import { useAuth } from '@ironflyer/data';
import { useThemeMode } from '../theme';
import { text } from '@ironflyer/design-tokens/brand';

const icon = (d: string) => (
  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d={d} /></svg>
);

// Avatar → dropdown with account info, theme, preferences, and sign out.
export function AccountMenu({ size = 28 }: { size?: number }) {
  const { user, signOut } = useAuth();
  const { mode, toggle } = useThemeMode();
  const navigate = useNavigate();
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
          <Typography sx={{ fontWeight: 600, fontSize: text.s90 }} noWrap>{label}</Typography>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s70, color: 'text.disabled' })}>{user ? `${user.plan ?? 'free'} plan` : 'offline preview'}</Typography>
        </Box>
        <Divider />

        <MenuItem onClick={toggle}>
          <ListItemIcon sx={{ color: 'text.secondary' }}>{mode === 'dark' ? icon('M12 2v2M12 20v2M4.9 4.9l1.4 1.4M2 12h2M20 12h2M5 19l1.4-1.4M12 8a4 4 0 100 8 4 4 0 000-8z') : icon('M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z')}</ListItemIcon>
          <Typography sx={{ flex: 1, fontSize: text.s90 }}>{mode === 'dark' ? 'Light theme' : 'Dark theme'}</Typography>
          <Switch size="small" checked={mode === 'light'} />
        </MenuItem>

        <MenuItem onClick={() => { setAnchor(null); navigate('/plans'); }}>
          <ListItemIcon sx={{ color: 'text.secondary' }}>{icon('M3 10h18M3 6h18v12H3zM7 15h2')}</ListItemIcon>
          <Typography sx={{ fontSize: text.s90 }}>Billing & wallet</Typography>
        </MenuItem>

        <Divider />
        {user && (
          <MenuItem onClick={() => { setAnchor(null); void signOut(); }}>
            <ListItemIcon sx={{ color: 'error.main' }}>{icon('M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4M16 17l5-5-5-5M21 12H9')}</ListItemIcon>
            <Typography sx={{ fontSize: text.s90, color: 'error.main' }}>Sign out</Typography>
          </MenuItem>
        )}
      </Menu>
    </>
  );
}
