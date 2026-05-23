'use client';

// First-run wizard at /welcome. Four steps from idea to a running finisher,
// targeting < 60 seconds end-to-end. Owns its own layout (no AppShell) so
// the user sees only the path forward. Reuses readImageAsBase64 from the
// ChatPane upload pattern and the existing api helpers (createProject,
// addVisualTarget, runFinisher).

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  ArrowBack, ArrowForward, AutoAwesome, Bolt, Check, Close,
  CloudUpload, Image as ImageIcon, RocketLaunch, Schedule,
} from '@mui/icons-material';
import {
  Box, Button, IconButton, LinearProgress, Stack, TextField, Typography,
} from '@mui/material';
import { IronflyerLogo } from '../../components/brand/IronflyerLogo';
import { api } from '../../lib/api';
import { tokens } from '../../lib/theme';
import { RequireAuth } from '../auth-context';

type StepIndex = 0 | 1 | 2 | 3;

interface StackChoice {
  id: 'auto' | 'next-supabase' | 'go-postgres' | 'custom';
  label: string;
  detail: string;
  promptHint: string;
  recommended?: boolean;
}

const STACK_CHOICES: StackChoice[] = [
  {
    id: 'auto',
    label: 'Auto-pick',
    detail: 'Let the Architect gate choose the fastest credible path to runtime, budget, and deploy.',
    promptHint: '',
    recommended: true,
  },
  {
    id: 'next-supabase',
    label: 'Next.js + Supabase',
    detail: 'App Router, MUI, Supabase auth, and Postgres for SaaS, portals, and dashboards.',
    promptHint: 'Stack: Next.js (App Router) with Supabase for auth and Postgres.',
  },
  {
    id: 'go-postgres',
    label: 'Go + Postgres',
    detail: 'chi, zerolog, and Postgres for APIs, services, and internal operating tools.',
    promptHint: 'Stack: Go (chi + zerolog) with Postgres for storage.',
  },
  {
    id: 'custom',
    label: 'Custom',
    detail: 'Name your runtime, database, deploy target, or constraints in the brief.',
    promptHint: '',
  },
];

const QUICK_STARTS: { label: string; prompt: string }[] = [
  {
    label: 'Web app',
    prompt: 'A production web app with auth, a primary user workflow, admin controls, tests, and a deploy-ready runtime.',
  },
  {
    label: 'Mobile app',
    prompt: 'A mobile-first PWA with account flows, offline caching, runtime checks, and a clean deploy path.',
  },
  {
    label: 'Game',
    prompt: 'A browser game with a playable loop, responsive controls, asset loading, build checks, and deploy instructions.',
  },
  {
    label: 'E-commerce',
    prompt: 'An e-commerce storefront with catalog, cart, checkout, order states, admin tools, and budget-aware APIs.',
  },
  {
    label: 'Dashboard',
    prompt: 'An operator dashboard with dense tables, saved views, exports, role access, tests, and runtime telemetry.',
  },
  {
    label: 'Internal tool',
    prompt: 'An internal tool with approvals, audit history, reports, SSO-ready auth, and deployment gates.',
  },
];

const PLACEHOLDER_EXAMPLES: string[] = [
  'A usage dashboard with budget alerts and deploy history',
  'A client portal with auth, documents, billing, and audit logs',
  'A support triage tool with approvals and runtime telemetry',
  'A deployable checkout flow with admin order controls',
  'A playable browser game with tests and hosted preview',
];

const MAX_REFERENCES = 6;
const MAX_REFERENCE_BYTES = 4 * 1024 * 1024; // matches orchestrator's 4 MiB cap per upload

interface ReferenceImage {
  id: string;
  name: string;
  mediaType: 'image/png' | 'image/jpeg' | 'image/webp';
  dataUrl: string; // for preview thumbnails
  base64: string;  // raw bytes (no data: prefix) for the wire
  bytes: number;
  width: number;
  height: number;
}

export default function WelcomePage() {
  return (
    <RequireAuth>
      <WelcomeWizard />
    </RequireAuth>
  );
}

