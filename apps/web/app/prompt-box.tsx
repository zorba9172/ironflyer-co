'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Add, Article, ArrowUpward, AttachFile, AutoAwesome, Close, ContentCopy,
  DesignServices, FactCheck, History, Hub, Image, InsertDriveFile, Save, Tune,
} from '@mui/icons-material';
import {
  Box, Button, Chip, Dialog, DialogContent, DialogTitle, IconButton, Menu,
  MenuItem, Stack, TextField, Tooltip, Typography,
} from '@mui/material';
import { tokens } from '../../../packages/design-tokens';

interface PromptBoxProps {
  value?: string;
  onChange?: (value: string) => void;
  onSubmit?: (value: string, mode: 'build' | 'plan') => void;
  busy?: boolean;
  error?: string | null;
  placeholder?: string;
  cta?: string;
  size?: 'hero' | 'dashboard' | 'preview';
}

const designPresets = [
  'Output-inspired dark product UI',
  'Dense operator dashboard',
  'Editorial launch page',
  'Minimal internal tool',
];

const connectorOptions = ['GitHub', 'Figma', 'Supabase', 'Stripe', 'Postgres', 'Vercel'];

const qualityGates = ['Spec', 'UX', 'Architecture', 'Code', 'Tests', 'Security', 'Deploy'];

const recipeOptions = [
  {
    label: 'SaaS App',
    value: 'Build a production-ready SaaS app with auth, teams, billing, dashboard analytics, admin settings, onboarding, and deployment.',
  },
  {
    label: 'Internal Tool',
    value: 'Build an internal operations tool with approvals, role-based access, audit history, reports, and a dense dashboard UI.',
  },
  {
    label: 'Launch Site',
    value: 'Build a product launch website with a full-bleed hero, waitlist form, pricing, FAQ, social proof, and analytics events.',
  },
  {
    label: 'Client Portal',
    value: 'Build a client portal with authentication, document uploads, project status, messaging, notifications, and admin controls.',
  },
];

const advancedOptions = {
  appTypes: ['SaaS', 'Internal tool', 'Marketplace', 'Portal', 'Landing page'],
  stacks: ['Next.js + Go', 'Next.js + Supabase', 'React + Node', 'MUI dashboard'],
  auth: ['JWT auth', 'Team workspaces', 'SSO-ready', 'Role-based access'],
  deployment: ['Vercel', 'Docker', 'Private cloud', 'PWA'],
};

