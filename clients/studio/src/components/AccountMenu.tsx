import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Avatar, Box, Divider, ListItemIcon, Menu, MenuItem, Switch, Typography } from '@mui/material';
import { useAuth } from '@ironflyer/data';
import { Icon } from '../icons';
import { useThemeMode } from '../theme';
import { text } from '@ironflyer/design-tokens/brand';

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
          <ListItemIcon sx={{ color: 'text.secondary' }}><Icon name={mode === 'dark' ? 'sun' : 'moon'} size={17} /></ListItemIcon>
          <Typography sx={{ flex: 1, fontSize: text.s90 }}>{mode === 'dark' ? 'Light theme' : 'Dark theme'}</Typography>
          <Switch size="small" checked={mode === 'light'} />
        </MenuItem>

        <MenuItem onClick={() => { setAnchor(null); navigate('/plans'); }}>
          <ListItemIcon sx={{ color: 'text.secondary' }}><Icon name="wallet" size={17} /></ListItemIcon>
          <Typography sx={{ fontSize: text.s90 }}>Billing & wallet</Typography>
        </MenuItem>

        <Divider />
        {user && (
          <MenuItem onClick={() => { setAnchor(null); void signOut(); }}>
            <ListItemIcon sx={{ color: 'error.main' }}><Icon name="external" size={17} /></ListItemIcon>
            <Typography sx={{ fontSize: text.s90, color: 'error.main' }}>Sign out</Typography>
          </MenuItem>
        )}
      </Menu>
    </>
  );
}