function WelcomeWizard() {
  const router = useRouter();
  const [step, setStep] = useState<StepIndex>(0);
  const [idea, setIdea] = useState('');
  const [stack, setStack] = useState<StackChoice['id']>('auto');
  const [refs, setRefs] = useState<ReferenceImage[]>([]);
  const [placeholderIdx, setPlaceholderIdx] = useState(0);
  const [launching, setLaunching] = useState(false);
  const [launchError, setLaunchError] = useState<string | null>(null);
  const [launchStage, setLaunchStage] = useState<string>('');

  useEffect(() => {
    const t = window.setInterval(() => {
      setPlaceholderIdx((idx) => (idx + 1) % PLACEHOLDER_EXAMPLES.length);
    }, 3200);
    return () => window.clearInterval(t);
  }, []);

  // Carry over a draft prompt from the marketing PromptBox so the user
  // doesn't have to retype after sign-up.
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const pending = window.localStorage.getItem('ironflyer.pendingIdea');
    if (pending && !idea) {
      setIdea(pending);
      window.localStorage.removeItem('ironflyer.pendingIdea');
    }
  }, [idea]);

  const placeholder = `e.g. ${PLACEHOLDER_EXAMPLES[placeholderIdx]}`;
  const canAdvanceFromIdea = idea.trim().length >= 8;

  function next() {
    setStep((s) => (Math.min(3, s + 1) as StepIndex));
  }
  function back() {
    setStep((s) => (Math.max(0, s - 1) as StepIndex));
  }

  function applyQuickStart(prompt: string) {
    setIdea((current) => (current.trim() ? `${current.trim()}\n\n${prompt}` : prompt));
  }

  function projectNameFromIdea(raw: string): string {
    const cleaned = raw.replace(/\s+/g, ' ').trim();
    if (!cleaned) return 'Untitled project';
    const words = cleaned.split(' ').slice(0, 5).join(' ');
    return words.length > 60 ? words.slice(0, 60) : words;
  }

  function composeFinalIdea(): string {
    const parts: string[] = [idea.trim()];
    const choice = STACK_CHOICES.find((s) => s.id === stack);
    if (choice && choice.promptHint) parts.push(choice.promptHint);
    if (refs.length > 0) {
      parts.push(
        `Reference images attached: ${refs.length} (treat as pixel-perfect blocking gates for the UX gate).`,
      );
    }
    return parts.filter(Boolean).join('\n\n');
  }

  async function launch() {
    if (launching) return;
    if (!canAdvanceFromIdea) {
      setStep(0);
      return;
    }
    setLaunching(true);
    setLaunchError(null);
    try {
      setLaunchStage('Creating project...');
      const finalIdea = composeFinalIdea();
      const project = await api.createProject({
        name: projectNameFromIdea(idea),
        description: 'Created from the /welcome first-run wizard',
        idea: finalIdea,
      });

      if (refs.length > 0) {
        setLaunchStage(`Attaching ${refs.length} reference${refs.length === 1 ? '' : 's'}...`);
        // Sequential so we don't hammer the upload endpoint and so the user
        // sees them attach in order; each upload is small (<= 4 MiB).
        for (let i = 0; i < refs.length; i++) {
          const ref = refs[i];
          await api.addVisualTarget(project.id, {
            name: ref.name,
            viewportW: ref.width || 1280,
            viewportH: ref.height || 800,
            imagePngBase64: ref.base64,
            tolerance: 0.04,
          });
        }
      }

      setLaunchStage('Kicking off the finisher...');
      // Fire-and-forget: runFinisher is long-running but the orchestrator
      // returns when the loop hits its iteration cap. We don't await the
      // completion - the project workspace will subscribe to /stream and
      // show progress as it arrives.
      void api.runFinisher(project.id).catch(() => {
        // The project page surfaces run errors via its own SSE stream.
      });

      router.replace(`/projects/${project.id}?firstRun=1`);
    } catch (err) {
      setLaunching(false);
      setLaunchStage('');
      setLaunchError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <Box sx={{
      minHeight: '100vh',
      display: 'flex',
      flexDirection: 'column',
      bgcolor: tokens.color.bg.base,
      color: tokens.color.text.primary,
      backgroundImage: `linear-gradient(180deg, ${tokens.color.bg.surfaceRaised} 0%, ${tokens.color.bg.base} 44%)`,
    }}>
      <WizardHeader step={step} />

      <Box component="main" sx={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        px: { xs: 2, sm: 3, md: 6 },
        py: { xs: 2.4, md: 5 },
      }}>
        <Box sx={{ width: '100%', maxWidth: 960, mx: 'auto', flex: 1, display: 'flex', flexDirection: 'column' }}>
          {step === 0 && (
            <IdeaStep
              idea={idea}
              setIdea={setIdea}
              placeholder={placeholder}
              onPickQuickStart={applyQuickStart}
            />
          )}
          {step === 1 && (
            <StackStep stack={stack} setStack={setStack} />
          )}
          {step === 2 && (
            <ReferencesStep refs={refs} setRefs={setRefs} />
          )}
          {step === 3 && (
            <LaunchStep
              idea={idea}
              stack={stack}
              refs={refs}
              launching={launching}
              launchError={launchError}
              launchStage={launchStage}
              onLaunch={launch}
            />
          )}
        </Box>
      </Box>

      <WizardFooter
        step={step}
        canAdvanceFromIdea={canAdvanceFromIdea}
        onBack={back}
        onNext={next}
        onLaunch={launch}
        launching={launching}
        refsCount={refs.length}
      />
    </Box>
  );
}

