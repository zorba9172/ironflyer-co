// ─────────────────────────────────────────────────────────────────────────
// Illustrated / 3D / animated ASSET registry — typed accessor over the
// generated manifest. The single source of truth for non-glyph visuals
// (feature tiles, empty states, onboarding, marketing) across Ironflyer apps.
//
//   import { asset, assetsByPack, AssetImage } from '../icons/assets';
//   <AssetImage id="insurance-3d/security" width={48} />
//   const heroLoop = asset('animated-3d/ai-brain');
//
// UI control glyphs are NOT here — use `../icons` (Lucide). See
// DESIGN_CONSTITUTION.md › Iconography Law. Regenerate after adding files:
//   node scripts/gen-icon-manifest.mjs
// ─────────────────────────────────────────────────────────────────────────
import { ASSET_MANIFEST, ASSET_PACKS, type AssetEntry, type AssetKind, type AssetPack } from './asset-manifest';

export { ASSET_MANIFEST, ASSET_PACKS };
export type { AssetEntry, AssetKind, AssetPack };

const BY_ID = new Map<string, AssetEntry>(ASSET_MANIFEST.map((e) => [e.id, e]));

/** Look up one asset entry by its `pack/name` id. */
export function asset(id: string): AssetEntry | undefined {
  return BY_ID.get(id);
}

/** Resolve an asset id (or a raw `/...` path) to a servable URL. */
export function assetUrl(idOrPath: string): string | undefined {
  if (idOrPath.startsWith('/')) return idOrPath;
  return BY_ID.get(idOrPath)?.path;
}

/** Every asset in a pack, optionally filtered by kind. */
export function assetsByPack(pack: AssetPack, kind?: AssetKind): AssetEntry[] {
  return ASSET_MANIFEST.filter((e) => e.pack === pack && (!kind || e.kind === kind));
}

/** Search by free text over id + name (case-insensitive). */
export function findAssets(query: string, kind?: AssetKind): AssetEntry[] {
  const q = query.toLowerCase();
  return ASSET_MANIFEST.filter(
    (e) => (!kind || e.kind === kind) && (e.id.includes(q) || e.name.toLowerCase().includes(q)),
  );
}

/** Count summary, handy for tooling / Storybook. */
export const assetStats = {
  total: ASSET_MANIFEST.length,
  packs: ASSET_PACKS.length,
  byKind: ASSET_MANIFEST.reduce<Record<AssetKind, number>>(
    (m, e) => ((m[e.kind] = (m[e.kind] ?? 0) + 1), m),
    {} as Record<AssetKind, number>,
  ),
};
