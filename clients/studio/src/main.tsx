import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { ThemeModeProvider } from '@ironflyer/ui-web';
import '@ironflyer/ui-web/fonts.css';
import { IronflyerDataProvider, AuthProvider } from '@ironflyer/data';
import { App } from './App';
import { LoginGate } from './components/LoginGate';

// Endpoint comes from env; when unset the data hooks fall back to local
// sample data so the studio is fully usable offline.
const endpoint = import.meta.env.VITE_GRAPHQL_ENDPOINT as string | undefined;
const getToken = () => localStorage.getItem('if-token');

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <IronflyerDataProvider endpoint={endpoint} getToken={getToken}>
      <ThemeModeProvider>
        <AuthProvider>
          <BrowserRouter>
            <LoginGate>
              <App />
            </LoginGate>
          </BrowserRouter>
        </AuthProvider>
      </ThemeModeProvider>
    </IronflyerDataProvider>
  </StrictMode>,
);