function WizardHeader({ step }: { step: StepIndex }) {
  const stepCount = 4;
  return (
    <Box component="header" sx={{
      flex: '0 0 auto',
      borderBottom: `1px solid ${tokens.color.border.subtle}`,
      bgcolor: tokens.color.bg.overlay,
      backdropFilter: 'blur(14px)',
    }}>
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ px: { xs: 2, sm: 3, md: 6 }, py: 1.2, gap: 1.5 }}
      >
        <Link href="/" style={{ color: 'inherit', textDecoration: 'none' }}>
          <IronflyerLogo size={26} tone="dark" />
        </Link>
        <Stack direction="row" spacing={1.2} alignItems="center">
          <Typography variant="caption" sx={{ color: tokens.color.text.secondary, fontWeight: 800, letterSpacing: '0.08em', whiteSpace: 'nowrap' }}>
            STEP {step + 1} OF {stepCount}
          </Typography>
          <Stack direction="row" spacing={0.7}>
            {Array.from({ length: stepCount }).map((_, i) => (
              <Box key={i} sx={{
                width: i === step ? 28 : 10,
                height: 8,
                borderRadius: tokens.radius.sm,
                bgcolor: i <= step ? tokens.color.accent.lime : tokens.color.border.strong,
                transition: `width ${tokens.motion.fast} ${tokens.motion.curve}, background-color ${tokens.motion.fast} ${tokens.motion.curve}`,
              }} />
            ))}
          </Stack>
        </Stack>
      </Stack>
    </Box>
  );
}

function StepHeading({
  eyebrow, title, subtitle,
}: { eyebrow: string; title: string; subtitle: string }) {
  return (
    <Box sx={{ mb: { xs: 2.4, md: 3.2 } }}>
      <Typography variant="overline" sx={{ color: tokens.color.accent.sky, fontWeight: 900, letterSpacing: '0.14em' }}>
        {eyebrow}
      </Typography>
      <Typography
        component="h1"
        sx={{
          mt: 0.6,
          fontFamily: tokens.font.display,
          fontSize: { xs: '1.9rem', sm: '2.2rem', md: '2.8rem' },
          lineHeight: 1,
          textTransform: 'uppercase',
          textWrap: 'balance',
          letterSpacing: 0,
          color: tokens.color.text.primary,
        }}
      >
        {title}
      </Typography>
      <Typography variant="body1" sx={{ mt: 1.2, color: tokens.color.text.secondary, maxWidth: 680, fontWeight: 500, lineHeight: 1.55 }}>
        {subtitle}
      </Typography>
    </Box>
  );
}

