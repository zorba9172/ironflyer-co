"use client";

// Lightbox — reusable thumbnail-plus-anchor that opens a high-res
// image in Fancybox. Use this when a surface has a one-off screenshot
// (e.g. a project artifact preview, a gate verdict screenshot, a deploy
// snapshot) and you want the cockpit-grade lightbox without rewiring
// data-attributes by hand.
//
// Each instance binds Fancybox on mount and unbinds on unmount; the
// bind() call is idempotent so several Lightboxes on the same page
// share a single Fancybox installation.

import { ZoomInRounded } from "@mui/icons-material";
import { Box } from "@mui/material";
import { useEffect, useMemo } from "react";
import {
  bind as bindLightbox,
  unbind as unbindLightbox,
} from "../../lib/lightbox";
import { tokens } from "../../theme";

export interface LightboxProps {
  // Full-resolution image URL Fancybox opens when the thumbnail is
  // clicked.
  src: string;
  // Optional srcSet — applied to the thumbnail <img> for retina.
  srcSet?: string;
  // Caption rendered below the slide in Fancybox.
  caption?: string;
  // Alt text for the thumbnail. Defaults to caption or "Preview".
  alt?: string;
  // Group identifier — same group means arrow-key navigation between
  // siblings. Defaults to a per-instance "lightbox-<n>" group so
  // standalone instances do not accidentally chain.
  group?: string;
  // Thumbnail width / height (CSS values).
  width?: number | string;
  height?: number | string;
}

let groupCounter = 0;

export function Lightbox({
  src,
  srcSet,
  caption,
  alt,
  group,
  width = 160,
  height = 100,
}: LightboxProps) {
  // Generate a stable, unique group id for this instance unless the
  // caller wants to chain into a named gallery.
  const groupId = useMemo(() => {
    if (group) return group;
    groupCounter += 1;
    return `lightbox-${groupCounter}`;
  }, [group]);

  useEffect(() => {
    bindLightbox();
    return () => {
      unbindLightbox();
    };
  }, []);

  return (
    <Box
      component="a"
      href={src}
      data-fancybox={groupId}
      data-src={src}
      data-caption={caption}
      aria-label={alt ?? caption ?? "Open preview"}
      sx={{
        display: "inline-block",
        position: "relative",
        width,
        height,
        borderRadius: 1,
        overflow: "hidden",
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: tokens.color.bg.surfaceRaised,
        transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}`,
        "&:hover": {
          borderColor: tokens.color.accent.violet,
          "& [data-lightbox-overlay]": {
            opacity: 1,
          },
        },
      }}
    >
      <Box
        component="img"
        src={src}
        srcSet={srcSet}
        alt={alt ?? caption ?? ""}
        sx={{
          width: "100%",
          height: "100%",
          objectFit: "cover",
          display: "block",
        }}
      />
      <Box
        data-lightbox-overlay
        sx={{
          position: "absolute",
          inset: 0,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          bgcolor: tokens.color.bg.overlay,
          color: tokens.color.text.primary,
          opacity: 0,
          transition: `opacity ${tokens.motion.fast} ${tokens.motion.snap}`,
        }}
      >
        <ZoomInRounded sx={{ fontSize: 22 }} />
      </Box>
    </Box>
  );
}

export default Lightbox;
