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

import { Box, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { Suspense, type ReactNode } from "react";
import { tokens } from "../../theme";
import { BrandLogo } from "../BrandLogo";
import { Nav } from "./Nav";

export function CockpitFrame({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const isMarketingHome = pathname === "/";
  const isPublicMarketing =
    isMarketingHome ||
    pathname === "/product" ||
    pathname === "/templates" ||
    pathname === "/solutions" ||
    pathname === "/pricing" ||
    pathname === "/resources" ||
    pathname === "/enterprise" ||
    pathname === "/vscode" ||
    pathname === "/appsec" ||
    pathname === "/compare" ||
    pathname === "/developers" ||
    pathname === "/mobile" ||
    pathname === "/security" ||
    pathname === "/showcase" ||
    pathname === "/blog" ||
    pathname === "/changelog" ||
    pathname?.startsWith("/vs/") === true;
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
    isPublicMarketing || isStudioEntry || isStudioWorkspace || isAuthRoute;
  // The marketing home renders its own bespoke <Footer> (orbital +
  // proof bands + CTA). Suppress the shared ShellFooter on `/` so the
  // page does not stack two footers.
  const showFooter =
    !isAuthRoute && !isStudioEntry && !isStudioWorkspace && !isMarketingHome;

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
      {!isAuthRoute && !isStudioEntry && !isStudioWorkspace && (
        <Suspense fallback={null}>
          <Nav />
        </Suspense>
      )}
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
      {showFooter && (
        <Suspense fallback={<ShellFooter timing="dark" />}>
          <ShellFooterWithTiming publicMarketing={isPublicMarketing} />
        </Suspense>
      )}
    </Box>
  );
}

function ShellFooterWithTiming({
  publicMarketing,
}: {
  publicMarketing: boolean;
}) {
  const search = useSearchParams();
  const timing =
    publicMarketing && search?.get("theme") !== "dark" ? "light" : "dark";
  return <ShellFooter timing={timing} />;
}

function ShellFooter({ timing }: { timing: "light" | "dark" }) {
  const light = timing === "light";
  const text = light ? "#0b1040" : tokens.color.text.primary;
  const secondary = light ? "#66708d" : tokens.color.text.secondary;
  const border = light ? "rgba(18,22,55,0.10)" : tokens.color.border.subtle;
  const bg = light ? "#fbfaff" : tokens.color.bg.base;
  const columns = [
    ["Product", "Product", "Templates", "Pricing", "VS Code"],
    ["Solutions", "Solutions", "Enterprise", "Resources", "Security"],
    ["Account", "Studio", "Projects", "Wallet", "Settings"],
  ];

  return (
    <Box
      component="footer"
      sx={{
        borderTop: `1px solid ${border}`,
        bgcolor: bg,
        color: text,
        px: { xs: 2.5, md: 6 },
        py: { xs: 4, md: 5 },
      }}
    >
      <Box
        sx={{
          maxWidth: 1240,
          mx: "auto",
          display: "grid",
          gap: { xs: 3, md: 6 },
          gridTemplateColumns: { xs: "1fr", md: "1.15fr repeat(3, 0.7fr)" },
        }}
      >
        <Stack spacing={1.4}>
          <BrandLogo size={25} inverse={!light} href={`/?theme=${timing}`} />
          <Typography
            sx={{
              color: secondary,
              fontSize: 13.5,
              lineHeight: 1.55,
              maxWidth: 320,
            }}
          >
            Build production apps from a prompt, then keep the work inside
            Studio, code, preview and deploy.
          </Typography>
        </Stack>
        {columns.map(([title, ...links]) => (
          <Stack key={title} spacing={1}>
            <Typography
              sx={{
                color: text,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                fontWeight: 800,
                textTransform: "uppercase",
              }}
            >
              {title}
            </Typography>
            {links.map((label) => {
              const href =
                label === "VS Code"
                  ? "/vscode"
                  : label === "Studio"
                    ? "/studio"
                    : `/${label.toLowerCase()}`;
              return (
                <Typography
                  key={label}
                  component={Link}
                  href={`${href}${href.includes("?") ? "&" : "?"}theme=${timing}`}
                  sx={{
                    color: secondary,
                    fontSize: 13,
                    textDecoration: "none",
                    "&:hover": { color: text },
                  }}
                >
                  {label}
                </Typography>
              );
            })}
          </Stack>
        ))}
      </Box>
    </Box>
  );
}
