"use client";

// CockpitFrame — top-level shell rendered by app/layout.tsx.
//
//   <ThemeMode dark>
//     <Nav />
//     <main>{children}</main>
//   </ThemeMode>
//
// The frame is responsible only for chrome — it never opens data
// queries. Per-page <RequireAuth /> guards decide whether the page
// renders or redirects to /login.

import { Box } from "@mui/material";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { tokens } from "../../theme";
import { Nav } from "./Nav";

export function CockpitFrame({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const isMarketingHome = pathname === "/";
  const isStudioEntry = pathname === "/studio";
  const isStudioWorkspace = pathname?.startsWith("/p/");
  // /login and /signup own their own full-bleed split layout (AuthShell)
  // and intentionally suppress the cockpit nav so the auth surface
  // matches the Base44 reference.
  const isAuthRoute =
    pathname === "/login" ||
    pathname === "/signup" ||
    pathname?.startsWith("/login/") === true ||
    pathname?.startsWith("/signup/") === true;
  const isFullBleed =
    isMarketingHome || isStudioEntry || isStudioWorkspace || isAuthRoute;

  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        bgcolor: tokens.color.bg.base,
        overflowX: "clip",
      }}
    >
      {!isAuthRoute && <Nav />}
      <Box
        component="main"
        sx={{
          flex: 1,
          width: "100%",
          maxWidth: isFullBleed ? "none" : 1440,
          mx: "auto",
          minWidth: 0,
          overflowX: "clip",
          px: isFullBleed ? 0 : { xs: 2, md: 4 },
          py: isFullBleed ? 0 : { xs: 3, md: 4 },
        }}
      >
        {children}
      </Box>
    </Box>
  );
}