function IdeaStep({
  idea, setIdea, placeholder, onPickQuickStart,
}: {
  idea: string;
  setIdea: (value: string) => void;
  placeholder: string;
  onPickQuickStart: (prompt: string) => void;
}) {
  return (
    <Box>
      <StepHeading
        eyebrow="Step 1"
        title="Define the finish line"
        subtitle="Describe the product, runtime, budget limits, deploy target, and the gates that must pass before it counts as done."
      />

      <TextField
        value={idea}
        onChange={(event) => setIdea(event.target.value)}
        placeholder={placeholder}
        multiline
        minRows={5}
        maxRows={10}
        fullWidth
        autoFocus
        sx={{
          '& .MuiOutlinedInput-root': {
            bgcolor: tokens.color.bg.inset,
            borderRadius: `${tokens.radius.sm}px`,
            border: `1px solid ${tokens.color.border.strong}`,
            color: tokens.color.text.primary,
            fontSize: { xs: 16, md: 18 },
            fontWeight: 600,
            lineHeight: 1.4,
            p: { xs: 1.5, md: 2 },
            transition: `border-color ${tokens.motion.fast} ${tokens.motion.curve}, box-shadow ${tokens.motion.fast} ${tokens.motion.curve}, background-color ${tokens.motion.fast} ${tokens.motion.curve}`,
            '& fieldset': { border: 'none' },
            '&:hover': {
              borderColor: tokens.color.border.accent,
              bgcolor: tokens.color.bg.surface,
            },
            '&.Mui-focused': {
              borderColor: tokens.color.accent.lime,
              boxShadow: '0 0 0 3px rgba(229,255,0,0.18)',
            },
          },
          '& .MuiInputBase-input::placeholder': {
            color: tokens.color.text.muted,
            opacity: 1,
          },
        }}
      />

      <Stack sx={{ mt: 3 }} spacing={1.2}>
        <Typography variant="caption" sx={{ color: tokens.color.text.secondary, fontWeight: 800, letterSpacing: '0.1em' }}>
          START FROM A BUILD SHAPE
        </Typography>
        <Box sx={{
          display: 'grid',
          gridTemplateColumns: { xs: 'repeat(2, minmax(0, 1fr))', sm: 'repeat(3, minmax(0, 1fr))', md: 'repeat(6, minmax(0, 1fr))' },
          gap: 1,
        }}>
          {QUICK_STARTS.map((qs) => (
            <Button
              key={qs.label}
              variant="outlined"
              onClick={() => onPickQuickStart(qs.prompt)}
              sx={quickStartButtonSx}
            >
              {qs.label}
            </Button>
          ))}
        </Box>
      </Stack>
    </Box>
  );
}

function StackStep({
  stack, setStack,
}: { stack: StackChoice['id']; setStack: (value: StackChoice['id']) => void }) {
  return (
    <Box>
      <StepHeading
        eyebrow="Step 2 (optional)"
        title="Choose the runtime"
        subtitle="Pin a stack now, or let the Architect gate decide from your brief before code, tests, budget, and deploy checks run."
      />

      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' },
        gap: 1.6,
      }}>
        {STACK_CHOICES.map((choice) => {
          const active = stack === choice.id;
          return (
            <Box
              key={choice.id}
              onClick={() => setStack(choice.id)}
              role="button"
              tabIndex={0}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  setStack(choice.id);
                }
              }}
              sx={{
                cursor: 'pointer',
                p: 2.2,
                borderRadius: `${tokens.radius.sm}px`,
                bgcolor: active ? 'rgba(229,255,0,0.1)' : tokens.color.bg.surface,
                border: active
                  ? `2px solid ${tokens.color.accent.lime}`
                  : `2px solid ${tokens.color.border.subtle}`,
                color: tokens.color.text.primary,
                transition: `border-color ${tokens.motion.fast} ${tokens.motion.curve}, background-color ${tokens.motion.fast} ${tokens.motion.curve}, transform ${tokens.motion.fast} ${tokens.motion.curve}`,
                '&:hover': {
                  borderColor: active ? tokens.color.accent.lime : tokens.color.border.strong,
                  bgcolor: active ? 'rgba(229,255,0,0.14)' : tokens.color.bg.surfaceHover,
                  transform: 'translateY(-1px)',
                },
              }}
            >
              <Stack direction="row" spacing={1} alignItems="center" justifyContent="space-between">
                <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>
                  {choice.label}
                </Typography>
                {choice.recommended && (
                  <Box sx={{
                    px: 0.9, py: 0.2,
                    borderRadius: `${tokens.radius.sm}px`,
                    bgcolor: 'rgba(120,219,255,0.16)',
                    color: tokens.color.accent.sky,
                    border: '1px solid rgba(120,219,255,0.34)',
                    fontSize: 10,
                    fontWeight: 900,
                    letterSpacing: '0.1em',
                  }}>
                    RECOMMENDED
                  </Box>
                )}
              </Stack>
              <Typography variant="body2" sx={{ mt: 0.8, color: tokens.color.text.secondary, lineHeight: 1.5 }}>
                {choice.detail}
              </Typography>
              {active && (
                <Stack direction="row" spacing={0.6} alignItems="center" sx={{ mt: 1.4 }}>
                  <Check sx={{ fontSize: 16, color: tokens.color.accent.lime }} />
                  <Typography variant="caption" sx={{ color: tokens.color.accent.lime, fontWeight: 800 }}>
                    Selected
                  </Typography>
                </Stack>
              )}
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}

