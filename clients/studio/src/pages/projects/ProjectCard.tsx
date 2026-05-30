import { Box, CircularProgress, IconButton, Stack, Tooltip, Typography } from '@mui/material';
import { Icon } from '../../icons';
import { BUCKET_LABEL, bucketColor, bucketFor } from './projectStatus';

// ─────────────────────────────────────────────────────────────────────────
// ProjectCard — a home-grade glass feature tile (mx.md › Cards: "should not
// feel like a card"): faint fill + hairline + backdrop blur, a status mark in
// semantic neon, hover lift on the slow neon easing, and a soft accent rim that
// blooms on hover. The whole surface opens the project; a hover-revealed delete
// affordance and an explicit "open" glyph make the affordances legible. Zero
// raw color literals — all marks/effects come from the studio theme.
// ─────────────────────────────────────────────────────────────────────────

export type ProjectCardModel = {
  id: string;
  name: string;
  description?: string | null;
  idea?: string | null;
  status: string;
  updatedAt?: string | null;
};

export function ProjectCard(props: {
  project: ProjectCardModel;
  opening: boolean;
  onOpen: () => void;
  onDelete: () => void;
}) {
  const { project: p, opening, onOpen, onDelete } = props;
  const bucket = bucketFor(p.status);
  const body = p.description || p.idea || 'No description yet.';
  const updated = p.updatedAt ? new Date(p.updatedAt).toLocaleDateString() : null;

  return (
    <Box
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onOpen();
        }
      }}
      sx={(theme) => ({
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
        p: 2.5,
        cursor: 'pointer',
        overflow: 'hidden',
        backgroundColor: theme.palette.cardBg,
        border: `1px solid ${theme.palette.cardBorder}`,
        borderRadius: `${theme.studio.effect.card.radius}px`,
        backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        transition: `transform ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}, box-shadow ${theme.studio.motion.base}`,
        // Soft accent bloom anchored to the status mark, revealed on hover.
        '&::before': {
          content: '""',
          position: 'absolute',
          inset: 0,
          pointerEvents: 'none',
          opacity: 0,
          background: `radial-gradient(420px 220px at 0% 0%, ${bucketColor(theme, bucket)}1F, transparent 62%)`,
          transition: `opacity ${theme.studio.motion.base}`,
        },
        '&:hover, &:focus-visible': {
          transform: 'translateY(-3px)',
          borderColor: theme.palette.borderSubtle,
          boxShadow: theme.studio.effect.promptBuilder.glow,
          outline: 'none',
        },
        '&:hover::before, &:focus-visible::before': { opacity: 1 },
        '&:hover .if-card-actions, &:focus-within .if-card-actions': { opacity: 1 },
        '&:hover .if-open, &:focus-visible .if-open': { transform: 'translate(2px, -2px)' },
      })}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={1.5}>
        <Stack direction="row" alignItems="center" spacing={1.25} sx={{ minWidth: 0 }}>
          <Box
            aria-hidden
            sx={(theme) => ({
              width: 10,
              height: 10,
              flexShrink: 0,
              borderRadius: theme.studio.radius.pill,
              backgroundColor: bucketColor(theme, bucket),
              boxShadow: `0 0 10px ${bucketColor(theme, bucket)}`,
            })}
          />
          <Typography
            variant="caption"
            sx={(theme) => ({
              color: bucketColor(theme, bucket),
              fontWeight: 700,
              textTransform: 'uppercase',
              letterSpacing: '0.1em',
            })}
          >
            {BUCKET_LABEL[bucket]}
          </Typography>
        </Stack>

        <Stack direction="row" alignItems="center" spacing={0.5}>
          {opening && <CircularProgress size={15} />}
          <IconButton
            className="if-card-actions"
            size="small"
            aria-label={`Delete ${p.name}`}
            onClick={(e) => {
              e.stopPropagation();
              onDelete();
            }}
            sx={(theme) => ({
              opacity: 0,
              color: theme.palette.text.disabled,
              transition: `opacity ${theme.studio.motion.fast}, color ${theme.studio.motion.fast}`,
              '&:hover': { color: theme.studio.neon.danger },
            })}
          >
            <Icon name="trash" size={16} strokeWidth={1.8} />
          </IconButton>
        </Stack>
      </Stack>

      <Stack direction="row" alignItems="flex-start" justifyContent="space-between" spacing={1}>
        <Typography variant="h6" sx={{ fontWeight: 700, lineHeight: 1.25, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {p.name}
        </Typography>
        <Box
          className="if-open"
          aria-hidden
          sx={(theme) => ({
            color: theme.palette.text.disabled,
            flexShrink: 0,
            display: 'inline-flex',
            transition: `transform ${theme.studio.motion.base}, color ${theme.studio.motion.base}`,
          })}
        >
          <Icon name="arrowUpRight" size={18} strokeWidth={2} />
        </Box>
      </Stack>

      <Typography
        variant="body2"
        color="text.secondary"
        sx={{
          minHeight: '2.6em',
          display: '-webkit-box',
          WebkitLineClamp: 2,
          WebkitBoxOrient: 'vertical',
          overflow: 'hidden',
        }}
      >
        {body}
      </Typography>

      <Stack direction="row" alignItems="center" spacing={0.75} sx={{ mt: 'auto', pt: 0.5 }}>
        <Tooltip title={p.status} placement="top">
          <Typography
            variant="caption"
            sx={(theme) => ({
              maxWidth: 160,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              color: theme.palette.text.disabled,
            })}
          >
            {p.status}
          </Typography>
        </Tooltip>
        {updated && (
          <>
            <Box aria-hidden sx={(theme) => ({ width: 3, height: 3, borderRadius: theme.studio.radius.pill, backgroundColor: theme.palette.text.disabled })} />
            <Stack direction="row" alignItems="center" spacing={0.5} sx={(theme) => ({ color: theme.palette.text.disabled })}>
              <Icon name="clock" size={12} strokeWidth={1.8} />
              <Typography variant="caption" color="inherit">
                {updated}
              </Typography>
            </Stack>
          </>
        )}
      </Stack>
    </Box>
  );
}
