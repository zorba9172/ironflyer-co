import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { ThemeModeProvider } from '@ironflyer/ui-web';
import '@ironflyer/ui-web/fonts.css';
import { QueryProvider } from '@ironflyer/data';
import { App } from './App';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryProvider>
      <ThemeModeProvider>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </ThemeModeProvider>
    </QueryProvider>
  </StrictMode>,
);