export function PromptBox({
  value,
  onChange,
  onSubmit,
  busy = false,
  error = null,
  placeholder = 'Ask Ironflyer to build...',
  cta = 'Build',
  size = 'dashboard',
}: PromptBoxProps) {
  const [localValue, setLocalValue] = useState('');
  const [mode, setMode] = useState<'build' | 'plan'>('build');
  const [attachments, setAttachments] = useState<string[]>([]);
  const [designPreset, setDesignPreset] = useState('');
  const [connectors, setConnectors] = useState<string[]>([]);
  const [history, setHistory] = useState<string[]>([]);
  const [advanced, setAdvanced] = useState<Record<string, string>>({});
  const [selectedGates, setSelectedGates] = useState<string[]>(qualityGates);
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [designOpen, setDesignOpen] = useState(false);
  const [connectorsOpen, setConnectorsOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [recipesOpen, setRecipesOpen] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const prompt = value ?? localValue;
  const setPrompt = onChange ?? setLocalValue;
  const isHero = size === 'hero';
  const isPreview = size === 'preview';
  const isDashboard = size === 'dashboard';
  const isLightSurface = isHero || isPreview || isDashboard;
  const showFullTools = !isHero && !isPreview;
  const draftKey = `ironflyer.promptDraft.${size}`;
  const historyKey = 'ironflyer.promptHistory';

  useEffect(() => {
    if (isPreview || typeof window === 'undefined') return;
    const storedDraft = window.localStorage.getItem(draftKey);
    const storedHistory = window.localStorage.getItem(historyKey);
    if (!prompt && storedDraft) setPrompt(storedDraft);
    if (storedHistory) {
      try { setHistory(JSON.parse(storedHistory) as string[]); } catch {}
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (isPreview || typeof window === 'undefined' || !prompt.trim()) return;
    const t = window.setTimeout(() => {
      window.localStorage.setItem(draftKey, prompt);
    }, 500);
    return () => window.clearTimeout(t);
  }, [draftKey, isPreview, prompt]);

  const slashMatches = useMemo(() => {
    const trimmed = prompt.trim().toLowerCase();
    if (!trimmed.startsWith('/')) return [];
    return recipeOptions.filter((recipe) => `/${recipe.label.toLowerCase().replace(/\s+/g, '-')}`.includes(trimmed));
  }, [prompt]);

  const promptStrength = useMemo(() => {
    let score = 0;
    if (prompt.trim().length > 20) score += 20;
    if (prompt.trim().length > 120) score += 18;
    if (/user|role|admin|customer|team|client/i.test(prompt)) score += 12;
    if (/workflow|dashboard|portal|billing|auth|data|api|deploy/i.test(prompt)) score += 14;
    if (designPreset) score += 10;
    if (connectors.length) score += 8;
    if (attachments.length) score += 8;
    if (Object.values(advanced).some(Boolean)) score += 10;
    return Math.min(score, 100);
  }, [advanced, attachments.length, connectors.length, designPreset, prompt]);

  function submit(nextPrompt = prompt) {
    const clean = composePrompt(nextPrompt);
    if (!clean || busy) return;
    rememberPrompt(clean);
    trackPromptSubmit(clean);
    if (onSubmit) {
      onSubmit(clean, mode);
      return;
    }
    window.localStorage.setItem('ironflyer.pendingIdea', clean);
    window.location.href = '/login';
  }

  function composePrompt(raw: string) {
    const parts = [raw.trim()];
    if (mode === 'plan') parts.push('Mode: plan the product first, include architecture, UX, data model, milestones, and risks before code.');
    if (designPreset) parts.push(`Design direction: ${designPreset}.`);
    if (attachments.length) parts.push(`Attached context filenames: ${attachments.join(', ')}.`);
    if (connectors.length) parts.push(`Preferred connectors: ${connectors.join(', ')}.`);
    if (selectedGates.length) parts.push(`Required finisher gates: ${selectedGates.join(', ')}.`);
    const advancedLines = Object.entries(advanced)
      .filter(([, selected]) => selected)
      .map(([group, selected]) => `${group}: ${selected}`);
    if (advancedLines.length) parts.push(`Advanced options:\n${advancedLines.join('\n')}`);
    return parts.filter(Boolean).join('\n\n');
  }

  function rememberPrompt(clean: string) {
    if (typeof window === 'undefined') return;
    const next = [clean, ...history.filter((item) => item !== clean)].slice(0, 12);
    setHistory(next);
    window.localStorage.setItem(historyKey, JSON.stringify(next));
    window.localStorage.removeItem(draftKey);
  }

  function saveDraft() {
    if (typeof window === 'undefined') return;
    window.localStorage.setItem(draftKey, prompt);
  }

  function generateBrief() {
    const base = prompt.trim() || 'Build a production-ready application';
    setPrompt([
      base.replace(/\.$/, ''),
      '',
      'Product brief:',
      '- Target users:',
      '- Core jobs to be done:',
      '- Primary workflows:',
      '- Data model:',
      '- Integrations:',
      '- Permission model:',
      '- Key screens:',
      '- Empty, loading, error, and success states:',
      '- Test plan:',
      '- Security and deployment gates:',
    ].join('\n'));
  }

  function trackPromptSubmit(clean: string) {
    if (typeof window === 'undefined') return;
    const layer = (window as typeof window & { dataLayer?: unknown[] }).dataLayer;
    if (Array.isArray(layer)) {
      layer.push({
        event: 'ironflyer_prompt_submit',
        promptMode: mode,
        promptLength: clean.length,
        attachmentCount: attachments.length,
        connectorCount: connectors.length,
        hasDesignPreset: Boolean(designPreset),
      });
    }
  }

  function improvePrompt() {
    const clean = prompt.trim();
    const base = clean || 'Build a production-ready application';
    setPrompt([
      base.replace(/\.$/, ''),
      '',
      'Include: user roles, core workflows, responsive UI, data model, API boundaries, auth, permissions, loading/error states, tests, security review, and deployment plan.',
      'Use Ironflyer finisher gates for spec, UX, architecture, code, tests, security, and deploy.',
    ].join('\n'));
  }

  function addFiles(files: FileList | null) {
    if (!files?.length) return;
    setAttachments((current) => {
      const names = Array.from(files).map((file) => file.name);
      return Array.from(new Set([...current, ...names])).slice(0, 8);
    });
  }

  function toggleConnector(connector: string) {
    setConnectors((current) => (
      current.includes(connector)
        ? current.filter((item) => item !== connector)
        : [...current, connector]
    ));
  }

  function toggleGate(gate: string) {
    setSelectedGates((current) => (
      current.includes(gate)
        ? current.filter((item) => item !== gate)
        : [...current, gate]
    ));
  }

  async function copyReview() {
    try {
      await navigator.clipboard.writeText(composePrompt(prompt));
    } catch {}
  }

  return (
    <Box sx={{
	      width: '100%',
	      border: `1px solid ${isLightSurface ? 'rgba(17,17,17,0.12)' : tokens.color.border.strong}`,
	      borderRadius: '8px',
	      backgroundColor: isLightSurface ? 'rgba(244,240,232,0.96)' : 'rgba(13,14,15,0.92)',
	      boxShadow: isHero ? tokens.shadow.lg : isDashboard ? '0 18px 60px rgba(17,17,17,0.12)' : tokens.shadow.md,
	      color: isLightSurface ? tokens.color.text.inverse : tokens.color.text.primary,
	      overflow: 'hidden',
	      transition: `box-shadow ${tokens.motion.base} ${tokens.motion.curve}, transform ${tokens.motion.base} ${tokens.motion.curve}`,
	      '&:focus-within': {
	        boxShadow: isLightSurface ? '0 0 0 3px rgba(229,255,0,0.18)' : '0 0 0 3px rgba(229,255,0,0.24)',
	      },
	    }}>
      <Box sx={{ px: { xs: 1.5, sm: 2 }, pt: { xs: 1.5, sm: 2 } }}>
        <TextField
          fullWidth
          multiline
          minRows={isHero ? 3 : isPreview ? 1 : isDashboard ? 2 : 3}
          maxRows={8}
          value={prompt}
          onChange={(event) => setPrompt(event.target.value)}
          placeholder={placeholder}
          disabled={busy}
          inputProps={{ readOnly: isPreview }}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
              event.preventDefault();
              submit();
            }
          }}
          sx={{
            '& .MuiOutlinedInput-root': {
              p: 0,
              alignItems: 'flex-start',
              bgcolor: 'transparent',
              borderRadius: 0,
              color: isLightSurface ? tokens.color.text.inverse : tokens.color.text.primary,
              fontSize: isHero ? { xs: 17, md: 22 } : { xs: 15, md: 16 },
              fontWeight: 700,
              lineHeight: 1.35,
              '& fieldset': { border: 'none' },
            },
            '& .MuiInputBase-input': {
              color: isLightSurface ? tokens.color.text.inverse : tokens.color.text.primary,
              '&::placeholder': {
                opacity: 1,
                color: isLightSurface ? 'rgba(17,17,17,0.62)' : tokens.color.text.secondary,
              },
            },
          }}
        />
      </Box>

      <Stack
        direction={{ xs: 'column', sm: 'row' }}
        alignItems={{ xs: 'stretch', sm: 'center' }}
        justifyContent="space-between"
        spacing={1.5}
        sx={{ px: { xs: 1.25, sm: 1.5 }, py: 1.1 }}
      >
        <Stack direction="row" spacing={0.55} alignItems="center" useFlexGap flexWrap="wrap">
          <input
            ref={fileInputRef}
            type="file"
            multiple
            hidden
            onChange={(event) => addFiles(event.target.files)}
          />
          <Tooltip title="Attach screenshots or docs">
            <IconButton
              size="small"
              sx={toolButtonSx(isLightSurface)}
              onClick={(event) => {
                if (!isPreview) setMenuAnchor(event.currentTarget);
              }}
            >
              <Add fontSize="small" />
            </IconButton>
          </Tooltip>
          {showFullTools && (
            <>
              <ToolChip icon={<AttachFile />} label="Attach" onClick={() => fileInputRef.current?.click()} light={isLightSurface} />
              <ToolChip icon={<DesignServices />} label={designPreset ? 'Design set' : 'Design'} onClick={() => setDesignOpen(true)} active={!!designPreset} light={isLightSurface} />
              <ToolChip icon={<Hub />} label={connectors.length ? `${connectors.length} connectors` : 'Connectors'} onClick={() => setConnectorsOpen(true)} active={connectors.length > 0} light={isLightSurface} />
              <ToolChip icon={<AutoAwesome />} label="Improve" onClick={improvePrompt} light={isLightSurface} />
              <ToolChip icon={<Article />} label="Brief" onClick={generateBrief} light={isLightSurface} />
              <ToolChip icon={<Tune />} label="Options" onClick={() => setAdvancedOpen(true)} active={Object.values(advanced).some(Boolean)} light={isLightSurface} />
              <ToolChip icon={<History />} label="History" onClick={() => setHistoryOpen(true)} active={history.length > 0} light={isLightSurface} />
              <ToolChip icon={<FactCheck />} label="Review" onClick={() => setReviewOpen(true)} active={promptStrength >= 70} light={isLightSurface} />
            </>
          )}
        </Stack>

        <Stack direction="row" spacing={0.75} alignItems="center" justifyContent={{ xs: 'space-between', sm: 'flex-end' }}>
          {!isPreview && (
            <Stack direction="row" sx={{
	              p: 0.35,
	              border: `1px solid ${isLightSurface ? 'rgba(17,17,17,0.14)' : tokens.color.border.strong}`,
	              borderRadius: '8px',
	              bgcolor: isLightSurface ? 'rgba(17,17,17,0.05)' : tokens.color.bg.inset,
	            }}>
              {(['build', 'plan'] as const).map((item) => (
                <Button
                  key={item}
                  size="small"
                  onClick={() => setMode(item)}
                  sx={{
	                    minWidth: 56,
	                    px: 1.3,
	                    py: 0.5,
	                    borderRadius: '6px',
	                    color: mode === item ? tokens.color.text.inverse : isLightSurface ? 'rgba(17,17,17,0.64)' : tokens.color.text.secondary,
                    bgcolor: mode === item ? tokens.color.accent.lime : 'transparent',
                    '&:hover': { bgcolor: mode === item ? tokens.color.accent.lime : isLightSurface ? 'rgba(17,17,17,0.09)' : tokens.color.bg.surfaceHover },
                  }}
                >
                  {item === 'build' ? 'Build' : 'Plan'}
                </Button>
              ))}
            </Stack>
          )}
          <Button
            variant="contained"
            disabled={!prompt.trim() || busy}
            onClick={() => { if (!isPreview) submit(); }}
            endIcon={<ArrowUpward />}
	            sx={{
	              minWidth: isHero ? 112 : 92,
	              borderRadius: '8px',
	              py: 1,
              '&.Mui-disabled': {
                bgcolor: isLightSurface ? 'rgba(17,17,17,0.08)' : 'rgba(244,240,232,0.1)',
                color: isLightSurface ? 'rgba(17,17,17,0.32)' : 'rgba(244,240,232,0.32)',
              },
              ...(isPreview ? {
                bgcolor: tokens.color.text.inverse,
                color: tokens.color.text.primary,
                '&:hover': { bgcolor: tokens.color.text.inverse },
              } : {}),
            }}
          >
            {busy ? 'Working...' : cta}
          </Button>
        </Stack>
      </Stack>

      {showFullTools && (
        <Box sx={{ px: 1.5, pb: 1.25 }}>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ xs: 'stretch', sm: 'center' }}>
            <Typography variant="caption" sx={{ minWidth: 118, fontWeight: 800, color: isLightSurface ? 'rgba(17,17,17,0.66)' : tokens.color.text.secondary }}>
              Prompt strength {promptStrength}%
            </Typography>
	            <Box sx={{ flex: 1, height: 6, borderRadius: '999px', bgcolor: isLightSurface ? 'rgba(17,17,17,0.1)' : tokens.color.bg.inset, overflow: 'hidden' }}>
              <Box sx={{
                width: `${promptStrength}%`,
                height: '100%',
                bgcolor: promptStrength > 68 ? tokens.color.accent.lime : promptStrength > 35 ? tokens.color.accent.warning : tokens.color.accent.coral,
                transition: `width ${tokens.motion.base} ${tokens.motion.curve}`,
              }} />
            </Box>
            <Button size="small" variant="outlined" onClick={() => setReviewOpen(true)} disabled={!prompt.trim()}>
              Review
            </Button>
          </Stack>
        </Box>
      )}

      {!isPreview && slashMatches.length > 0 && (
        <Box sx={{ px: 1.5, pb: 1.25 }}>
          <Stack spacing={0.75} sx={{
	            p: 1,
	            border: `1px solid ${tokens.color.border.subtle}`,
	            borderRadius: '8px',
	            bgcolor: tokens.color.bg.surfaceRaised,
          }}>
            {slashMatches.map((recipe) => (
              <Button
                key={recipe.label}
                onClick={() => setPrompt(recipe.value)}
                sx={{ justifyContent: 'flex-start', color: tokens.color.text.primary }}
              >
                /{recipe.label.toLowerCase().replace(/\s+/g, '-')}
              </Button>
            ))}
          </Stack>
        </Box>
      )}

      {!isPreview && (attachments.length > 0 || designPreset || connectors.length > 0) && (
        <Stack direction="row" spacing={0.75} useFlexGap flexWrap="wrap" sx={{ px: 1.5, pb: 1.25 }}>
          {attachments.map((name) => (
            <Chip
              key={name}
              icon={<InsertDriveFile />}
              label={name}
              onDelete={() => setAttachments((current) => current.filter((item) => item !== name))}
              size="small"
              sx={metaChipSx}
            />
          ))}
          {designPreset && (
            <Chip
              icon={<Tune />}
              label={designPreset}
              onDelete={() => setDesignPreset('')}
              size="small"
              sx={metaChipSx}
            />
          )}
          {connectors.map((connector) => (
            <Chip
              key={connector}
              icon={<Hub />}
              label={connector}
              onDelete={() => toggleConnector(connector)}
              size="small"
              sx={metaChipSx}
            />
          ))}
        </Stack>
      )}

      {error && (
        <Typography variant="caption" color="error" sx={{ display: 'block', px: 2, pb: 1.5 }}>
          {error}
        </Typography>
      )}

      <Menu
        anchorEl={menuAnchor}
        open={Boolean(menuAnchor)}
        onClose={() => setMenuAnchor(null)}
        PaperProps={{
          sx: {
	            mt: 1,
	            bgcolor: tokens.color.bg.surfaceRaised,
	            color: tokens.color.text.primary,
	            border: `1px solid ${tokens.color.border.subtle}`,
	            borderRadius: '8px',
	          },
        }}
      >
        <MenuItem onClick={() => { fileInputRef.current?.click(); setMenuAnchor(null); }}>
          <AttachFile fontSize="small" style={{ marginRight: 10 }} /> Attach files
        </MenuItem>
        <MenuItem onClick={() => { setDesignOpen(true); setMenuAnchor(null); }}>
          <DesignServices fontSize="small" style={{ marginRight: 10 }} /> Choose design direction
        </MenuItem>
        <MenuItem onClick={() => { setConnectorsOpen(true); setMenuAnchor(null); }}>
          <Hub fontSize="small" style={{ marginRight: 10 }} /> Pick connectors
        </MenuItem>
        <MenuItem onClick={() => { improvePrompt(); setMenuAnchor(null); }}>
          <AutoAwesome fontSize="small" style={{ marginRight: 10 }} /> Improve prompt
        </MenuItem>
        <MenuItem onClick={() => { generateBrief(); setMenuAnchor(null); }}>
          <Article fontSize="small" style={{ marginRight: 10 }} /> Generate brief
        </MenuItem>
        <MenuItem onClick={() => { saveDraft(); setMenuAnchor(null); }}>
          <Save fontSize="small" style={{ marginRight: 10 }} /> Save draft
        </MenuItem>
        <MenuItem onClick={() => { setRecipesOpen(true); setMenuAnchor(null); }}>
          <AutoAwesome fontSize="small" style={{ marginRight: 10 }} /> Prompt recipes
        </MenuItem>
        <MenuItem onClick={() => { setHistoryOpen(true); setMenuAnchor(null); }}>
          <History fontSize="small" style={{ marginRight: 10 }} /> Prompt history
        </MenuItem>
      </Menu>

      <PromptDialog title="Design direction" open={designOpen} onClose={() => setDesignOpen(false)}>
        <Stack spacing={1.2}>
          {designPresets.map((preset) => (
            <Button
              key={preset}
              variant={designPreset === preset ? 'contained' : 'outlined'}
              onClick={() => { setDesignPreset(preset); setDesignOpen(false); }}
              startIcon={<Image />}
              sx={{ justifyContent: 'flex-start' }}
            >
              {preset}
            </Button>
          ))}
        </Stack>
      </PromptDialog>

      <PromptDialog title="Connectors" open={connectorsOpen} onClose={() => setConnectorsOpen(false)}>
        <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap">
          {connectorOptions.map((connector) => (
            <Chip
              key={connector}
              label={connector}
              icon={<Hub />}
              onClick={() => toggleConnector(connector)}
              sx={{
	                borderRadius: '8px',
	                bgcolor: connectors.includes(connector) ? tokens.color.accent.lime : tokens.color.bg.inset,
                color: connectors.includes(connector) ? tokens.color.text.inverse : tokens.color.text.primary,
                border: `1px solid ${connectors.includes(connector) ? tokens.color.accent.lime : tokens.color.border.strong}`,
              }}
            />
          ))}
        </Stack>
        <Button variant="contained" sx={{ mt: 2 }} onClick={() => setConnectorsOpen(false)}>Done</Button>
      </PromptDialog>

      <PromptDialog title="Prompt recipes" open={recipesOpen} onClose={() => setRecipesOpen(false)}>
        <Stack spacing={1.2}>
          {recipeOptions.map((recipe) => (
            <Button
              key={recipe.label}
              variant="outlined"
              onClick={() => { setPrompt(recipe.value); setRecipesOpen(false); }}
              startIcon={<AutoAwesome />}
              sx={{ justifyContent: 'flex-start', textAlign: 'left' }}
            >
              {recipe.label}
            </Button>
          ))}
        </Stack>
      </PromptDialog>

      <PromptDialog title="Advanced options" open={advancedOpen} onClose={() => setAdvancedOpen(false)}>
        <Stack spacing={2}>
          {Object.entries(advancedOptions).map(([group, options]) => (
            <Box key={group}>
              <Typography variant="overline" color="text.secondary">{group}</Typography>
              <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap" sx={{ mt: 0.75 }}>
                {options.map((option) => (
                  <Chip
                    key={option}
                    label={option}
                    onClick={() => setAdvanced((current) => ({
                      ...current,
                      [group]: current[group] === option ? '' : option,
                    }))}
                    sx={{
	                      borderRadius: '8px',
	                      bgcolor: advanced[group] === option ? tokens.color.accent.lime : tokens.color.bg.inset,
                      color: advanced[group] === option ? tokens.color.text.inverse : tokens.color.text.primary,
                      border: `1px solid ${advanced[group] === option ? tokens.color.accent.lime : tokens.color.border.strong}`,
                    }}
                  />
                ))}
              </Stack>
            </Box>
          ))}
          <Box>
            <Typography variant="overline" color="text.secondary">finisher gates</Typography>
            <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap" sx={{ mt: 0.75 }}>
              {qualityGates.map((gate) => (
                <Chip
                  key={gate}
                  label={gate}
                  onClick={() => toggleGate(gate)}
                  sx={{
	                    borderRadius: '8px',
	                    bgcolor: selectedGates.includes(gate) ? tokens.color.accent.lime : tokens.color.bg.inset,
                    color: selectedGates.includes(gate) ? tokens.color.text.inverse : tokens.color.text.primary,
                    border: `1px solid ${selectedGates.includes(gate) ? tokens.color.accent.lime : tokens.color.border.strong}`,
                  }}
                />
              ))}
            </Stack>
          </Box>
          <Button variant="contained" onClick={() => setAdvancedOpen(false)}>Done</Button>
        </Stack>
      </PromptDialog>

      <PromptDialog title="Review prompt" open={reviewOpen} onClose={() => setReviewOpen(false)}>
        <Stack spacing={2}>
          <Typography variant="body2" color="text.secondary">
            This is the complete instruction Ironflyer will use to create the project.
          </Typography>
          <TextField
            multiline
            minRows={10}
            value={composePrompt(prompt)}
            inputProps={{ readOnly: true }}
            sx={{
              '& .MuiInputBase-root': {
                alignItems: 'flex-start',
                fontFamily: tokens.font.mono,
                fontSize: 12,
              },
            }}
          />
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
            <Button variant="outlined" startIcon={<ContentCopy />} onClick={copyReview}>Copy</Button>
            <Button
              variant="contained"
              disabled={!prompt.trim() || busy}
              onClick={() => {
                setReviewOpen(false);
                submit();
              }}
            >
              {busy ? 'Working...' : cta}
            </Button>
          </Stack>
        </Stack>
      </PromptDialog>

      <PromptDialog title="Prompt history" open={historyOpen} onClose={() => setHistoryOpen(false)}>
        <Stack spacing={1}>
          {history.length === 0 && (
            <Typography variant="body2" color="text.secondary">No prompts yet.</Typography>
          )}
          {history.map((item) => (
            <Button
              key={item}
              variant="outlined"
              onClick={() => { setPrompt(item); setHistoryOpen(false); }}
              sx={{ justifyContent: 'flex-start', textAlign: 'left' }}
            >
              {item.split('\n')[0].slice(0, 92)}
            </Button>
          ))}
        </Stack>
      </PromptDialog>
    </Box>
  );
}