function ReferencesStep({
  refs, setRefs,
}: {
  refs: ReferenceImage[];
  setRefs: (next: ReferenceImage[]) => void;
}) {
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  async function addFiles(files: FileList | File[] | null) {
    if (!files) return;
    setError(null);
    const accepted: ReferenceImage[] = [...refs];
    for (const file of Array.from(files)) {
      if (accepted.length >= MAX_REFERENCES) {
        setError(`At most ${MAX_REFERENCES} references in the first run. Add more later from the project.`);
        break;
      }
      const mt = file.type.toLowerCase();
      if (mt !== 'image/png' && mt !== 'image/jpeg' && mt !== 'image/webp') {
        setError('References must be PNG, JPEG, or WebP images.');
        continue;
      }
      if (file.size > MAX_REFERENCE_BYTES) {
        setError(`"${file.name}" is over 4 MB - pick a smaller export.`);
        continue;
      }
      try {
        const { dataUrl, base64, width, height } = await readImage(file);
        accepted.push({
          id: crypto.randomUUID(),
          name: file.name || 'reference.png',
          mediaType: mt as ReferenceImage['mediaType'],
          dataUrl,
          base64,
          bytes: file.size,
          width,
          height,
        });
      } catch (err) {
        setError(`Could not read ${file.name}: ${(err as Error)?.message ?? err}`);
      }
    }
    setRefs(accepted);
    if (inputRef.current) inputRef.current.value = '';
  }

  function removeRef(id: string) {
    setRefs(refs.filter((r) => r.id !== id));
  }

  return (
    <Box>
      <StepHeading
        eyebrow="Step 3 (optional)"
        title="Add visual gates"
        subtitle="Attach screenshots or mockups when the UX must match. They become blocking checks against the live runtime preview."
      />

      <Box
        onDragOver={(event) => { event.preventDefault(); setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDrop={(event) => {
          event.preventDefault();
          setDragging(false);
          void addFiles(event.dataTransfer.files);
        }}
        onClick={() => inputRef.current?.click()}
        role="button"
        tabIndex={0}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            inputRef.current?.click();
          }
        }}
        sx={{
          cursor: 'pointer',
          borderRadius: `${tokens.radius.sm}px`,
          border: dragging
            ? `2px dashed ${tokens.color.accent.lime}`
            : `2px dashed ${tokens.color.border.strong}`,
          bgcolor: dragging ? 'rgba(229,255,0,0.1)' : tokens.color.bg.surface,
          p: { xs: 3, md: 4.4 },
          textAlign: 'center',
          transition: `background-color ${tokens.motion.fast} ${tokens.motion.curve}, border-color ${tokens.motion.fast} ${tokens.motion.curve}`,
          '&:hover': { borderColor: tokens.color.border.accent, bgcolor: tokens.color.bg.surfaceHover },
        }}
      >
        <CloudUpload sx={{ fontSize: 36, color: tokens.color.accent.sky }} />
        <Typography variant="subtitle1" sx={{ mt: 1.2, fontWeight: 900 }}>
          Drop visual targets, or click to pick files
        </Typography>
        <Typography variant="body2" sx={{ mt: 0.6, color: tokens.color.text.secondary }}>
          PNG, JPEG, or WebP. Up to {MAX_REFERENCES} files, 4 MB each. You can skip this gate.
        </Typography>
        <input
          ref={inputRef}
          type="file"
          accept="image/png,image/jpeg,image/webp"
          multiple
          hidden
          onChange={(event) => void addFiles(event.target.files)}
        />
      </Box>

      {error && (
        <Typography variant="body2" sx={{ mt: 1.6, color: tokens.color.accent.danger, fontWeight: 700 }}>
          {error}
        </Typography>
      )}

      {refs.length > 0 && (
        <Box sx={{
          mt: 2.4,
          display: 'grid',
          gridTemplateColumns: { xs: 'repeat(2, 1fr)', sm: 'repeat(3, 1fr)', md: 'repeat(4, 1fr)' },
          gap: 1.2,
        }}>
          {refs.map((ref) => (
            <Box key={ref.id} sx={{
              position: 'relative',
              borderRadius: `${tokens.radius.sm}px`,
              overflow: 'hidden',
              border: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: tokens.color.bg.surface,
            }}>
              <Box
                component="img"
                src={ref.dataUrl}
                alt={ref.name}
                sx={{ width: '100%', aspectRatio: '4 / 3', objectFit: 'cover', display: 'block' }}
              />
              <Box sx={{ px: 1, py: 0.7 }}>
                <Typography variant="caption" sx={{ display: 'block', fontWeight: 700 }} noWrap>
                  {ref.name}
                </Typography>
                <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
                  {ref.width}x{ref.height} · {(ref.bytes / 1024).toFixed(0)} KB
                </Typography>
              </Box>
              <IconButton
                size="small"
                aria-label={`Remove ${ref.name}`}
                onClick={(event) => { event.stopPropagation(); removeRef(ref.id); }}
                sx={{
                  position: 'absolute',
                  top: 6,
                  right: 6,
                  width: 26,
                  height: 26,
                  bgcolor: tokens.color.bg.overlay,
                  color: tokens.color.text.primary,
                  '&:hover': { bgcolor: tokens.color.bg.inset },
                }}
              >
                <Close sx={{ fontSize: 14 }} />
              </IconButton>
            </Box>
          ))}
        </Box>
      )}
    </Box>
  );
}

