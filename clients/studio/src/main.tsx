import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { StudioThemeProvider } from './theme';
import '@ironflyer/ui-web/fonts.css';
import { IronflyerDataProvider, AuthProvider } from '@ironflyer/data';
import { App } from './App';
import { LoginGate } from './components/LoginGate';
import { getStoredAuthToken } from './lib/authToken';

// Endpoint comes from env; when unset the data hooks fall back to local
// sample data so the studio is fully usable offline.
const endpoint = import.meta.env.VITE_GRAPHQL_ENDPOINT as string | undefined;

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <IronflyerDataProvider endpoint={endpoint} getToken={getStoredAuthToken}>
      <StudioThemeProvider>
        <AuthProvider>
          <BrowserRouter>
            <LoginGate>
              <App />
            </LoginGate>
          </BrowserRouter>
        </AuthProvider>
      </StudioThemeProvider>
    </IronflyerDataProvider>
  </StrictMode>,
);
