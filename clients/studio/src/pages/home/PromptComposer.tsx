import * as React from 'react';
import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import InputBase from '@mui/material/InputBase';
import IconButton from '@mui/material/IconButton';
import { LuArrowRight, LuMic, LuPlus, LuSlidersHorizontal, LuWorkflow } from 'react-icons/lu';

const PLACEHOLDER =
  'Describe the app you want to create...';

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
        overflow: 'hidden',
        maxWidth: 1040,
        mx: 'auto',
        width: '100%',
        minHeight: { xs: 190, md: 218 },
        p: { xs: 1.8, sm: 3 },
        borderRadius: `${theme.studio.effect.promptBuilder.radius}px`,
        border: `1px solid ${theme.palette.cardBorder}`,
        borderBottomColor: theme.palette.primary.main,
        background: theme.palette.background.paper,
        boxShadow: '0 18px 42px rgba(24,22,20,0.08)',
      })}
    >
      <InputBase
        inputRef={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={PLACEHOLDER}
        autoFocus
        multiline
        minRows={3}
        sx={(theme) => ({
          width: '100%',
          color: theme.palette.text.primary,
          fontSize: { xs: '1rem', md: '1.32rem' },
          lineHeight: 1.55,
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
        sx={{ mt: { xs: 1, md: 1.5 }, gap: { xs: 1.25, md: 2 } }}
      >
        <Stack direction="row" alignItems="center" useFlexGap sx={{ gap: 1, flexWrap: 'wrap' }}>
          {[LuPlus, LuWorkflow, LuSlidersHorizontal].map((Icon, index) => (
            <IconButton
              key={index}
              aria-label={index === 0 ? 'Add context' : index === 1 ? 'Choose workflow' : 'Prompt settings'}
              sx={(theme) => ({
                width: { xs: 38, md: 44 },
                height: { xs: 38, md: 44 },
                borderRadius: `${theme.studio.radius.sm}px`,
                border: `1px solid ${theme.palette.divider}`,
                bgcolor: theme.palette.background.paper,
                color: theme.palette.text.primary,
                boxShadow: '0 1px 2px rgba(24,22,20,0.05)',
                '&:hover': { bgcolor: theme.palette.surfaceHover },
              })}
            >
              <Icon size={20} />
            </IconButton>
          ))}

          <Stack direction="row" alignItems="center" sx={{ gap: 0.75 }}>
            <Typography
              variant="body2"
              sx={{ color: 'text.primary', fontWeight: 800 }}
            >
              Plan
            </Typography>
            <Switch
              checked={planFirst}
              onChange={(e) => onPlanFirstChange(e.target.checked)}
              inputProps={{ 'aria-label': 'Plan-first mode' }}
              sx={(theme) => ({
                '& .MuiSwitch-switchBase.Mui-checked + .MuiSwitch-track': {
                  backgroundColor: theme.palette.primary.main,
                  opacity: 1,
                },
                '& .MuiSwitch-track': {
                  backgroundColor: theme.palette.divider,
                  opacity: 1,
                },
                '& .MuiSwitch-thumb': { color: theme.palette.background.paper },
              })}
            />
          </Stack>
        </Stack>

        <Stack direction="row" spacing={1} alignItems="center">
          <Typography sx={{ display: { xs: 'none', md: 'block' }, color: 'text.disabled', fontSize: '0.8rem', fontWeight: 700 }}>
            Budget {budget}
          </Typography>
          <IconButton aria-label="Voice input" sx={{ color: 'text.primary' }}><LuMic size={20} /></IconButton>
          <Button
            variant="contained"
            color="primary"
            endIcon={<LuArrowRight size={18} />}
            onClick={onSubmit}
            sx={(theme) => ({
              height: { xs: 40, md: `${theme.studio.effect.cta.height}px` },
              minWidth: 56,
              borderRadius: `${theme.studio.effect.cta.radius}px`,
              px: 2,
              fontSize: '1rem',
            })}
          >
            Build
          </Button>
        </Stack>
      </Stack>
    </Box>
  );
}
