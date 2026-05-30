import { useState } from 'react';
import { Box, CircularProgress } from '@mui/material';
import { useLocation } from 'react-router-dom';
import { useAuth } from '@ironflyer/data';
import { Login } from '../pages/Login';
import { Landing } from '../pages/Landing';
import type { ReactNode } from 'react';

// Offline (no endpoint) → render the app in sample mode. Online but no session
// → the Neon Intelligence landing (the logged-out marketing entry); its CTAs
// hand off to the sign-in form. Online + checking → brief splash.
export function LoginGate({ children }: { children: ReactNode }) {
  const { online, ready, user } = useAuth();
  const { pathname, search } = useLocation();
  const [showAuth, setShowAuth] = useState(false);
  const publicRoute = ['/', '/welcome', '/build', '/studio', '/projects', '/templates', '/integrations', '/agents', '/plans'].includes(pathname);
  const authIntent = new URLSearchParams(search).get('auth') === '1';
  if (online && !ready) {
    return (
      <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', bgcolor: 'background.default' }}>
        <CircularProgress size={28} />
      </Box>
    );
  }
  if (publicRoute) return <>{children}</>;
  if (online && !user) {
    if (authIntent) return <Login />;
    return showAuth ? <Login /> : <Landing onEnter={() => setShowAuth(true)} />;
  }
  return <>{children}</>;
}
