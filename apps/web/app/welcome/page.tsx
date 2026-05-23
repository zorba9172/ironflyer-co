'use client';

// First-run wizard at /welcome. Four steps from idea to a running finisher,
// targeting < 60 seconds end-to-end. Owns its own layout (no AppShell) so
// the user sees only the path forward. Reuses readImageAsBase64 from the
// ChatPane upload pattern and the existing api helpers (createProject,
// addVisualTarget, runFinisher).

import { useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  ArrowBack, ArrowForward, AutoAwesome, Bolt, Check, Close,
  CloudUpload, Folder, Image as ImageIcon, RocketLaunch, Schedule,
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
    detail: 'Ironflyer chooses a stack that fits the idea. Recommended for the first run.',
    promptHint: '',
    recommended: true,
  },
  {
    id: 'next-supabase',
    label: 'Next.js + Supabase',
    detail: 'App Router, MUI, Supabase auth and Postgres. Good for SaaS and dashboards.',
    promptHint: 'Stack: Next.js (App Router) with Supabase for auth and Postgres.',
  },
  {
    id: 'go-postgres',
    label: 'Go + Postgres',
    detail: 'chi router, zerolog, sqlc-style Postgres. Good for backends and internal tools.',
    promptHint: 'Stack: Go (chi + zerolog) with Postgres for storage.',
  },
  {
    id: 'custom',
    label: 'Custom',
    detail: 'Describe the stack in the prompt. The Architect gate will pin it down.',
    promptHint: '',
  },
];

const QUICK_STARTS: { label: string; prompt: string }[] = [
  {
    label: 'Web app',
    prompt: 'A web app with auth, a primary user workflow, an admin dashboard, and email notifications.',
  },
  {
    label: 'Mobile app',
    prompt: 'A mobile-first PWA with offline caching, push notifications, and a clean account flow.',
  },
  {
    label: 'Game',
    prompt: 'A browser game built with Phaser where players defend a castle through escalating waves.',
  },
  {
    label: 'E-commerce',
    prompt: 'An e-commerce storefront with catalog, cart, Stripe checkout, order states, and an admin console.',
  },
  {
    label: 'Dashboard',
    prompt: 'An operator dashboard with KPI cards, dense tables, saved views, exports, and role-based access.',
  },
  {
    label: 'Internal tool',
    prompt: 'An internal operations tool with approvals, audit history, reports, and SSO-ready auth.',
  },
];

