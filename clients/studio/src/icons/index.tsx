// ─────────────────────────────────────────────────────────────────────────
// IRONFLYER STUDIO — ICON SOURCE OF TRUTH (UI glyphs)
//
// One semantic glyph vocabulary for the whole studio. Components reference an
// app-level NAME, never a vendor glyph, so the underlying set (Lucide) can be
// swapped in exactly one place. See DESIGN_CONSTITUTION.md › Iconography Law.
//
//   import { Icon } from '../icons';
//   <Icon name="rocket" size={18} />            // inherits currentColor
//   <Icon name="shieldCheck" color="success" /> // semantic palette tone
//
// Illustrated / 3D / animated assets live in './assets' + <AssetImage/>.
// Brand / tech marks live in '../lib/techIcons'.
// ─────────────────────────────────────────────────────────────────────────
import type { IconType } from 'react-icons';
import {
  LuActivity, LuArrowRight, LuArrowUpRight, LuBell, LuBot, LuBox, LuCalendarClock,
  LuChartBar, LuChartPie, LuCheck, LuCheckCheck, LuChevronDown, LuChevronRight, LuCircleHelp,
  LuClock, LuCode, LuCopy, LuDatabase, LuDownload, LuEllipsis, LuExternalLink, LuEye, LuFilter,
  LuFolder, LuFolderPlus, LuGauge, LuGitPullRequestArrow, LuGlobe, LuGrid2X2, LuHouse, LuInfo,
  LuLayers, LuLayoutDashboard, LuLayoutGrid, LuLink, LuMic, LuMoon, LuNetwork, LuPanelLeft,
  LuPause, LuPencil, LuPlay, LuPlug, LuPlus, LuRadioTower, LuRefreshCw, LuRocket, LuSearch,
  LuSettings, LuShield, LuShieldCheck, LuSlidersHorizontal, LuSmartphone, LuSparkles, LuStore, LuSun,
  LuTrash2, LuTriangleAlert, LuUsers, LuWallet, LuWebhook, LuWorkflow, LuWrench, LuX, LuZap,
} from 'react-icons/lu';
import { useTheme, type Theme } from '@mui/material/styles';

// Semantic name → glyph. Keys are app vocabulary; values are the chosen set.
export const ICONS = {
  // ── navigation ──────────────────────────────────────────────────────────
  home: LuHouse,
  projects: LuLayoutGrid,
  agents: LuBot,
  templates: LuGrid2X2,
  deployments: LuRocket,
  integrations: LuPlug,
  settings: LuSettings,
  data: LuDatabase,
  users: LuUsers,
  analytics: LuChartBar,
  domains: LuGlobe,
  automations: LuWorkflow,
  api: LuCode,
  marketing: LuStore,
  dashboard: LuLayoutDashboard,
  // ── actions ───────────────────────────────────────────────────────────────
  build: LuRocket,
  add: LuPlus,
  newProject: LuFolderPlus,
  search: LuSearch,
  filter: LuFilter,
  refresh: LuRefreshCw,
  download: LuDownload,
  external: LuExternalLink,
  link: LuLink,
  edit: LuPencil,
  copy: LuCopy,
  trash: LuTrash2,
  wrench: LuWrench,
  sliders: LuSlidersHorizontal,
  more: LuEllipsis,
  close: LuX,
  play: LuPlay,
  pause: LuPause,
  collapse: LuPanelLeft,
  // ── status / signal ───────────────────────────────────────────────────────
  check: LuCheck,
  checkAll: LuCheckCheck,
  alert: LuTriangleAlert,
  shield: LuShield,
  shieldCheck: LuShieldCheck,
  clock: LuClock,
  activity: LuActivity,
  zap: LuZap,
  sparkles: LuSparkles,
  eye: LuEye,
  info: LuInfo,
  help: LuCircleHelp,
  bell: LuBell,
  gauge: LuGauge,
  // ── domain ──────────────────────────────────────────────────────────────
  bot: LuBot,
  code: LuCode,
  box: LuBox,
  layers: LuLayers,
  network: LuNetwork,
  workflow: LuWorkflow,
  wallet: LuWallet,
  store: LuStore,
  webhook: LuWebhook,
  plug: LuPlug,
  mic: LuMic,
  smartphone: LuSmartphone,
  pullRequest: LuGitPullRequestArrow,
  radio: LuRadioTower,
  schedule: LuCalendarClock,
  folder: LuFolder,
  chartBar: LuChartBar,
  chartPie: LuChartPie,
  // ── chrome ──────────────────────────────────────────────────────────────
  sun: LuSun,
  moon: LuMoon,
  arrowRight: LuArrowRight,
  arrowUpRight: LuArrowUpRight,
  chevronRight: LuChevronRight,
  chevronDown: LuChevronDown,
} satisfies Record<string, IconType>;

export type IconName = keyof typeof ICONS;

// Semantic palette tones an icon may request (else inherits currentColor).
type IconTone = 'primary' | 'secondary' | 'success' | 'warning' | 'danger' | 'info' | 'muted';
const toneColor = (theme: Theme, tone?: IconTone): string | undefined => {
  switch (tone) {
    case 'primary': return theme.palette.primary.main;
    case 'secondary': return theme.palette.secondary.main;
    case 'success': return theme.palette.success.main;
    case 'warning': return theme.palette.warning.main;
    case 'danger': return theme.palette.error.main;
    case 'info': return theme.palette.info.main;
    case 'muted': return theme.palette.text.secondary;
    default: return undefined; // inherit currentColor
  }
};

export type IconProps = {
  name: IconName;
  size?: number;
  /** semantic tone from the theme palette; omit to inherit currentColor */
  color?: IconTone;
  strokeWidth?: number;
  className?: string;
  title?: string;
  'aria-hidden'?: boolean;
};

// The one icon component. Glyphs inherit `currentColor` by default so they tint
// with the surrounding themed text — pass `color` only for a semantic accent.
export function Icon({ name, size = 18, color, strokeWidth, className, title, ...rest }: IconProps) {
  const theme = useTheme();
  const Glyph = ICONS[name];
  return (
    <Glyph
      size={size}
      color={toneColor(theme, color)}
      strokeWidth={strokeWidth}
      className={className}
      title={title}
      aria-hidden={rest['aria-hidden'] ?? (title ? undefined : true)}
    />
  );
}

export { AssetImage } from './AssetImage';
export type { AssetImageProps } from './AssetImage';
export { asset, assetUrl, assetsByPack, findAssets, assetStats, ASSET_MANIFEST, ASSET_PACKS } from './assets';
export type { AssetEntry, AssetKind, AssetPack } from './assets';
