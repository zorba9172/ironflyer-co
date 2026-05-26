import * as React from "react";
import { AppRouterCacheProvider } from "@mui/material-nextjs/v15-appRouter";
import CssBaseline from "@mui/material/CssBaseline";
import { SessionProvider } from "next-auth/react";

export const metadata = {
  title: "Ironflyer SaaS Starter",
  description:
    "Next.js 15 + MUI 6 + Prisma + NextAuth v5 + Stripe — multi-tenant SaaS blueprint.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body style={{ margin: 0, fontFamily: "system-ui, sans-serif" }}>
        <AppRouterCacheProvider>
          <CssBaseline />
          <SessionProvider>{children}</SessionProvider>
        </AppRouterCacheProvider>
      </body>
    </html>
  );
}
