"use client";

// V22 cockpit top bar.
//
// Layout: [brand mark] [primary nav]  ··· [wallet chip] [user menu]
//
// The wallet chip is the V22 signature — it surfaces availableUSD
// next to the user menu so every cockpit page reminds the operator
// that paid execution is a prepaid wallet contract (law 1).
// Operator-only nav links are hidden unless the caller has the
// "operator" plan tag (the orchestrator's role.IsOperator helper).

import {
  AccountBalanceWalletOutlined,
  ArrowForwardRounded,
  ExpandMoreRounded,
  LogoutRounded,
  MenuRounded,
  NotificationsNoneRounded,
  PersonOutlineRounded,
  SettingsOutlined,
} from "@mui/icons-material";
import {
  AppBar,
  Avatar,
  Box,
  Button,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Toolbar,
  Tooltip,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useMemo, useState } from "react";
import { useExecutionsQuery } from "../../lib/gql/__generated__";
import { useAuth } from "../../lib/auth";
import { useWalletBalance } from "../../lib/hooks";
import { formatMoney } from "../../lib/format";
import { tokens } from "../../theme";
import { BrandLogo } from "../BrandLogo";
import { NotificationsBell } from "./NotificationsBell";

interface NavLink {
  label: string;
  href: string;
  match: (path: string) => boolean;
  operatorOnly?: boolean;
}

// Marketing nav — shown on the public site (home, /product, /pricing, …)
// or to unauthenticated visitors anywhere.
const MARKETING_LINKS: NavLink[] = [
  {
    label: "Product",
    href: "/product",
    match: (p) => p === "/" || p.startsWith("/product"),
  },
  {
    label: "Templates",
    href: "/templates",
    match: (p) => p.startsWith("/templates"),
  },
  { label: "Mobile", href: "/mobile", match: (p) => p.startsWith("/mobile") },
  { label: "Solutions", href: "/solutions", match: (p) => p.startsWith("/solutions") },
  { label: "Pricing", href: "/pricing", match: (p) => p.startsWith("/pricing") },
  {
    label: "Resources",
    href: "/resources",
    match: (p) =>
      p.startsWith("/resources") ||
      p.startsWith("/showcase") ||
      p.startsWith("/changelog") ||
      p.startsWith("/blog") ||
      p.startsWith("/security") ||
      p.startsWith("/developers"),
  },
  { label: "Enterprise", href: "/enterprise", match: (p) => p.startsWith("/enterprise") },
];

// Sub-links surfaced under the "Resources" dropdown on marketing nav.
// Keeps the top bar from getting crowded while exposing all six SEO
// landing pages added in v22.4.7.
interface ResourceLink {
  label: string;
  href: string;
  description: string;
}

const RESOURCE_LINKS: ResourceLink[] = [
  { label: "Showcase", href: "/showcase", description: "What people ship with Ironflyer." },
  { label: "Developers", href: "/developers", description: "GraphQL, SSE, runtime API, @ironflyer/sdk." },
  { label: "Security", href: "/security", description: "OwnerID isolation, wallet hard-block, ledger." },
  { label: "Changelog", href: "/changelog", description: "What we shipped to the gate." },
  { label: "Blog", href: "/blog", description: "Field notes on shipping AI-built software." },
  { label: "Resources", href: "/resources", description: "Guides, templates, and references." },
];

// Cockpit nav — shown to authenticated callers on cockpit routes. The
// shape is identical across /dashboard, /projects, /executions, /wallet,
// /deploy, /settings, /operator so chrome stays stable as the user
// moves between surfaces.
const COCKPIT_LINKS: NavLink[] = [
  { label: "Overview", href: "/dashboard", match: (p) => p === "/dashboard" || p.startsWith("/dashboard/") },
  { label: "Projects", href: "/projects", match: (p) => p === "/projects" || p.startsWith("/projects/") || p.startsWith("/p/") },
  { label: "Executions", href: "/executions", match: (p) => p.startsWith("/executions") || p.startsWith("/execution/") },
  { label: "Wallet", href: "/wallet", match: (p) => p.startsWith("/wallet") },
  { label: "Deploy", href: "/deploy", match: (p) => p.startsWith("/deploy") },
  {
    label: "Health",
    href: "/cockpit/health",
    match: (p) => p.startsWith("/cockpit/health"),
  },
  {
    label: "Learning",
    href: "/cockpit/learning",
    match: (p) => p.startsWith("/cockpit/learning"),
  },
  {
    label: "Operator",
    href: "/operator",
    match: (p) => p.startsWith("/operator"),
    operatorOnly: true,
  },
];

