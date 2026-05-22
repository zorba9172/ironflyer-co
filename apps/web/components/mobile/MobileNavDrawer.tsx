'use client';

/**
 * MobileNavDrawer — bottom-sheet navigation for small screens.
 *
 * The desktop workspace shell (apps/web/components/workspace/) is owned
 * by another agent and we do NOT modify it here. Instead, this drawer
 * is a drop-in mobile mirror of the same nav items. The integrator
 * passes the same items they would render in the desktop sidebar.
 *
 * To wire (when a sibling agent owns the call-site):
 *   1. import { MobileNavDrawer } from '@/components/mobile/MobileNavDrawer';
 *   2. import { useBreakpoint } from '@/components/responsive/useBreakpoint';
 *   3. const { isMobile } = useBreakpoint();
 *   4. render a hamburger that toggles a local `open` state on mobile,
 *      then `<MobileNavDrawer items={navItems} open={open} onClose={...} />`.
 *
 * The drawer stays fully self-contained — no global store, no router
 * coupling, no theme overrides. Pass an `onNavigate` to bridge the
 * caller's router (Next's `useRouter().push`, for example).
 */
import { useState, type ReactNode } from 'react';
import {
  Drawer,
  IconButton,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Box,
  Typography,
  Divider,
} from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';
import CloseIcon from '@mui/icons-material/Close';

export type MobileNavItem = {
  id: string;
  label: string;
  href?: string;
  icon?: ReactNode;
  /** When provided, called instead of navigating to `href`. */
  onSelect?: () => void;
  /** Optional visual emphasis — e.g. the active workspace. */
  active?: boolean;
};

export type MobileNavDrawerProps = {
  items: MobileNavItem[];
  /** Optional controlled state. When omitted, the drawer manages its own. */
  open?: boolean;
  onClose?: () => void;
  /** Header label shown at the top of the sheet. */
  title?: string;
  /** Router bridge; receives the item's href when no custom onSelect set. */
  onNavigate?: (href: string) => void;
  /** Hide the built-in hamburger trigger when the host renders its own. */
  hideTrigger?: boolean;
};

export function MobileNavDrawer({
  items,
  open: controlledOpen,
  onClose,
  title = 'Ironflyer',
  onNavigate,
  hideTrigger = false,
}: MobileNavDrawerProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isControlled = typeof controlledOpen === 'boolean';
  const open = isControlled ? controlledOpen : internalOpen;

  const close = () => {
    if (isControlled) onClose?.();
    else setInternalOpen(false);
  };

  const handleSelect = (item: MobileNavItem) => {
    if (item.onSelect) {
      item.onSelect();
    } else if (item.href && onNavigate) {
      onNavigate(item.href);
    } else if (item.href && typeof window !== 'undefined') {
      window.location.href = item.href;
    }
    close();
  };

  return (
    <>
      {!hideTrigger && !isControlled && (
        <IconButton
          aria-label="open navigation"
          onClick={() => setInternalOpen(true)}
          className="tap-target"
          sx={{ color: '#0d0e0f' }}
        >
          <MenuIcon />
        </IconButton>
      )}
      <Drawer
        anchor="bottom"
        open={open}
        onClose={close}
        PaperProps={{
          className: 'safe-area-bottom',
          sx: {
            background: '#f4f0e8',
            color: '#0d0e0f',
            borderTopLeftRadius: 18,
            borderTopRightRadius: 18,
            maxHeight: '85vh',
            pt: 1,
          },
        }}
      >
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            px: 2,
            pt: 1,
            pb: 0.5,
          }}
        >
          <Box
            aria-hidden
            sx={{
              position: 'absolute',
              top: 8,
              left: '50%',
              transform: 'translateX(-50%)',
              width: 36,
              height: 4,
              borderRadius: 999,
              background: 'rgba(13, 14, 15, 0.2)',
            }}
          />
          <Typography
            variant="subtitle1"
            sx={{ fontFamily: 'var(--font-display)', letterSpacing: 0.5, mt: 1 }}
          >
            {title}
          </Typography>
          <IconButton aria-label="close navigation" onClick={close} className="tap-target">
            <CloseIcon />
          </IconButton>
        </Box>
        <Divider sx={{ borderColor: 'rgba(13, 14, 15, 0.08)' }} />
        <List sx={{ py: 1 }}>
          {items.map((item) => (
            <ListItemButton
              key={item.id}
              onClick={() => handleSelect(item)}
              selected={!!item.active}
              sx={{
                minHeight: 'var(--tap-target)',
                px: 2,
                gap: 1.5,
                '&.Mui-selected': {
                  background: 'rgba(229, 255, 0, 0.18)',
                },
                '&.Mui-selected:hover': {
                  background: 'rgba(229, 255, 0, 0.26)',
                },
              }}
            >
              {item.icon && (
                <ListItemIcon sx={{ minWidth: 32, color: '#0d0e0f' }}>{item.icon}</ListItemIcon>
              )}
              <ListItemText
                primary={item.label}
                primaryTypographyProps={{ fontSize: '1.0625rem', fontWeight: 500 }}
              />
            </ListItemButton>
          ))}
        </List>
      </Drawer>
    </>
  );
}
