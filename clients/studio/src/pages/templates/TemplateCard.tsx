import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { Icon, AssetImage } from '../../icons';
import { studioTokens } from '../../theme';
import type { Template } from './data';

// ─────────────────────────────────────────────────────────────────────────
// TemplateCard — a single starter blueprint, rendered as one uniform glass
// proof surface (Component Law: soft border, 12–14px radius, hover lift). It
// mirrors the home TemplateRail energy at full-page scale: a neon-washed
// thumbnail carrying an illustrated 3D mark (AssetImage) with a blueprint glyph
// (Icon), a readiness meter (viz-first), a gate proof chip, the stack line, and
// a clear "Use template" CTA.
//
// Interaction follows the Vercel guideline that hover/active raise contrast and
// only touch GPU-cheap transform/opacity. The whole card is clickable; the CTA
// is the explicit affordance. The click contract is unchanged: onUse() drives
// startFromPrompt + navigate from the parent. All glyphs route through the Icon
// barrel; all color/radius/blur/motion flow through the studio theme.
// ─────────────────────────────────────────────────────────────────────────

export function TemplateCard(props: { template: Template; onUse: () => void }) {
  const { template: t, onUse } = props;

  return (
    <Box
      role="button"
      tabIndex={0}
      aria-label={`Use the ${t.name} template`}
      onClick={onUse}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onUse();
        }
      }}
      sx={(theme) => ({
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        cursor: 'pointer',
        overflow: 'hidden',
        backgroundColor: theme.palette.cardBg,
        border: `1px solid ${theme.palette.cardBorder}`,
        borderRadius: `${theme.studio.effect.card.radius}px`,
        backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        transition: `transform ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}, box-shadow ${theme.studio.motion.base}`,
        willChange: 'transform',
        '&:hover, &:focus-visible': {
          transform: 'translateY(-4px)',
          borderColor: `${t.accent}66`,
          boxShadow: theme.studio.effect.promptBuilder.glow,
          outline: 'none',
        },
        '&:hover .tpl-thumb-art, &:focus-visible .tpl-thumb-art': {
          transform: 'scale(1.06)',
        },
        '&:hover .tpl-cta, &:focus-visible .tpl-cta': { opacity: 1 },
      })}
    >
      {/* Thumbnail — neon-washed atmosphere + illustrated 3D mark + blueprint
          glyph corner. No raw circles; the wash derives from the accent token. */}
      <Box
        aria-hidden
        sx={(theme) => ({
          position: 'relative',
          height: 144,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderBottom: `1px solid ${theme.palette.divider}`,
          background: `radial-gradient(120% 140% at 16% 0%, ${t.accent}26, transparent 62%), radial-gradient(120% 140% at 100% 100%, ${theme.studio.neon.pink}1F, transparent 60%)`,
          overflow: 'hidden',
          '&::after': {
            content: '""',
            position: 'absolute',
            inset: 0,
            backgroundImage: `linear-gradient(to right, ${theme.studio.effect.gridLine} 1px, transparent 1px), linear-gradient(to bottom, ${theme.studio.effect.gridLine} 1px, transparent 1px)`,
            backgroundSize: '34px 34px',
            maskImage: `radial-gradient(80% 80% at 50% 40%, ${theme.palette.common.black}, transparent 78%)`,
            WebkitMaskImage: `radial-gradient(80% 80% at 50% 40%, ${theme.palette.common.black}, transparent 78%)`,
            opacity: 0.7,
          },
        })}
      >
        <Box
          className="tpl-thumb-art"
          sx={(theme) => ({
            position: 'relative',
            zIndex: 1,
            display: 'inline-flex',
            transition: `transform ${theme.studio.motion.base}`,
          })}
        >
          <AssetImage id={t.art} size={76} alt={`${t.name} template`} />
        </Box>

        {/* Blueprint glyph chip, top-left — the semantic mark for the category. */}
        <Box
          aria-hidden
          sx={(theme) => ({
            position: 'absolute',
            top: 12,
            left: 12,
            zIndex: 1,
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 32,
            height: 32,
            borderRadius: `${theme.studio.radius.sm}px`,
            color: t.accent,
            backgroundColor: theme.palette.surfaceRaised,
            border: `1px solid ${t.accent}40`,
          })}
        >
          <Icon name={t.icon} size={17} strokeWidth={1.6} />
        </Box>

        {/* Ship-horizon tag, top-right. */}
        <Chip
          size="small"
          label={t.ships}
          icon={<Icon name="zap" size={12} />}
          sx={(theme) => ({
            position: 'absolute',
            top: 12,
            right: 12,
            zIndex: 1,
            height: 24,
            borderRadius: theme.studio.radius.pill,
            backgroundColor: theme.palette.surfaceRaised,
            border: `1px solid ${theme.palette.borderSubtle}`,
            color: theme.palette.text.secondary,
            fontWeight: 600,
            '& .MuiChip-icon': { color: theme.studio.neon.warning, ml: 0.5, mr: -0.25 },
            '& .MuiChip-label': { px: 0.75 },
          })}
        />
      </Box>

      {/* Body. */}
      <Box sx={{ p: 2.5, display: 'flex', flexDirection: 'column', gap: 1.25, flex: 1 }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
          <Typography variant="h6">{t.name}</Typography>
          <Chip
            size="small"
            label={t.cat}
            sx={(theme) => ({
              height: 22,
              borderRadius: theme.studio.radius.pill,
              backgroundColor: theme.palette.surfaceHover,
              color: theme.palette.text.secondary,
              fontWeight: 600,
              '& .MuiChip-label': { px: 1 },
            })}
          />
        </Stack>

        <Typography variant="body2" color="text.secondary" sx={{ minHeight: 40 }}>
          {t.desc}
        </Typography>

        {/* Readiness meter — viz-first mirror of how production-ready the
            starter lands (quiet track + one neon arc, per the viz law). */}
        <Stack spacing={0.75} sx={{ mt: 0.25 }}>
          <Stack direction="row" alignItems="center" justifyContent="space-between">
            <Typography
              variant="overline"
              sx={(theme) => ({ color: theme.palette.text.secondary, lineHeight: 1 })}
            >
              Readiness
            </Typography>
            <Typography variant="caption" sx={{ color: t.accent, fontWeight: 700 }}>
              {t.readiness}%
            </Typography>
          </Stack>
          <Box
            aria-hidden
            sx={(theme) => ({
              position: 'relative',
              height: 5,
              borderRadius: theme.studio.radius.pill,
              backgroundColor: theme.palette.surfaceHover,
              overflow: 'hidden',
            })}
          >
            <Box
              sx={(theme) => ({
                position: 'absolute',
                inset: 0,
                width: `${t.readiness}%`,
                borderRadius: theme.studio.radius.pill,
                background: `linear-gradient(90deg, ${t.accent}, ${theme.studio.neon.pink})`,
              })}
            />
          </Box>
        </Stack>

        <Box sx={{ flex: 1 }} />

        {/* Proof footer: gate count + stack line, then the CTA. */}
        <Stack
          direction="row"
          alignItems="center"
          spacing={1}
          sx={(theme) => ({
            pt: 1.5,
            mt: 0.25,
            borderTop: `1px solid ${theme.palette.borderSubtle}`,
          })}
        >
          <Box aria-hidden sx={{ display: 'inline-flex', flexShrink: 0 }}>
            <Icon name="shieldCheck" size={14} color="muted" />
          </Box>
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600, flexShrink: 0 }}>
            {t.gates} gates
          </Typography>
          <Typography
            variant="caption"
            sx={(theme) => ({
              fontFamily: studioTokens.font.mono,
              color: theme.palette.text.disabled,
              ml: 'auto',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            })}
          >
            {t.stack}
          </Typography>
        </Stack>

        <Button
          variant="contained"
          color="primary"
          fullWidth
          endIcon={<Icon name="arrowRight" size={16} />}
          className="tpl-cta"
          onClick={(e) => {
            e.stopPropagation();
            onUse();
          }}
          sx={(theme) => ({
            mt: 1,
            opacity: 0.92,
            transition: `opacity ${theme.studio.motion.fast}`,
          })}
        >
          Use template
        </Button>
      </Box>
    </Box>
  );
}
