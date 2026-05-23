'use client';

/**
 * PWAInstaller — install-prompt banner + service-worker handshake.
 *
 * Responsibilities:
 *  - Register `/sw.js` on mount (no-op when SW is unsupported or another
 *    registrar already claimed it; we tolerate both `pwa-register.tsx`
 *    and this file running side by side).
 *  - Listen for `beforeinstallprompt`, stash the event, and after 15
 *    seconds on a first-time visit, surface an install CTA banner.
 *  - Persist dismissal in `localStorage` so the banner does not nag.
 *
 * Visual: alabaster sheet, lime CTA, slides up from bottom.
 */
import { useEffect, useRef, useState } from 'react';
import { Box, Button, IconButton, Slide, Typography } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { IronflyerMark } from './brand/IronflyerLogo';

type BeforeInstallPromptEvent = Event & {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
};

const DISMISS_KEY = 'ironflyer.pwa.installer.dismissed.v1';
const ACCEPTED_KEY = 'ironflyer.pwa.installer.accepted.v1';
const PROMPT_DELAY_MS = 15_000;

export function PWAInstaller() {
  const promptEventRef = useRef<BeforeInstallPromptEvent | null>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    // 1. Register the service worker. Quietly no-op on failure.
    if ('serviceWorker' in navigator) {
      const register = () => {
        navigator.serviceWorker.register('/sw.js').catch(() => {});
      };
      if (document.readyState === 'complete') register();
      else window.addEventListener('load', register, { once: true });
    }

    // 2. Capture beforeinstallprompt only if the user hasn't already
    //    dismissed or installed.
    let dismissed = false;
    let accepted = false;
    try {
      dismissed = localStorage.getItem(DISMISS_KEY) === '1';
      accepted = localStorage.getItem(ACCEPTED_KEY) === '1';
    } catch {
      /* private mode / storage blocked — fall through and prompt anyway */
    }
    if (dismissed || accepted) return;

    let timerId: ReturnType<typeof setTimeout> | null = null;

    const handleBeforeInstall = (e: Event) => {
      e.preventDefault();
      promptEventRef.current = e as BeforeInstallPromptEvent;
      timerId = setTimeout(() => setVisible(true), PROMPT_DELAY_MS);
    };

    const handleInstalled = () => {
      try {
        localStorage.setItem(ACCEPTED_KEY, '1');
      } catch {}
      setVisible(false);
    };

    window.addEventListener('beforeinstallprompt', handleBeforeInstall);
    window.addEventListener('appinstalled', handleInstalled);

    return () => {
      window.removeEventListener('beforeinstallprompt', handleBeforeInstall);
      window.removeEventListener('appinstalled', handleInstalled);
      if (timerId) clearTimeout(timerId);
    };
  }, []);

  const dismiss = () => {
    try {
      localStorage.setItem(DISMISS_KEY, '1');
    } catch {}
    setVisible(false);
  };

  const install = async () => {
    const evt = promptEventRef.current;
    if (!evt) {
      dismiss();
      return;
    }
    try {
      await evt.prompt();
      const choice = await evt.userChoice;
      if (choice.outcome === 'accepted') {
        try {
          localStorage.setItem(ACCEPTED_KEY, '1');
        } catch {}
      } else {
        try {
          localStorage.setItem(DISMISS_KEY, '1');
        } catch {}
      }
    } catch {
      /* user closed the native sheet — treat as dismiss */
      try {
        localStorage.setItem(DISMISS_KEY, '1');
      } catch {}
    } finally {
      promptEventRef.current = null;
      setVisible(false);
    }
  };

  return (
    <Slide direction="up" in={visible} mountOnEnter unmountOnExit>
      <Box
        role="dialog"
        aria-label="Install app"
        className="safe-area-bottom"
        sx={{
          position: 'fixed',
          left: 16,
          right: 16,
          bottom: 16,
          zIndex: 1500,
          background: '#f4f0e8',
          color: '#0d0e0f',
          borderRadius: 1,
          boxShadow: '0 14px 32px rgba(13,14,15,0.16)',
          border: '1px solid rgba(13,14,15,0.08)',
          p: 2,
          display: 'flex',
          alignItems: 'center',
          gap: 1.5,
          flexDirection: { xs: 'column', sm: 'row' },
        }}
      >
        <IronflyerMark size={34} tone="light" />
        <Box sx={{ flex: 1, minWidth: 0, textAlign: 'left' }}>
          <Typography
            variant="subtitle1"
            sx={{ fontFamily: 'var(--font-display)', fontWeight: 700, lineHeight: 1.2 }}
          >
            Install Ironflyer
          </Typography>
          <Typography variant="body2" sx={{ opacity: 0.75, mt: 0.5 }}>
            Faster access from your home screen, with offline support when the network drops.
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Button
            onClick={install}
            variant="contained"
            disableElevation
            sx={{
              background: '#e5ff00',
              color: '#0d0e0f',
              fontWeight: 700,
              borderRadius: 999,
              px: 2.5,
              minHeight: 'var(--tap-target)',
              '&:hover': { background: '#d4ee00' },
            }}
          >
            Install Ironflyer
          </Button>
          <IconButton
            aria-label="Close"
            onClick={dismiss}
            className="tap-target"
            sx={{ color: '#0d0e0f' }}
          >
            <CloseIcon />
          </IconButton>
        </Box>
      </Box>
    </Slide>
  );
}
