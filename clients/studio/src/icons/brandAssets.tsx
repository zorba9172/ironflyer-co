// ─────────────────────────────────────────────────────────────────────────
// IRONFLYER STUDIO — BRANDED ILLUSTRATED-ASSET VOCABULARY
//
// The semantic layer over the raw illustrated/3D manifest, mirroring the Icon
// barrel: components reference an app-level NAME (`agent.security`,
// `build.shipped`), never a vendor asset id. The underlying premium 3D / SVG
// art (curated to the on-brand AI · dev · security · data families) can be
// re-mapped in exactly one place. See DESIGN_CONSTITUTION.md › Iconography Law.
//
//   import { BrandAsset } from '../icons';
//   <BrandAsset name="agent.deployer" size={28} />
//
// Tasteful-accents rule: one signature illustrated moment per surface. Flat UI
// control glyphs stay on Lucide via <Icon/>; this vocabulary is reserved for
// agent avatars, capability tiles, build thumbnails, empty/hero states.
// ─────────────────────────────────────────────────────────────────────────
import { AssetImage, type AssetImageProps } from './AssetImage';

// App-level name → curated manifest id. Every value is an on-brand asset that
// exists in `asset-manifest.ts` (verified against public/icons). Keys are the
// product vocabulary; only this table knows the vendor ids.
export const BRAND_ASSET = {
  // ── Agent role avatars (premium 3D, reads as a polished app-icon) ──────────
  'agent.orchestrator': 'strategy-3d/4-gear',
  'agent.coder': 'strategy-3d/10-computer',
  'agent.identity': 'security/multifactor-authentication',
  'agent.payments': 'strategy-3d/3-wallet',
  'agent.data': 'security/secure-cloud-storage',
  'agent.security': 'security/zero-trust',
  'agent.deployer': 'strategy-3d/2-rocket',
  'agent.mobile': 'strategy-3d/8-monitor',

  // ── Recent-build category badges (status-driven) ──────────────────────────
  'build.shipped': 'strategy-3d/2-rocket',
  'build.blocked': 'security/firewall',
  'build.preview': 'strategy-3d/8-monitor',
  'build.progress': 'strategy-3d/4-gear',

  // ── Capability / domain marks (feature tiles, headers, empty states) ──────
  'cap.ship': 'strategy-3d/2-rocket',
  'cap.code': 'strategy-3d/10-computer',
  'cap.search': 'strategy-3d/5-search',
  'cap.wallet': 'strategy-3d/3-wallet',
  'cap.data': 'security/secure-cloud-storage',
  'cap.analytics': 'infographic/14-chart-infographic',
  'cap.funnel': 'infographic/20-funnel-infographic',
  'cap.timeline': 'infographic/15-timeline-infographic',
  'cap.growth': 'marketing-3d/user-engagement',

  // ── Security / AppSec illustrations (the differentiator surface) ──────────
  'security.hero': 'security/zero-trust',
  'security.firewall': 'security/firewall',
  'security.secrets': 'security/data-encrypt',
  'security.scan': 'security/penetration-test',
  'security.policy': 'security/access-control',
  'security.threat': 'security/cyber-threat-intelligence',
} as const satisfies Record<string, string>;

export type BrandAssetName = keyof typeof BRAND_ASSET;

/** Resolve a branded app-name to its underlying manifest id. */
export function brandAssetId(name: BrandAssetName): string {
  return BRAND_ASSET[name];
}

export type BrandAssetProps = { name: BrandAssetName } & Omit<AssetImageProps, 'id'>;

// The one branded-illustration component. Tasteful, sized through the theme via
// AssetImage's MUI Box — no inline literals. Reserve for accent moments only.
export function BrandAsset({ name, ...rest }: BrandAssetProps) {
  return <AssetImage id={BRAND_ASSET[name]} {...rest} />;
}
