import { Box, type BoxProps } from '@mui/material';
import { asset, assetUrl } from './assets';

export type AssetImageProps = {
  /** asset id ("pack/name") or a raw `/...` web path */
  id: string;
  /** square size in px (sets width & height); use width/height for non-square */
  size?: number;
  width?: number | string;
  height?: number | string;
  alt?: string;
  /** loop animated mp4/gif assets (default true) */
  loop?: boolean;
  sx?: BoxProps['sx'];
};

// Renders an illustrated/3D/animated asset from the manifest. SVG/PNG render as
// <img>; animated .mp4 render as a muted autoplay <video> (the 3D loops). All
// sizing/spacing flows through the theme via the MUI Box — no inline literals.
export function AssetImage({ id, size, width, height, alt, loop = true, sx }: AssetImageProps) {
  const entry = asset(id);
  const src = assetUrl(id);
  if (!src) return null;
  const w = width ?? size ?? 48;
  const h = height ?? size ?? 48;
  const isVideo = entry?.kind === 'animated' && src.toLowerCase().endsWith('.mp4');
  const common = { width: w, height: h, display: 'block', objectFit: 'contain' as const };

  if (isVideo) {
    return (
      <Box
        component="video"
        src={src}
        autoPlay={loop}
        loop={loop}
        muted
        playsInline
        aria-label={alt ?? entry?.name}
        sx={{ ...common, ...sx }}
      />
    );
  }
  return <Box component="img" src={src} alt={alt ?? entry?.name ?? ''} loading="lazy" sx={{ ...common, ...sx }} />;
}