const COCKPIT_ROUTE_PREFIXES = [
  "/dashboard",
  "/projects",
  "/executions",
  "/execution/",
  "/wallet",
  "/deploy",
  "/settings",
  "/operator",
  "/cockpit/",
  "/p/",
];

function isCockpitRoute(path: string): boolean {
  return COCKPIT_ROUTE_PREFIXES.some((prefix) =>
    prefix.endsWith("/") ? path.startsWith(prefix) : path === prefix || path.startsWith(`${prefix}/`),
  );
}

function isOperator(plan?: string | null): boolean {
  if (!plan) return false;
  const p = plan.toLowerCase();
  return p === "operator" || p === "admin" || p === "owner";
}

export function Nav() {
  const pathname = usePathname() || "/";
  const { user, authenticated, signOut } = useAuth();
  const operator = isOperator(user?.plan);

  const cockpitMode = authenticated && isCockpitRoute(pathname);
  const links = cockpitMode ? COCKPIT_LINKS : MARKETING_LINKS;

  const { availableUSD: available, lowBalance } = useWalletBalance();

  // Notifications surface — most recent paid executions surfaced as
  // "Execution succeeded / failed" lines. We use the executions query
  // already on the cache (kept fresh by the cockpit pages) so this
  // adds zero extra network when the user is browsing the cockpit.
  const execQuery = useExecutionsQuery({
    skip: !authenticated,
    variables: { limit: 5, offset: 0 },
    fetchPolicy: "cache-first",
    pollInterval: authenticated && cockpitMode ? 30000 : 0,
  });
  const recentExecutions = useMemo(
    () => execQuery.data?.executions ?? [],
    [execQuery.data],
  );

  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [notifAnchor, setNotifAnchor] = useState<HTMLElement | null>(null);
  const [mobileNavAnchor, setMobileNavAnchor] = useState<HTMLElement | null>(null);
  const [resourcesAnchor, setResourcesAnchor] = useState<HTMLElement | null>(null);
  const visibleLinks = links.filter((l) => !l.operatorOnly || operator);
  const authReturn = pathname === "/" ? "/studio" : pathname;
  const loginHref = `/login?returnTo=${encodeURIComponent(authReturn)}`;
  const signupHref = `/signup?redirect=${encodeURIComponent(authReturn)}`;

  return (
    <AppBar
      position="sticky"
      sx={{
        bgcolor: `${tokens.color.bg.surface}c7`,
        backdropFilter: "blur(18px) saturate(140%)",
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
      }}
    >
      <Toolbar
        sx={{
          px:
            pathname === "/studio"
              ? { xs: 1, sm: 1.4, md: 1.6 }
              : { xs: 1, sm: 2, md: 4 },
          gap: { xs: 0.8, sm: 1.5, md: 3 },
          minHeight: pathname === "/studio" ? 58 : 70,
          position: "relative",
        }}
      >
        <IconButton
          size="small"
          onClick={(e) => setMobileNavAnchor(e.currentTarget)}
          aria-label="Open navigation menu"
          sx={{
            display: { xs: "inline-flex", md: "none" },
            color: tokens.color.text.secondary,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            width: 36,
            height: 36,
            mr: 0.5,
            "&:hover": {
              bgcolor: tokens.color.bg.surfaceHover,
              color: tokens.color.text.primary,
            },
          }}
        >
          <MenuRounded sx={{ fontSize: 20 }} />
        </IconButton>
        <Menu
          anchorEl={mobileNavAnchor}
          open={!!mobileNavAnchor}
          onClose={() => setMobileNavAnchor(null)}
          slotProps={{
            paper: {
              sx: {
                mt: 1,
                minWidth: 220,
                border: `1px solid ${tokens.color.border.subtle}`,
              },
            },
          }}
        >
          {visibleLinks.map((l) => {
            const active = l.match(pathname);
            return (
              <MenuItem
                key={l.href}
                component={Link}
                href={l.href}
                onClick={() => setMobileNavAnchor(null)}
                sx={{
                  fontWeight: 700,
                  fontSize: 14,
                  color: active ? tokens.color.text.primary : tokens.color.text.secondary,
                  bgcolor: active ? `${tokens.color.accent.purple}24` : "transparent",
                }}
              >
                {l.label}
              </MenuItem>
            );
          })}
        </Menu>

        <BrandLogo compact={false} inverse size={30} href="/" />

        <Stack
          direction="row"
          spacing={0.5}
          sx={{
            display: { xs: "none", md: "flex" },
            left: pathname === "/studio" ? "50%" : "auto",
            ml: pathname === "/studio" ? 0 : 2,
            position: pathname === "/studio" ? "absolute" : "static",
            transform: pathname === "/studio" ? "translateX(-50%)" : "none",
          }}
        >
          {visibleLinks.map((l) => {
            const active = l.match(pathname);
            const hasMenuCaret =
              !cockpitMode && (l.label === "Solutions" || l.label === "Resources");
            const isResources = !cockpitMode && l.label === "Resources";
            const sxButton = {
              color: active ? tokens.color.text.primary : tokens.color.text.secondary,
              bgcolor: active ? `${tokens.color.accent.purple}24` : "transparent",
              borderRadius: 1,
              px: 1.5,
              py: 0.75,
              fontWeight: 700,
              letterSpacing: 0.1,
              "&:hover": {
                bgcolor: tokens.color.bg.surfaceHover,
                color: tokens.color.text.primary,
              },
            } as const;
            const inner = (
              <Stack direction="row" sx={{ alignItems: "center", gap: 0.35 }}>
                {l.label}
                {hasMenuCaret && <ExpandMoreRounded sx={{ fontSize: 15 }} />}
              </Stack>
            );
            if (isResources) {
              return (
                <Button
                  key={l.href}
                  onClick={(e) => setResourcesAnchor(e.currentTarget)}
                  size="small"
                  sx={sxButton}
                  aria-haspopup="menu"
                  aria-expanded={!!resourcesAnchor}
                >
                  {inner}
                </Button>
              );
            }
            return (
              <Button
                key={l.href}
                component={Link}
                href={l.href}
                size="small"
                sx={sxButton}
              >
                {inner}
              </Button>
            );
          })}
        </Stack>

        <Menu
          anchorEl={resourcesAnchor}
          open={!!resourcesAnchor}
          onClose={() => setResourcesAnchor(null)}
          slotProps={{
            paper: {
              sx: {
                mt: 1,
                minWidth: 320,
                bgcolor: tokens.color.bg.surfaceRaised,
                border: `1px solid ${tokens.color.border.subtle}`,
              },
            },
          }}
        >
          {RESOURCE_LINKS.map((r) => (
            <MenuItem
              key={r.href}
              component={Link}
              href={r.href}
              onClick={() => setResourcesAnchor(null)}
              sx={{
                alignItems: "flex-start",
                py: 1.2,
                gap: 0.4,
                flexDirection: "column",
              }}
            >
              <Typography
                sx={{ fontWeight: 700, fontSize: 14, color: tokens.color.text.primary }}
              >
                {r.label}
              </Typography>
              <Typography sx={{ fontSize: 12.5, color: tokens.color.text.secondary }}>
                {r.description}
              </Typography>
            </MenuItem>
          ))}
        </Menu>

        <Box sx={{ flexGrow: 1 }} />

        {authenticated && (
          <Tooltip
            title={lowBalance ? "Wallet running low — top up to keep executions admitted" : "Top up wallet"}
            arrow
          >
            <Button
              component={Link}
              href="/wallet"
              size="small"
              startIcon={<AccountBalanceWalletOutlined sx={{ fontSize: 18 }} />}
              sx={{
                color: tokens.color.text.primary,
                bgcolor: tokens.color.bg.surfaceRaised,
                border: `1px solid ${lowBalance ? tokens.color.accent.warning : tokens.color.border.subtle}`,
                px: { xs: 1, md: 1.5 },
                py: 0.75,
                minHeight: 36,
                fontFamily: tokens.font.mono,
                fontWeight: 600,
                fontSize: { xs: 12, md: 13 },
                whiteSpace: "nowrap",
                transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}, background-color ${tokens.motion.fast} ${tokens.motion.snap}`,
                animation: lowBalance ? "ironflyerWalletPulse 2.4s ease-in-out infinite" : "none",
                "@keyframes ironflyerWalletPulse": {
                  "0%, 100%": { boxShadow: `0 0 0 0 ${tokens.color.accent.warning}00` },
                  "50%": { boxShadow: `0 0 0 4px ${tokens.color.accent.warning}33` },
                },
                "&:hover": {
                  bgcolor: tokens.color.bg.surfaceHover,
                  borderColor: tokens.color.border.strong,
                },
                "& .MuiButton-startIcon": {
                  mr: { xs: 0.5, md: 1 },
                },
              }}
            >
              <Box
                component="span"
                sx={{
                  color: lowBalance
                    ? tokens.color.accent.warning
                    : available > 0
                      ? tokens.color.brand.mint
                      : tokens.color.text.muted,
                  mr: 0.5,
                }}
              >
                {formatMoney(available)}
              </Box>
              <Box
                component="span"
                sx={{ color: tokens.color.text.muted, display: { xs: "none", sm: "inline" } }}
              >
                available
              </Box>
            </Button>
          </Tooltip>
        )}

        {authenticated && <NotificationsBell />}

        {authenticated && (
          <>
            <Tooltip title="Recent executions" arrow>
              <IconButton
                size="small"
                onClick={(e) => setNotifAnchor(e.currentTarget)}
                aria-label="Recent executions"
                sx={{
                  color: tokens.color.text.secondary,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  borderRadius: 1,
                  width: 34,
                  height: 34,
                  "&:hover": {
                    bgcolor: tokens.color.bg.surfaceHover,
                    color: tokens.color.text.primary,
                  },
                }}
              >
                <NotificationsNoneRounded sx={{ fontSize: 18 }} />
              </IconButton>
            </Tooltip>
            <Menu
              anchorEl={notifAnchor}
              open={!!notifAnchor}
              onClose={() => setNotifAnchor(null)}
              slotProps={{
                paper: {
                  sx: {
                    mt: 1,
                    minWidth: 320,
                    maxWidth: 380,
                    border: `1px solid ${tokens.color.border.subtle}`,
                  },
                },
              }}
            >
              <Box sx={{ px: 2, py: 1.25 }}>
                <Typography variant="overline" sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}>
                  Recent executions
                </Typography>
              </Box>
              <Divider />
              {recentExecutions.length === 0 ? (
                <Box sx={{ px: 2, py: 2.5 }}>
                  <Typography sx={{ fontSize: 13, color: tokens.color.text.muted }}>
                    No executions yet. Start a build to see live events here.
                  </Typography>
                </Box>
              ) : (
                recentExecutions.slice(0, 6).map((e) => (
                  <MenuItem
                    key={e.id}
                    component={Link}
                    href={`/execution/${e.id}`}
                    onClick={() => setNotifAnchor(null)}
                    sx={{ alignItems: "flex-start", py: 1 }}
                  >
                    <Box sx={{ minWidth: 0, flex: 1 }}>
                      <Typography
                        sx={{
                          fontSize: 13,
                          fontWeight: 700,
                          color: tokens.color.text.primary,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {e.promptSummary || e.blueprintID || e.id}
                      </Typography>
                      <Typography
                        sx={{
                          mt: 0.25,
                          fontSize: 11.5,
                          color: tokens.color.text.muted,
                          fontFamily: tokens.font.mono,
                          textTransform: "uppercase",
                          letterSpacing: 0.6,
                        }}
                      >
                        {e.status}
                      </Typography>
                    </Box>
                  </MenuItem>
                ))
              )}
              <Divider />
              <MenuItem
                component={Link}
                href="/executions"
                onClick={() => setNotifAnchor(null)}
                sx={{ justifyContent: "center", py: 1, fontSize: 13, fontWeight: 700 }}
              >
                View all executions
              </MenuItem>
            </Menu>
          </>
        )}

        {authenticated ? (
          <>
            <IconButton
              onClick={(e) => setAnchor(e.currentTarget)}
              size="small"
              sx={{
                ml: 0.5,
                gap: 0.5,
                borderRadius: tokens.radius.pill,
                px: 0.75,
                transition: `box-shadow ${tokens.motion.fast} ${tokens.motion.snap}`,
                "&:hover": {
                  bgcolor: tokens.color.bg.surfaceHover,
                  boxShadow: `0 0 0 2px ${tokens.color.accent.purple}52`,
                },
              }}
            >
              <Avatar
                sx={{
                  width: 32,
                  height: 32,
                  background: `linear-gradient(135deg, ${tokens.color.accent.violet} 0%, ${tokens.color.accent.purple} 100%)`,
                  color: tokens.color.text.primary,
                  fontSize: 13,
                  fontWeight: 800,
                }}
              >
                {(user?.name || user?.email || "?").slice(0, 1).toUpperCase()}
              </Avatar>
              <ExpandMoreRounded sx={{ fontSize: 16, color: tokens.color.text.secondary }} />
            </IconButton>
            <Menu
              anchorEl={anchor}
              open={!!anchor}
              onClose={() => setAnchor(null)}
              slotProps={{
                paper: {
                  sx: {
                    mt: 1,
                    minWidth: 220,
                    border: `1px solid ${tokens.color.border.subtle}`,
                  },
                },
              }}
            >
              <Box sx={{ px: 2, py: 1.25 }}>
                <Typography sx={{ fontWeight: 700, fontSize: 13 }}>
                  {user?.name || user?.email}
                </Typography>
                <Typography sx={{ fontSize: 12, color: tokens.color.text.secondary }}>
                  {user?.email}
                </Typography>
              </Box>
              <Divider />
              <MenuItem component={Link} href="/settings" onClick={() => setAnchor(null)}>
                <ListItemIcon><PersonOutlineRounded fontSize="small" /></ListItemIcon>
                <ListItemText primary="Profile" />
              </MenuItem>
              <MenuItem component={Link} href="/wallet" onClick={() => setAnchor(null)}>
                <ListItemIcon><AccountBalanceWalletOutlined fontSize="small" /></ListItemIcon>
                <ListItemText primary="Wallet" />
              </MenuItem>
              {operator && (
                <MenuItem component={Link} href="/operator" onClick={() => setAnchor(null)}>
                  <ListItemIcon><SettingsOutlined fontSize="small" /></ListItemIcon>
                  <ListItemText primary="Operator console" />
                </MenuItem>
              )}
              <Divider />
              <MenuItem
                onClick={() => {
                  setAnchor(null);
                  void signOut();
                }}
              >
                <ListItemIcon><LogoutRounded fontSize="small" /></ListItemIcon>
                <ListItemText primary="Sign out" />
              </MenuItem>
            </Menu>
          </>
        ) : (
          <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
            <Button
              component={Link}
              href={loginHref}
              size="small"
              variant="text"
              sx={{
                color: tokens.color.text.primary,
                display: { xs: "none", sm: "inline-flex" },
                minWidth: 58,
                whiteSpace: "nowrap",
              }}
            >
              Log in
            </Button>
            <Button
              component={Link}
              href={signupHref}
              size="small"
              variant="contained"
              color="primary"
              endIcon={<ArrowForwardRounded sx={{ fontSize: 15 }} />}
              sx={{
                minWidth: { xs: 74, sm: 132 },
                px: { xs: 1.15, sm: 1.8 },
                whiteSpace: "nowrap",
                "& .MuiButton-endIcon": {
                  display: { xs: "none", sm: "inherit" },
                  ml: { sm: 0.75 },
                },
              }}
            >
              <Box component="span" sx={{ display: { xs: "none", sm: "inline" } }}>
                Start a project
              </Box>
              <Box component="span" sx={{ display: { xs: "inline", sm: "none" } }}>
                Start
              </Box>
            </Button>
          </Stack>
        )}
      </Toolbar>
    </AppBar>
  );
}