function LaunchStep({
  idea, stack, refs, launching, launchError, launchStage, onLaunch,
}: {
  idea: string;
  stack: StackChoice['id'];
  refs: ReferenceImage[];
  launching: boolean;
  launchError: string | null;
  launchStage: string;
  onLaunch: () => void;
}) {
  const stackChoice = STACK_CHOICES.find((s) => s.id === stack)!;
  return (
    <Box>
      <StepHeading
        eyebrow="Step 4"
        title="Start the run"
        subtitle="Review the build brief. Ironflyer will create the project, attach visual gates, start the finisher loop, and route you into the workspace."
      />

      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: '1.4fr 1fr' },
        gap: 2,
      }}>
        <Box sx={launchCardSx}>
          <PreviewLabel icon={<AutoAwesome sx={{ fontSize: 14 }} />} label="Prompt" />
          <Box sx={{
            mt: 1.2,
            p: 1.6,
            borderRadius: `${tokens.radius.sm}px`,
            bgcolor: tokens.color.bg.inset,
            border: `1px solid ${tokens.color.border.subtle}`,
            maxHeight: 240,
            overflowY: 'auto',
            whiteSpace: 'pre-wrap',
            fontSize: 14,
            lineHeight: 1.5,
            color: tokens.color.text.primary,
          }}>
            {idea.trim() || 'No prompt provided yet - go back to step 1.'}
          </Box>
        </Box>

        <Stack spacing={2}>
          <Box sx={launchCardSx}>
            <PreviewLabel icon={<Bolt sx={{ fontSize: 14 }} />} label="Stack" />
            <Typography variant="subtitle2" sx={{ mt: 1, fontWeight: 900 }}>{stackChoice.label}</Typography>
            <Typography variant="caption" sx={{ color: tokens.color.text.secondary, lineHeight: 1.5 }}>{stackChoice.detail}</Typography>
          </Box>

          <Box sx={launchCardSx}>
            <PreviewLabel icon={<ImageIcon sx={{ fontSize: 14 }} />} label="References" />
            {refs.length === 0 ? (
              <Typography variant="body2" sx={{ mt: 1, color: tokens.color.text.secondary }}>
                None. The UX gate will validate the generated runtime defaults.
              </Typography>
            ) : (
              <Stack direction="row" spacing={0.8} sx={{ mt: 1.2, flexWrap: 'wrap' }} useFlexGap>
                {refs.slice(0, 6).map((ref) => (
                  <Box
                    key={ref.id}
                    component="img"
                    src={ref.dataUrl}
                    alt={ref.name}
                    sx={{
                      width: 56,
                      height: 56,
                      borderRadius: `${tokens.radius.sm}px`,
                      objectFit: 'cover',
                      border: `1px solid ${tokens.color.border.subtle}`,
                    }}
                  />
                ))}
              </Stack>
            )}
          </Box>
        </Stack>
      </Box>

      <Box sx={{
        mt: 3,
        p: 2.4,
        borderRadius: `${tokens.radius.sm}px`,
        bgcolor: tokens.color.bg.surface,
        color: tokens.color.text.primary,
        border: `1px solid ${tokens.color.border.accent}`,
        display: 'flex',
        flexDirection: { xs: 'column', sm: 'row' },
        alignItems: { xs: 'stretch', sm: 'center' },
        justifyContent: 'space-between',
        gap: 2,
      }}>
        <Box>
          <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>
            Gates start as soon as the project opens
          </Typography>
          <Typography variant="body2" sx={{ mt: 0.4, color: tokens.color.text.secondary }}>
            Spec, UX, code, tests, security, budget, and deploy checks will stream in the workspace.
          </Typography>
        </Box>
        <Stack direction="row" spacing={1} alignItems="center" sx={{ flexShrink: 0 }}>
          <Schedule sx={{ fontSize: 18, color: tokens.color.accent.lime }} />
          <Typography variant="body2" sx={{ fontWeight: 900, color: tokens.color.accent.lime }}>
            First preview: ~60s
          </Typography>
        </Stack>
      </Box>

      {launching && (
        <Box sx={{ mt: 2 }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <RocketLaunch sx={{ fontSize: 18, color: tokens.color.accent.lime }} />
            <Typography variant="body2" sx={{ fontWeight: 800 }}>{launchStage}</Typography>
          </Stack>
          <LinearProgress sx={{
            mt: 1,
            height: 6,
            borderRadius: tokens.radius.sm,
            bgcolor: tokens.color.bg.surfaceRaised,
            '& .MuiLinearProgress-bar': { bgcolor: tokens.color.accent.lime },
          }} />
        </Box>
      )}

      {launchError && (
        <Box sx={{
          mt: 2,
          p: 1.6,
          borderRadius: `${tokens.radius.sm}px`,
          border: '1px solid rgba(255,24,24,0.4)',
          bgcolor: 'rgba(255,24,24,0.08)',
        }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 900, color: tokens.color.accent.danger }}>
            Could not start the run
          </Typography>
          <Typography variant="body2" sx={{ mt: 0.4, color: tokens.color.text.secondary }}>{launchError}</Typography>
          <Button
            variant="outlined"
            size="small"
            onClick={onLaunch}
            sx={{ mt: 1.2, borderRadius: `${tokens.radius.sm}px`, borderColor: 'rgba(255,24,24,0.5)', color: tokens.color.accent.danger }}
          >
            Try again
          </Button>
        </Box>
      )}
    </Box>
  );
}