const PLACEHOLDER_EXAMPLES: string[] = [
  'An e-commerce store for plant lovers',
  'A SaaS dashboard for a fitness studio',
  'A Phaser game where players defend a castle',
  'A client portal for an accounting practice',
  'An internal tool that triages incoming support tickets',
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
      // completion — the project workspace will subscribe to /stream and
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
      bgcolor: tokens.color.bg.alabaster,
      color: tokens.color.text.inverse,
      backgroundImage: 'linear-gradient(180deg, rgba(229,255,0,0.14), rgba(244,240,232,0) 320px)',
    }}>
      <WizardHeader step={step} />

      <Box component="main" sx={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        px: { xs: 2.4, md: 6 },
        py: { xs: 3, md: 5 },
      }}>
        <Box sx={{ width: '100%', maxWidth: 920, mx: 'auto', flex: 1, display: 'flex', flexDirection: 'column' }}>
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
      borderBottom: '1px solid rgba(17,17,17,0.08)',
      bgcolor: 'rgba(244,240,232,0.92)',
      backdropFilter: 'blur(14px)',
    }}>
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ px: { xs: 2.4, md: 6 }, py: 1.4 }}
      >
        <Link href="/" style={{ color: 'inherit', textDecoration: 'none' }}>
          <IronflyerLogo size={26} tone="light" />
        </Link>
        <Stack direction="row" spacing={1.2} alignItems="center">
          <Typography variant="caption" sx={{ color: '#5f5a52', fontWeight: 800, letterSpacing: '0.08em' }}>
            STEP {step + 1} OF {stepCount}
          </Typography>
          <Stack direction="row" spacing={0.7}>
            {Array.from({ length: stepCount }).map((_, i) => (
              <Box key={i} sx={{
                width: i === step ? 28 : 10,
                height: 8,
                borderRadius: '999px',
                bgcolor: i <= step ? tokens.color.accent.lime : 'rgba(17,17,17,0.14)',
                transition: 'width 220ms ease, background-color 220ms ease',
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
      <Typography variant="overline" sx={{ color: '#9fb500', fontWeight: 900, letterSpacing: '0.14em' }}>
        {eyebrow}
      </Typography>
      <Typography
        component="h1"
        sx={{
          mt: 0.6,
          fontFamily: tokens.font.display,
          fontSize: { xs: '1.85rem', md: '2.6rem' },
          lineHeight: 1,
          textTransform: 'uppercase',
          textWrap: 'balance',
        }}
      >
        {title}
      </Typography>
      <Typography variant="body1" sx={{ mt: 1.2, color: '#5f5a52', maxWidth: 640, fontWeight: 500 }}>
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
        title="What are you building?"
        subtitle="Describe the product in plain English. The finisher will plan, build, gate, and prepare it for deployment."
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
            bgcolor: '#fffcf3',
            borderRadius: '12px',
            border: '1px solid rgba(17,17,17,0.14)',
            color: tokens.color.text.inverse,
            fontSize: { xs: 16, md: 18 },
            fontWeight: 600,
            lineHeight: 1.4,
            p: { xs: 1.8, md: 2.4 },
            '& fieldset': { border: 'none' },
            '&.Mui-focused': {
              borderColor: tokens.color.accent.lime,
              boxShadow: '0 0 0 3px rgba(229,255,0,0.22)',
            },
          },
          '& .MuiInputBase-input::placeholder': {
            color: '#86807a',
            opacity: 1,
          },
        }}
      />

      <Stack sx={{ mt: 3 }} spacing={1.2}>
        <Typography variant="caption" sx={{ color: '#5f5a52', fontWeight: 800, letterSpacing: '0.1em' }}>
          OR PICK A QUICK-START
        </Typography>
        <Box sx={{
          display: 'grid',
          gridTemplateColumns: { xs: 'repeat(2, 1fr)', sm: 'repeat(3, 1fr)', md: 'repeat(6, 1fr)' },
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
        title="Pick a stack"
        subtitle="The Architect gate will commit to a stack either way. Pin one if you already know what you want."
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
                borderRadius: '12px',
                bgcolor: active ? 'rgba(229,255,0,0.18)' : '#fffcf3',
                border: active
                  ? `2px solid ${tokens.color.accent.lime}`
                  : '2px solid rgba(17,17,17,0.1)',
                transition: 'border-color 160ms, background-color 160ms, transform 160ms',
                '&:hover': {
                  borderColor: active ? tokens.color.accent.lime : 'rgba(17,17,17,0.32)',
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
                    borderRadius: '6px',
                    bgcolor: tokens.color.accent.lime,
                    color: tokens.color.text.inverse,
                    fontSize: 10,
                    fontWeight: 900,
                    letterSpacing: '0.1em',
                  }}>
                    RECOMMENDED
                  </Box>
                )}
              </Stack>
              <Typography variant="body2" sx={{ mt: 0.8, color: '#5f5a52', lineHeight: 1.5 }}>
                {choice.detail}
              </Typography>
              {active && (
                <Stack direction="row" spacing={0.6} alignItems="center" sx={{ mt: 1.4 }}>
                  <Check sx={{ fontSize: 16, color: '#6f7e00' }} />
                  <Typography variant="caption" sx={{ color: '#6f7e00', fontWeight: 800 }}>
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
        setError(`"${file.name}" is over 4 MB — pick a smaller export.`);
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
        title="Reference images"
        subtitle="Figma exports, screenshots, and mockups become pixel-perfect blocking gates. The UX gate diffs the live preview against each one."
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
          borderRadius: '12px',
          border: dragging
            ? `2px dashed ${tokens.color.accent.lime}`
            : '2px dashed rgba(17,17,17,0.2)',
          bgcolor: dragging ? 'rgba(229,255,0,0.14)' : '#fffcf3',
          p: { xs: 3, md: 4.4 },
          textAlign: 'center',
          transition: 'background-color 160ms, border-color 160ms',
          '&:hover': { borderColor: 'rgba(17,17,17,0.36)' },
        }}
      >
        <CloudUpload sx={{ fontSize: 36, color: '#9fb500' }} />
        <Typography variant="subtitle1" sx={{ mt: 1.2, fontWeight: 900 }}>
          Drop reference images here, or click to pick files
        </Typography>
        <Typography variant="body2" sx={{ mt: 0.6, color: '#5f5a52' }}>
          PNG, JPEG, or WebP. Up to {MAX_REFERENCES} files, 4 MB each. Skip-able.
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
        <Typography variant="body2" color="error" sx={{ mt: 1.6 }}>
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
              borderRadius: '10px',
              overflow: 'hidden',
              border: '1px solid rgba(17,17,17,0.14)',
              bgcolor: '#fffcf3',
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
                <Typography variant="caption" sx={{ color: '#86807a', fontFamily: tokens.font.mono }}>
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
                  bgcolor: 'rgba(17,17,17,0.78)',
                  color: '#fff',
                  '&:hover': { bgcolor: 'rgba(17,17,17,0.92)' },
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
        title="Launch"
        subtitle="Review the brief. When you click Build it, Ironflyer creates the project, attaches your references, and starts the finisher loop."
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
            borderRadius: '10px',
            bgcolor: '#fffcf3',
            border: '1px solid rgba(17,17,17,0.1)',
            maxHeight: 240,
            overflowY: 'auto',
            whiteSpace: 'pre-wrap',
            fontSize: 14,
            lineHeight: 1.5,
            color: tokens.color.text.inverse,
          }}>
            {idea.trim() || 'No prompt provided yet — go back to step 1.'}
          </Box>
        </Box>

        <Stack spacing={2}>
          <Box sx={launchCardSx}>
            <PreviewLabel icon={<Bolt sx={{ fontSize: 14 }} />} label="Stack" />
            <Typography variant="subtitle2" sx={{ mt: 1, fontWeight: 900 }}>{stackChoice.label}</Typography>
            <Typography variant="caption" sx={{ color: '#5f5a52' }}>{stackChoice.detail}</Typography>
          </Box>

          <Box sx={launchCardSx}>
            <PreviewLabel icon={<ImageIcon sx={{ fontSize: 14 }} />} label="References" />
            {refs.length === 0 ? (
              <Typography variant="body2" sx={{ mt: 1, color: '#5f5a52' }}>
                None. The UX gate will use defaults.
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
                      borderRadius: '8px',
                      objectFit: 'cover',
                      border: '1px solid rgba(17,17,17,0.12)',
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
        borderRadius: '12px',
        bgcolor: tokens.color.accent.lime,
        color: tokens.color.text.inverse,
        display: 'flex',
        flexDirection: { xs: 'column', sm: 'row' },
        alignItems: { xs: 'stretch', sm: 'center' },
        justifyContent: 'space-between',
        gap: 2,
      }}>
        <Box>
          <Typography variant="subtitle1" sx={{ fontWeight: 900 }}>
            Ready to ship a finished version
          </Typography>
          <Typography variant="body2" sx={{ mt: 0.4, color: '#3f4900' }}>
            The first run typically lands a working preview in under a minute. Gates run in the background after that.
          </Typography>
        </Box>
        <Stack direction="row" spacing={1} alignItems="center" sx={{ flexShrink: 0 }}>
          <Schedule sx={{ fontSize: 18, color: '#3f4900' }} />
          <Typography variant="body2" sx={{ fontWeight: 900, color: '#3f4900' }}>
            ~60 seconds
          </Typography>
        </Stack>
      </Box>

      {launching && (
        <Box sx={{ mt: 2 }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <RocketLaunch sx={{ fontSize: 18, color: '#6f7e00' }} />
            <Typography variant="body2" sx={{ fontWeight: 800 }}>{launchStage}</Typography>
          </Stack>
          <LinearProgress sx={{
            mt: 1,
            height: 6,
            borderRadius: '999px',
            bgcolor: 'rgba(17,17,17,0.08)',
            '& .MuiLinearProgress-bar': { bgcolor: tokens.color.accent.lime },
          }} />
        </Box>
      )}

      {launchError && (
        <Box sx={{
          mt: 2,
          p: 1.6,
          borderRadius: '10px',
          border: '1px solid rgba(220,38,38,0.4)',
          bgcolor: 'rgba(220,38,38,0.08)',
        }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 900, color: '#9b1d1d' }}>
            Could not start the run
          </Typography>
          <Typography variant="body2" sx={{ mt: 0.4, color: '#7a1d1d' }}>{launchError}</Typography>
          <Button
            variant="outlined"
            size="small"
            onClick={onLaunch}
            sx={{ mt: 1.2, borderColor: 'rgba(155,29,29,0.5)', color: '#9b1d1d' }}
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
      borderTop: '1px solid rgba(17,17,17,0.1)',
      bgcolor: 'rgba(244,240,232,0.96)',
      backdropFilter: 'blur(14px)',
      position: 'sticky',
      bottom: 0,
    }}>
      <Stack
        direction="row"
        justifyContent="space-between"
        alignItems="center"
        sx={{ px: { xs: 2.4, md: 6 }, py: 1.6, gap: 1.5 }}
      >
        <Button
          variant="text"
          startIcon={<ArrowBack />}
          disabled={step === 0 || launching}
          onClick={onBack}
          sx={{ color: '#5f5a52', fontWeight: 800, visibility: step === 0 ? 'hidden' : 'visible' }}
        >
          Back
        </Button>

        {isOptional && (
          <Typography variant="caption" sx={{ color: '#86807a', fontWeight: 700, display: { xs: 'none', sm: 'block' } }}>
            Optional step — skip if you want
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
            {launching ? 'Launching...' : 'Build it'}
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
    <Stack direction="row" spacing={0.7} alignItems="center" sx={{ color: '#6f7e00' }}>
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
  bgcolor: '#fffcf3',
  color: tokens.color.text.inverse,
  borderColor: 'rgba(17,17,17,0.14)',
  borderRadius: '10px',
  fontWeight: 800,
  py: 1.1,
  '&:hover': {
    borderColor: 'rgba(17,17,17,0.32)',
    bgcolor: 'rgba(229,255,0,0.24)',
  },
};

const launchCardSx = {
  p: 2,
  borderRadius: '12px',
  bgcolor: '#f8f4ec',
  border: '1px solid rgba(17,17,17,0.12)',
};

const primaryButtonSx = {
  bgcolor: tokens.color.accent.lime,
  color: tokens.color.text.inverse,
  borderRadius: '10px',
  fontWeight: 900,
  px: 2.6,
  py: 1.1,
  '&:hover': { bgcolor: tokens.color.accent.lime },
  '&.Mui-disabled': {
    bgcolor: 'rgba(17,17,17,0.08)',
    color: 'rgba(17,17,17,0.32)',
  },
};

const launchButtonSx = {
  ...primaryButtonSx,
  px: 3,
  fontSize: '1rem',
};

// Avoids an unused-import warning when Folder is referenced only conditionally
// in future iterations; keep here so the icon stays imported intentionally.
void Folder;
