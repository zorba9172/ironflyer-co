import { Box, CircularProgress } from '@mui/material';
import { useAuth } from '@ironflyer/data';
import { Login } from '../pages/Login';
import type { ReactNode } from 'react';

// Offline (no endpoint) → render the app in sample mode. Online but no session
// → require sign in. Online + checking → brief splash.
export function LoginGate({ children }: { children: ReactNode }) {
  const { online, ready, user } = useAuth();
  if (online && !ready) {
    return (
      <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', bgcolor: 'background.default' }}>
        <CircularProgress size={28} />
      </Box>
    );
  }
  if (online && !user) return <Login />;
  return <>{children}</>;
}