function WizardFooter({
  step, canAdvanceFromIdea, onBack, onNext, onLaunch, launching, refsCount,
}: {
  step: StepIndex;
  canAdvanceFromIdea: boolean;
  onBack: () => void;
  onNext: () => void;
  onLaunch: () => void;
  launching: boolean;
  refsCount: number;
}) {
  const isLast = step === 3;
  const isOptional = step === 1 || step === 2;

  let nextLabel = 'Continue';
  if (step === 2) nextLabel = refsCount > 0 ? 'Continue' : 'Skip';
  if (step === 1) nextLabel = 'Continue';

  const nextDisabled = step === 0 && !canAdvanceFromIdea;

  return (
    <Box component="footer" sx={{
      flex: '0 0 auto',
      borderTop: `1px solid ${tokens.color.border.subtle}`,
      bgcolor: tokens.color.bg.overlay,
      backdropFilter: 'blur(14px)',
      position: 'sticky',
      bottom: 0,
    }}>
      <Stack
        direction="row"
        justifyContent="space-between"
        alignItems="center"
        sx={{ px: { xs: 2, sm: 3, md: 6 }, py: 1.4, gap: 1.2 }}
      >
        <Button
          variant="text"
          startIcon={<ArrowBack />}
          disabled={step === 0 || launching}
          onClick={onBack}
          sx={{
            color: tokens.color.text.secondary,
            borderRadius: `${tokens.radius.sm}px`,
            fontWeight: 800,
            visibility: step === 0 ? 'hidden' : 'visible',
            minWidth: { xs: 44, sm: 96 },
            px: { xs: 1, sm: 2 },
            '&:hover': { color: tokens.color.text.primary, bgcolor: tokens.color.bg.surfaceHover },
          }}
        >
          Back
        </Button>

        {isOptional && (
          <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontWeight: 700, display: { xs: 'none', sm: 'block' } }}>
            Optional gate
          </Typography>
        )}

        {isLast ? (
          <Button
            variant="contained"
            disabled={launching || !canAdvanceFromIdea}
            onClick={onLaunch}
            startIcon={<RocketLaunch />}
            sx={launchButtonSx}
          >
            {launching ? 'Starting...' : 'Start run'}
          </Button>
        ) : (
          <Button
            variant="contained"
            disabled={nextDisabled}
            onClick={onNext}
            endIcon={<ArrowForward />}
            sx={primaryButtonSx}
          >
            {nextLabel}
          </Button>
        )}
      </Stack>
    </Box>
  );
}

