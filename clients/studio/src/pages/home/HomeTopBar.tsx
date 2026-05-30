import { Box, Button, IconButton, InputBase, Stack, Tooltip } from '@mui/material';
import { Icon } from '../../icons';
import { Hero } from './Hero';

// The home masthead. A thin utility row (search + notifications + the primary
// "New project" CTA) sits at the top, and the warm greeting (Bricolage via the
// Hero h2) reads full-width beneath it so the headline never wraps. Nothing
// here competes with the prompt builder below it.
export function HomeTopBar(props: { name?: string; onNewProject: () => void; onSearch?: () => void }) {
  const { name, onNewProject, onSearch } = props;
  return (
    <Stack useFlexGap sx={{ gap: { xs: 2, md: 2.5 }, width: '100%' }}>
      <Stack direction="row" alignItems="center" spacing={1.25} sx={{ justifyContent: 'flex-end', flexWrap: 'wrap' }}>
        <Box
          sx={(theme) => ({
            display: { xs: 'none', sm: 'flex' },
            alignItems: 'center',
            gap: 1,
            minWidth: 200,
            px: 1.5,
            height: 40,
            borderRadius: `${theme.studio.radius.md}px`,
            border: `1px solid ${theme.palette.cardBorder}`,
            bgcolor: theme.palette.background.paper,
            color: theme.palette.text.secondary,
            transition: `border-color ${theme.studio.motion.fast}`,
            '&:focus-within': { borderColor: theme.palette.primary.main },
          })}
        >
          <Icon name="search" size={16} />
          <InputBase
            placeholder="Search projects"
            onFocus={onSearch}
            sx={(theme) => ({ flex: 1, ...theme.typography.body2, color: theme.palette.text.primary })}
          />
        </Box>

        <Tooltip title="Notifications" arrow>
          <IconButton
            aria-label="Notifications"
            sx={(theme) => ({
              color: theme.palette.text.secondary,
              border: `1px solid ${theme.palette.cardBorder}`,
              bgcolor: theme.palette.background.paper,
              transition: `color ${theme.studio.motion.fast}, border-color ${theme.studio.motion.fast}`,
              '&:hover': { color: theme.palette.text.primary, borderColor: theme.palette.primary.main },
            })}
          >
            <Icon name="bell" size={18} />
          </IconButton>
        </Tooltip>

        <Button
          variant="contained"
          color="primary"
          startIcon={<Icon name="add" size={18} />}
          onClick={onNewProject}
          sx={{ whiteSpace: 'nowrap', px: 2.25 }}
        >
          New project
        </Button>
      </Stack>

      <Hero name={name} />
    </Stack>
  );
}
