import { Box, Chip } from '@mui/material';
import type { IconType } from 'react-icons';
import {
  LuLayoutDashboard,
  LuStore,
  LuWrench,
  LuShield,
  LuSmartphone,
  LuLayers,
} from 'react-icons/lu';

// ─────────────────────────────────────────────────────────────────────────
// TemplateRail — quick-start chips under the prompt builder (mx.md › Templates,
// step 4 of the hierarchy: nothing competes with the prompt box). A centered,
// wrapping row of floating glass pills; each maps to a starter blueprint id.
// ─────────────────────────────────────────────────────────────────────────

type Template = { id: string; label: string; icon: IconType };

const TEMPLATES: readonly Template[] = [
  { id: 'saas-dashboard', label: 'SaaS dashboard', icon: LuLayoutDashboard },
  { id: 'marketplace', label: 'Marketplace', icon: LuStore },
  { id: 'internal-tool', label: 'Internal tool', icon: LuWrench },
  { id: 'admin-panel', label: 'Admin panel', icon: LuShield },
  { id: 'mobile-app', label: 'Mobile app', icon: LuSmartphone },
  { id: 'more-templates', label: 'More templates', icon: LuLayers },
];

export function TemplateRail(props: { onSelect: (id: string) => void }) {
  return (
    <Box
      sx={{
        display: 'flex',
        flexWrap: 'wrap',
        justifyContent: 'center',
        alignItems: 'center',
        gap: 1.5,
      }}
    >
      {TEMPLATES.map(({ id, label, icon: Icon }) => (
        <Chip
          key={id}
          variant="outlined"
          onClick={() => props.onSelect(id)}
          icon={<Icon size={16} aria-hidden />}
          label={label}
          sx={(theme) => ({
            height: 44,
            maxWidth: '100%',
            px: 1.75,
            borderRadius: theme.studio.radius.pill,
            fontWeight: 600,
            color: theme.palette.text.secondary,
            backgroundColor: theme.palette.cardBg,
            border: `1px solid ${theme.palette.divider}`,
            backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
            transition: `transform ${theme.studio.motion.fast}, border-color ${theme.studio.motion.fast}, color ${theme.studio.motion.fast}, background-color ${theme.studio.motion.fast}`,
            '& .MuiChip-icon': {
              color: id === 'more-templates' ? theme.studio.neon.violet : theme.palette.text.disabled,
              ml: 0.25,
              mr: -0.25,
            },
            '& .MuiChip-label': { px: 1, overflow: 'hidden', textOverflow: 'ellipsis' },
            '&:hover': {
              transform: 'translateY(-2px)',
              borderColor: theme.studio.neon.violet,
              backgroundColor: theme.palette.surfaceHover,
              color: theme.palette.text.primary,
            },
          })}
        />
      ))}
    </Box>
  );
}
