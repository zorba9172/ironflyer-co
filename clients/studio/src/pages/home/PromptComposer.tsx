import * as React from 'react';
import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import InputBase from '@mui/material/InputBase';
import { Icon, type IconName } from '../../icons';

const PLACEHOLDER = 'What do you want to build today?';

const TOOLS: { name: IconName; label: string }[] = [
  { name: 'add', label: 'Attach' },
  { name: 'download', label: 'Import' },
  { name: 'templates', label: 'From template' },
];

// The product. A white prompt card (dark glass in dark) with a violet focus
// glow and an aurora-gradient "Start building" CTA. Nothing on Home competes
// with this surface — generous radius, soft elevation, calm tool chips below.
export function PromptComposer(props: {
  value: string;
  onChange: (v: string) => void;
  planFirst: boolean;
  onPlanFirstChange: (v: boolean) => void;
  onTool?: (tool: string) => void;
  onSubmit: () => void;
  inputRef?: React.Ref<HTMLTextAreaElement | HTMLInputElement>;
}) {
  const { value, onChange, planFirst, onPlanFirstChange, onTool, onSubmit, inputRef } = props;
  const [focused, setFocused] = React.useState(false);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      onSubmit();
    }
  };

  return (
    <Box
      sx={(theme) => ({
        position: 'relative',
        width: '100%',
        p: { xs: 2, md: 2.5 },
        borderRadius: `${theme.studio.effect.promptBuilder.radius}px`,
        border: `1px solid ${focused ? theme.palette.primary.main : theme.palette.cardBorder}`,
        backgroundColor: theme.palette.background.paper,
        boxShadow: focused ? theme.studio.effect.glow.focus : theme.studio.effect.card.shadow,
        transition: `border-color ${theme.studio.motion.base}, box-shadow ${theme.studio.motion.base}`,
      })}
    >
      <InputBase
        inputRef={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        placeholder={PLACEHOLDER}
        autoFocus
        multiline
        minRows={2}
        sx={(theme) => ({
          width: '100%',
          color: theme.palette.text.primary,
          ...theme.typography.h5,
          fontWeight: 500,
          lineHeight: 1.45,
          alignItems: 'flex-start',
          '& textarea::placeholder, & input::placeholder': {
            color: theme.palette.text.disabled,
            opacity: 1,
          },
        })}
      />

      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        flexWrap="wrap"
        useFlexGap
        sx={{ mt: { xs: 1.5, md: 2 }, gap: { xs: 1.25, md: 1.5 } }}
      >
        <Stack direction="row" alignItems="center" useFlexGap sx={{ gap: 1, flexWrap: 'wrap' }}>
          {TOOLS.map(({ name, label }) => (
            <Button
              key={label}
              variant="text"
              onClick={() => onTool?.(label)}
              startIcon={<Icon name={name} size={16} />}
              sx={(theme) => ({
                minHeight: 36,
                px: 1.4,
                borderRadius: `${theme.studio.radius.sm}px`,
                color: theme.palette.text.secondary,
                fontWeight: 600,
                border: `1px solid ${theme.palette.divider}`,
                '&:hover': {
                  bgcolor: theme.palette.surfaceHover,
                  color: theme.palette.text.primary,
                  borderColor: theme.palette.cardBorder,
                },
              })}
            >
              {label}
            </Button>
          ))}

          <Stack direction="row" alignItems="center" sx={{ gap: 0.5, ml: 0.5 }}>
            <Switch
              size="small"
              checked={planFirst}
              onChange={(e) => onPlanFirstChange(e.target.checked)}
              inputProps={{ 'aria-label': 'Plan-first mode' }}
            />
            <Typography variant="body2" sx={{ color: 'text.secondary', fontWeight: 600 }}>
              Plan first
            </Typography>
          </Stack>
        </Stack>

        <Button
          variant="contained"
          color="primary"
          endIcon={<Icon name="arrowRight" size={18} />}
          onClick={onSubmit}
          sx={(theme) => ({
            height: { xs: 40, md: `${theme.studio.effect.cta.height}px` },
            px: 2.5,
            borderRadius: `${theme.studio.effect.cta.radius}px`,
          })}
        >
          Start building
        </Button>
      </Stack>
    </Box>
  );
}
