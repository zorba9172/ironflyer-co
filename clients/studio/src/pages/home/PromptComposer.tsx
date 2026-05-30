import * as React from 'react';
import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Chip from '@mui/material/Chip';
import Switch from '@mui/material/Switch';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import InputBase from '@mui/material/InputBase';
import IconButton from '@mui/material/IconButton';
import Tooltip from '@mui/material/Tooltip';
import useMediaQuery from '@mui/material/useMediaQuery';
import { LuArrowRight, LuChevronDown, LuInfo, LuWallet, LuWandSparkles } from 'react-icons/lu';
import { studioTokens } from '../../theme';

const PLACEHOLDER =
  'A CRM with contacts, deals, and a kanban pipeline; notes and follow-up reminders.';

export function PromptComposer(props: {
  value: string;
  onChange: (v: string) => void;
  planFirst: boolean;
  onPlanFirstChange: (v: boolean) => void;
  budget?: string;
  onSubmit: () => void;
  inputRef?: React.Ref<HTMLTextAreaElement | HTMLInputElement>;
}) {
  const { value, onChange, planFirst, onPlanFirstChange, budget = '$27.00', onSubmit, inputRef } = props;
  const finePointer = useMediaQuery('(pointer: fine)');

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      onSubmit();
    }
  };

  // The prompt builder is the always-dark neon centerpiece in BOTH modes.
  const darkText = studioTokens.modes.dark.textPrimary;
  const darkMuted = studioTokens.modes.dark.textSecondary;

  return (
    <Box
      sx={(theme) => ({
        position: 'relative',
        overflow: 'hidden',
        maxWidth: 1036,
        mx: 'auto',
        width: '100%',
        minHeight: { xs: 252, md: 236 },
        p: { xs: 2.5, sm: 3.5, md: 4 },
        borderRadius: `${theme.studio.effect.promptBuilder.radius}px`,
        border: `1px solid ${theme.studio.effect.promptBuilder.borderColor}`,
        background: theme.studio.effect.promptBuilder.bg,
        backdropFilter: `blur(${theme.studio.effect.promptBuilder.blur}px)`,
        WebkitBackdropFilter: `blur(${theme.studio.effect.promptBuilder.blur}px)`,
        boxShadow: theme.studio.effect.promptBuilder.glow,
      })}
    >
      <InputBase
        inputRef={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={PLACEHOLDER}
        autoFocus={finePointer}
        multiline
        minRows={2}
        sx={{
          width: '100%',
          maxHeight: { xs: 132, md: 112 },
          overflowY: 'auto',
          color: darkText,
          fontSize: { xs: '1.0625rem', md: '1.5rem' },
          lineHeight: { xs: 1.5, md: 1.55 },
          fontFamily: studioTokens.font.mono,
          alignItems: 'flex-start',
          '& textarea::placeholder, & input::placeholder': {
            color: darkMuted,
            opacity: 1,
          },
        }}
      />

      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        flexWrap="wrap"
        useFlexGap
        sx={{ mt: 2.5, gap: 2 }}
      >
        <Stack direction="row" alignItems="center" useFlexGap sx={{ gap: 2, flexWrap: 'wrap' }}>
          <Chip
            icon={<LuWallet size={15} />}
            deleteIcon={<LuChevronDown size={14} />}
            onDelete={() => undefined}
            label={`Budget ${budget}`}
            sx={(theme) => ({
              height: 42,
              px: 0.5,
              borderRadius: `${theme.studio.radius.pill}px`,
              border: `1px solid ${studioTokens.modes.dark.cardBorder}`,
              backgroundColor: studioTokens.modes.dark.cardBg,
              color: darkText,
              fontWeight: 600,
              '& .MuiChip-icon': { color: theme.studio.neon.blue, ml: 0.75 },
              '& .MuiChip-label': { px: 1, fontSize: '0.8125rem' },
              '& .MuiChip-deleteIcon': { color: darkMuted, mr: 0.75 },
            })}
          />

          <Stack
            direction="row"
            alignItems="center"
            sx={(theme) => ({
              height: 42,
              gap: 0.75,
              px: 1.25,
              borderRadius: `${theme.studio.radius.pill}px`,
              border: `1px solid ${studioTokens.modes.dark.cardBorder}`,
              backgroundColor: studioTokens.modes.dark.cardBg,
            })}
          >
            <Typography
              variant="body2"
              sx={{ color: darkMuted, fontWeight: 600 }}
            >
              Plan-first
            </Typography>
            <Tooltip title="Plan the app before executing changes">
              <Box component="span" sx={{ display: 'inline-flex', color: darkMuted }}>
                <LuInfo size={14} />
              </Box>
            </Tooltip>
            <Switch
              checked={planFirst}
              onChange={(e) => onPlanFirstChange(e.target.checked)}
              inputProps={{ 'aria-label': 'Plan-first mode' }}
              sx={(theme) => ({
                '& .MuiSwitch-switchBase.Mui-checked + .MuiSwitch-track': {
                  backgroundImage: theme.studio.gradient.cta,
                  opacity: 1,
                },
                '& .MuiSwitch-track': {
                  backgroundColor: studioTokens.modes.dark.surfaceHover,
                  opacity: 1,
                },
                '& .MuiSwitch-thumb': { color: darkText },
              })}
            />
          </Stack>
        </Stack>

        <Stack direction="row" alignItems="center" spacing={1.25} sx={{ width: { xs: '100%', sm: 'auto' } }}>
          <Tooltip title="Enhance prompt">
            <IconButton
              aria-label="Enhance prompt"
              sx={(theme) => ({
                display: { xs: 'none', sm: 'inline-flex' },
                width: 56,
                height: 56,
                borderRadius: `${theme.studio.effect.cta.radius}px`,
                color: theme.studio.neon.pink,
                border: `1px solid ${studioTokens.modes.dark.cardBorder}`,
                backgroundColor: studioTokens.modes.dark.cardBg,
                boxShadow: `0 0 22px ${theme.studio.neon.pink}22`,
                '&:hover': { backgroundColor: studioTokens.modes.dark.surfaceHover },
              })}
            >
              <LuWandSparkles size={22} />
            </IconButton>
          </Tooltip>
          <Button
            variant="contained"
            color="primary"
            endIcon={<LuArrowRight size={18} />}
            onClick={onSubmit}
            sx={(theme) => ({
              height: `${theme.studio.effect.cta.height}px`,
              borderRadius: `${theme.studio.effect.cta.radius}px`,
              px: 3.25,
              fontSize: '1rem',
              width: { xs: '100%', sm: 'auto' },
            })}
          >
            Build it
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}
