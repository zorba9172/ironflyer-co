"use client";

// Root provider stack — wraps every cockpit route.
//
//   AppRouterCacheProvider   MUI SSR cache (Next.js v15 App Router)
//     ApolloProvider         shared Apollo client (HTTP + WS + auth)
//       ThemeMode dark       cockpit chrome defaults to dark
//         GlobalStyles       app-wide CSS resets + scrollbar
//           AuthProvider     `useAuth()` everywhere downstream
//             children

import { GlobalStyles } from "@mui/material";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v15-appRouter";
import type { ReactNode } from "react";
import { NotificationCenter } from "../src/components/cockpit/NotificationCenter";
import { ApolloProvider } from "../src/lib/apollo";
import { AuthProvider } from "../src/lib/auth";
import { ThemeMode } from "../src/theme/ThemeMode";
import { globalSx } from "./globalStyles";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <AppRouterCacheProvider>
      <ApolloProvider>
        <ThemeMode mode="dark">
          <GlobalStyles styles={globalSx} />
          <AuthProvider>
            {children}
            <NotificationCenter />
          </AuthProvider>
        </ThemeMode>
      </ApolloProvider>
    </AppRouterCacheProvider>
  );
}
