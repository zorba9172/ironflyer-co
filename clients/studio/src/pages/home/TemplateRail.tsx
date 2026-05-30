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

type Template = { id: string; label: string; icon: IconType };

const TEMPLATES: readonly Template[] = [
  { id: 'internal-tool', label: 'Tasks & Workflows', icon: LuWrench },
  { id: 'saas-dashboard', label: 'CRM & Sales', icon: LuLayoutDashboard },
  { id: 'marketplace', label: 'Content & Sites', icon: LuStore },
  { id: 'admin-panel', label: 'Finance', icon: LuShield },
  { id: 'mobile-app', label: 'Booking', icon: LuSmartphone },
  { id: 'more-templates', label: '••• More', icon: LuLayers },
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
            height: 50,
            px: 1.6,
            borderRadius: `${theme.studio.radius.sm}px`,
            fontWeight: 800,
            fontSize: { xs: '0.86rem', md: '1rem' },
            color: theme.palette.text.primary,
            backgroundColor: 'rgba(255,255,255,0.78)',
            border: '1px solid rgba(255,255,255,0.52)',
            transition: `transform ${theme.studio.motion.fast}, border-color ${theme.studio.motion.fast}, background-color ${theme.studio.motion.fast}`,
            '& .MuiChip-icon': {
              display: 'none',
              ml: 0.25,
              mr: -0.25,
            },
            '& .MuiChip-label': { px: 1 },
            '&:hover': {
              transform: 'translateY(-1px)',
              borderColor: theme.palette.primary.main,
              backgroundColor: theme.palette.background.paper,
            },
          })}
        />
      ))}
    </Box>
  );
}