function ToolChip({
  icon, label, onClick, active = false, light = false,
}: {
  icon: React.ReactElement;
  label: string;
  onClick?: () => void;
  active?: boolean;
  light?: boolean;
}) {
  return (
    <Chip
      icon={icon}
      label={label}
      size="small"
      onClick={onClick}
      sx={{
	        borderRadius: '8px',
	        bgcolor: active ? 'rgba(229,255,0,0.34)' : light ? 'rgba(17,17,17,0.05)' : 'rgba(244,240,232,0.06)',
        color: active ? tokens.color.text.inverse : light ? 'rgba(17,17,17,0.66)' : tokens.color.text.secondary,
        border: `1px solid ${active ? 'rgba(229,255,0,0.58)' : light ? 'rgba(17,17,17,0.1)' : tokens.color.border.subtle}`,
        fontWeight: 800,
        cursor: onClick ? 'pointer' : 'default',
        transition: `background-color ${tokens.motion.base} ${tokens.motion.curve}, color ${tokens.motion.base} ${tokens.motion.curve}`,
      }}
    />
  );
}

function PromptDialog({
  title, open, onClose, children,
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle sx={{
        bgcolor: tokens.color.bg.surface,
        color: tokens.color.text.primary,
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}>
        {title}
        <IconButton onClick={onClose} sx={{ color: tokens.color.text.secondary }}>
          <Close fontSize="small" />
        </IconButton>
      </DialogTitle>
      <DialogContent sx={{ bgcolor: tokens.color.bg.surface, color: tokens.color.text.primary, pt: 1, pb: 3 }}>
        {children}
      </DialogContent>
    </Dialog>
  );
}

const metaChipSx = {
	  borderRadius: '8px',
  bgcolor: 'rgba(229,255,0,0.16)',
  color: tokens.color.text.primary,
  border: `1px solid rgba(229,255,0,0.34)`,
  '& .MuiChip-deleteIcon': { color: tokens.color.text.secondary },
};

function toolButtonSx(inverse: boolean) {
  return {
	    width: 34,
	    height: 34,
	    borderRadius: '8px',
	    border: `1px solid ${inverse ? 'rgba(10,10,10,0.16)' : tokens.color.border.strong}`,
    color: inverse ? tokens.color.text.inverse : tokens.color.text.primary,
    bgcolor: inverse ? 'rgba(10,10,10,0.04)' : tokens.color.bg.inset,
    '&:hover': {
      bgcolor: inverse ? 'rgba(10,10,10,0.1)' : tokens.color.bg.surfaceHover,
    },
  };
}
