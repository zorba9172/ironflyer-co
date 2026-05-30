import { Box, Chip } from '@mui/material';
import { Icon } from '../../icons';
import type { IconName } from '../../icons';

// Starting-point accelerators that sit directly under the prompt builder.
// Each chip seeds a runnable scaffold so the operator lands on a live app in
// seconds. Soft uniform pills — calm, never competing with the prompt CTA.
type Template = { id: string; label: string; icon: IconName };

const TEMPLATES: readonly Template[] = [
  { id: 'shell', label: 'Start with code', icon: 'code' },
  { id: 'marketplace', label: 'Marketplace with payments', icon: 'store' },
  { id: 'saas-dashboard', label: 'AI dashboard', icon: 'dashboard' },
  { id: 'landing', label: 'Landing page', icon: 'box' },
  { id: 'internal-tool', label: 'Internal tool', icon: 'wrench' },
  { id: 'more-templates', label: 'More templates', icon: 'layers' },
];

export function TemplateRail(props: { onSelect: (id: string) => void }) {
  return (
    <Box
      sx={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        gap: 1,
      }}
    >
      {TEMPLATES.map(({ id, label, icon }) => (
        <Chip
          key={id}
          variant="outlined"
          onClick={() => props.onSelect(id)}
          icon={<Icon name={icon} size={15} />}
          label={label}
          sx={(theme) => ({
            height: 36,
            px: 0.75,
            borderRadius: `${theme.studio.radius.pill}px`,
            fontWeight: 600,
            color: theme.palette.text.secondary,
            backgroundColor: theme.palette.background.paper,
            border: `1px solid ${theme.palette.cardBorder}`,
            transition: `transform ${theme.studio.motion.fast}, border-color ${theme.studio.motion.fast}, color ${theme.studio.motion.fast}, background-color ${theme.studio.motion.fast}`,
            '& .MuiChip-icon': { color: theme.palette.text.disabled, ml: 0.75, mr: -0.25 },
            '& .MuiChip-label': { px: 1 },
            '&:hover': {
              transform: 'translateY(-1px)',
              color: theme.palette.text.primary,
              borderColor: theme.palette.primary.main,
              backgroundColor: theme.palette.surfaceHover,
              '& .MuiChip-icon': { color: theme.palette.primary.main },
            },
          })}
        />
      ))}
    </Box>
  );
}