function PreviewLabel({ icon, label }: { icon: React.ReactNode; label: string }) {
  return (
    <Stack direction="row" spacing={0.7} alignItems="center" sx={{ color: tokens.color.accent.sky }}>
      {icon}
      <Typography variant="overline" sx={{ color: 'inherit', fontWeight: 900, letterSpacing: '0.12em' }}>
        {label}
      </Typography>
    </Stack>
  );
}

// readImage decodes a File into a base64 payload (no data: prefix), the
// preview data URL, and the intrinsic pixel size. Width/height are used as
// the visual target's viewport hint so the UX gate diffs at the right
// resolution.
function readImage(
  file: File,
): Promise<{ dataUrl: string; base64: string; width: number; height: number }> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error('FileReader error'));
    reader.onload = () => {
      const dataUrl = String(reader.result || '');
      const commaIdx = dataUrl.indexOf(',');
      const base64 = commaIdx >= 0 ? dataUrl.slice(commaIdx + 1) : dataUrl;
      const img = new window.Image();
      img.onload = () => {
        resolve({
          dataUrl,
          base64,
          width: img.naturalWidth || 1280,
          height: img.naturalHeight || 800,
        });
      };
      img.onerror = () => resolve({ dataUrl, base64, width: 1280, height: 800 });
      img.src = dataUrl;
    };
    reader.readAsDataURL(file);
  });
}

const quickStartButtonSx = {
  bgcolor: tokens.color.bg.surface,
  color: tokens.color.text.primary,
  borderColor: tokens.color.border.subtle,
  borderRadius: `${tokens.radius.sm}px`,
  fontWeight: 800,
  minHeight: 44,
  py: 1.1,
  px: 1,
  whiteSpace: 'normal',
  lineHeight: 1.15,
  transition: `border-color ${tokens.motion.fast} ${tokens.motion.curve}, background-color ${tokens.motion.fast} ${tokens.motion.curve}, color ${tokens.motion.fast} ${tokens.motion.curve}`,
  '&:hover': {
    borderColor: tokens.color.border.accent,
    bgcolor: 'rgba(229,255,0,0.1)',
    color: tokens.color.accent.lime,
  },
};

const launchCardSx = {
  p: 2,
  borderRadius: `${tokens.radius.sm}px`,
  bgcolor: tokens.color.bg.surface,
  border: `1px solid ${tokens.color.border.subtle}`,
  color: tokens.color.text.primary,
};

const primaryButtonSx = {
  bgcolor: tokens.color.accent.lime,
  color: tokens.color.text.inverse,
  borderRadius: `${tokens.radius.sm}px`,
  fontWeight: 900,
  px: 2.6,
  py: 1.1,
  minHeight: 44,
  boxShadow: 'none',
  '&:hover': { bgcolor: '#f0ff36', boxShadow: 'none' },
  '&.Mui-disabled': {
    bgcolor: tokens.color.bg.surfaceRaised,
    color: tokens.color.text.muted,
  },
};

const launchButtonSx = {
  ...primaryButtonSx,
  px: 3,
  fontSize: '1rem',
};
