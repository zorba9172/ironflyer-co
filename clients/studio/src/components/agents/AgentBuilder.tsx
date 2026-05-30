import { useState, type ReactNode } from 'react';
import {
  Box, Button, Chip, Dialog, Divider, FormControl, FormControlLabel, IconButton, InputLabel,
  MenuItem, Select, Stack, Switch, TextField, Tooltip, Typography,
} from '@mui/material';
import { toast } from '@ironflyer/ui-web/fx';
import { Icon } from '../../icons';
import { newAgent, type Agent, type Gate } from '../../studioData';
import {
  AUTONOMY_LEVELS, GUARDRAILS, MODEL_OPTIONS, SKILL_CATEGORIES, SKILL_LIBRARY, TOOL_LIBRARY,
  autonomyLabel, modelLabel, scaffoldAgent, skillLabel, toolLabel,
} from '../../agentLibrary';
import { AgentCard } from './AgentCard';
import { ScheduleEditor } from './ScheduleEditor';
import { text as fontScale } from '@ironflyer/design-tokens/brand';

interface AgentBuilderProps {
  agent: Agent;
  gates: Gate[];
  /** every agent in scope — used as hand-off targets */
  allAgents: Agent[];
  exists: boolean;
  onClose: () => void;
  onSave: (a: Agent) => void;
}

// The deep agent builder: a full-screen split surface. Left = a sectioned form
// for everything an agent is (identity, mission, skills, tools, areas of
// responsibility, reasoning, guardrails, knowledge, orchestration, schedule).
// Right = a live preview card + a dry-run tester that mirrors how the
// orchestrator would route the agent. Describe-to-scaffold seeds a draft from
// one sentence so the operator never starts on a blank canvas.
export function AgentBuilder({ agent, gates, allAgents, exists, onClose, onSave }: AgentBuilderProps) {
  const [draft, setDraft] = useState<Agent>(agent);
  const set = <K extends keyof Agent>(key: K, value: Agent[K]) => setDraft((d) => ({ ...d, [key]: value }));

  const valid = draft.name.trim().length > 0 && draft.role.trim().length > 0;

  const save = () => {
    const next: Agent = {
      ...draft,
      name: draft.name.trim(),
      role: draft.role.trim(),
      description: draft.description?.trim() || undefined,
      instructions: draft.instructions?.trim() || undefined,
      custom: true,
    };
    onSave(next);
    toast(`${next.name} ${exists ? 'updated' : 'created'}.`, 'success');
    onClose();
  };

  return (
    <Dialog open fullScreen onClose={onClose} slotProps={{ paper: { sx: { backgroundImage: 'none', bgcolor: 'background.default' } } }}>
      {/* Header */}
      <Stack direction="row" alignItems="center" spacing={2} sx={{ px: { xs: 2, md: 4 }, py: 1.75, borderBottom: 1, borderColor: 'divider', flexShrink: 0 }}>
        <IconButton onClick={onClose} size="small" aria-label="close" sx={{ color: 'text.secondary' }}>
          <Icon name="close" size={20} />
        </IconButton>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="h5" sx={{ fontSize: fontScale.s120 }} noWrap>{exists ? 'Edit agent' : 'New agent'}</Typography>
          <Typography sx={{ color: 'text.secondary', fontSize: fontScale.s80 }} noWrap>Define what it does, the skills and tools it may use, and when it runs.</Typography>
        </Box>
        <Button color="inherit" onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!valid} onClick={save}>{exists ? 'Save changes' : 'Create agent'}</Button>
      </Stack>

      {/* Body: form | preview */}
      <Box sx={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'minmax(0, 1fr) 380px' } }}>
        {/* Left — form */}
        <Box sx={{ overflowY: 'auto', px: { xs: 2, md: 4 }, py: 3 }}>
          <Box sx={{ maxWidth: 720, mx: 'auto' }}>
            <ScaffoldBanner onScaffold={(prompt) => setDraft((d) => scaffoldAgent(prompt, d.name || d.role ? d : newAgent()))} />

            <Section title="Identity" hint="The name and the routing signal — when should the orchestrator reach for this agent?">
              <TextField label="Name" value={draft.name} onChange={(e) => set('name', e.target.value)} fullWidth size="small" placeholder="Einstein" autoFocus />
              <TextField label="When to use it" value={draft.description ?? ''} onChange={(e) => set('description', e.target.value)} fullWidth size="small" placeholder="Use when the build needs grounded domain research before a gate runs." />
            </Section>

            <Section title="Mission" hint="The one-line objective and the step-by-step instructions the agent follows.">
              <TextField label="Objective" value={draft.role} onChange={(e) => set('role', e.target.value)} fullWidth size="small" placeholder="What this agent is for, in one line" />
              <TextField
                label="Instructions"
                value={draft.instructions ?? ''}
                onChange={(e) => set('instructions', e.target.value)}
                fullWidth multiline minRows={4}
                placeholder={'Exactly what the agent should do, step by step.\ne.g. Research the domain, summarize findings into a doc, flag risks for the Security gate.'}
              />
            </Section>

            <Section title="Skills" hint="The capabilities the agent may use. Picked skills bias the orchestrator's model router.">
              <CatalogPicker
                categories={SKILL_CATEGORIES}
                items={SKILL_LIBRARY}
                selected={draft.skills ?? []}
                onToggle={(id) => set('skills', toggle(draft.skills ?? [], id))}
              />
              <FreeAdd
                placeholder="Add a custom skill and press Enter"
                onAdd={(v) => { if (!(draft.skills ?? []).includes(v)) set('skills', [...(draft.skills ?? []), v]); }}
              />
              <SelectedRow ids={draft.skills ?? []} label={skillLabel} onRemove={(id) => set('skills', (draft.skills ?? []).filter((x) => x !== id))} />
            </Section>

            <Section title="Tools" hint="What the agent can call. Tools are an allowlist — it can only reach what you grant.">
              <CatalogPicker
                categories={['Workspace', 'Web', 'Integrations', 'Knowledge']}
                items={TOOL_LIBRARY}
                selected={draft.tools ?? []}
                onToggle={(id) => set('tools', toggle(draft.tools ?? [], id))}
              />
              <SelectedRow ids={draft.tools ?? []} label={toolLabel} onRemove={(id) => set('tools', (draft.tools ?? []).filter((x) => x !== id))} />
            </Section>

            <Section title="Areas of responsibility" hint="The domains or path globs this agent owns. With “Stay in scope” on, it only edits inside these.">
              <FreeAdd
                placeholder="e.g. src/payments/** or “billing & invoices” — Enter to add"
                onAdd={(v) => { if (!(draft.responsibilities ?? []).includes(v)) set('responsibilities', [...(draft.responsibilities ?? []), v]); }}
              />
              <SelectedRow mono ids={draft.responsibilities ?? []} onRemove={(id) => set('responsibilities', (draft.responsibilities ?? []).filter((x) => x !== id))} />
            </Section>

            <Section title="Reasoning" hint="The model it thinks with and how much rope it gets over applying changes.">
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
                <FormControl fullWidth size="small">
                  <InputLabel id="model-label">Model</InputLabel>
                  <Select labelId="model-label" label="Model" value={draft.model ?? 'sonnet'} onChange={(e) => set('model', e.target.value)}>
                    {MODEL_OPTIONS.map((m) => (
                      <MenuItem key={m.id} value={m.id}>
                        <Box>
                          <Typography sx={{ fontSize: fontScale.s86 }}>{m.label} · {m.tier}</Typography>
                          <Typography sx={{ fontSize: fontScale.s72, color: 'text.disabled' }}>{m.note}</Typography>
                        </Box>
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
                <FormControl fullWidth size="small">
                  <InputLabel id="autonomy-label">Autonomy</InputLabel>
                  <Select labelId="autonomy-label" label="Autonomy" value={draft.autonomy ?? 'approval'} onChange={(e) => set('autonomy', e.target.value as Agent['autonomy'])}>
                    {AUTONOMY_LEVELS.map((a) => (
                      <MenuItem key={a.value} value={a.value}>
                        <Box>
                          <Typography sx={{ fontSize: fontScale.s86 }}>{a.label}</Typography>
                          <Typography sx={{ fontSize: fontScale.s72, color: 'text.disabled' }}>{a.desc}</Typography>
                        </Box>
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </Stack>
            </Section>

            <Section title="Guardrails" hint="Hard limits the agent can't cross. Deny-by-default — start strict, loosen deliberately.">
              <Stack spacing={1}>
                {GUARDRAILS.map((g) => {
                  const on = (draft.guardrails ?? []).includes(g.id);
                  return (
                    <Box
                      key={g.id}
                      onClick={() => set('guardrails', toggle(draft.guardrails ?? [], g.id))}
                      sx={{ display: 'flex', alignItems: 'center', gap: 1.5, p: 1.25, borderRadius: 2, border: 1, borderColor: on ? 'primary.main' : 'divider', bgcolor: on ? (t) => `${t.palette.primary.main}14` : 'transparent', cursor: 'pointer', transition: 'border-color .12s, background-color .12s' }}
                    >
                      <Switch size="small" checked={on} onChange={() => set('guardrails', toggle(draft.guardrails ?? [], g.id))} onClick={(e) => e.stopPropagation()} />
                      <Box sx={{ minWidth: 0 }}>
                        <Typography sx={{ fontSize: fontScale.s86, fontWeight: 500 }}>{g.label}</Typography>
                        <Typography sx={{ fontSize: fontScale.s76, color: 'text.secondary' }}>{g.desc}</Typography>
                      </Box>
                    </Box>
                  );
                })}
              </Stack>
            </Section>

            <Section title="Knowledge" hint="Documents and references that ground the agent before it acts.">
              <FreeAdd
                placeholder="Attach a doc or note name and press Enter"
                onAdd={(v) => { if (!(draft.knowledge ?? []).includes(v)) set('knowledge', [...(draft.knowledge ?? []), v]); }}
              />
              <SelectedRow ids={draft.knowledge ?? []} onRemove={(id) => set('knowledge', (draft.knowledge ?? []).filter((x) => x !== id))} />
            </Section>

            <Section title="Orchestration" hint="Where this agent sits in the run and who it can hand off to.">
              <FormControl fullWidth size="small">
                <InputLabel id="gate-label">Assigned gate (optional)</InputLabel>
                <Select labelId="gate-label" label="Assigned gate (optional)" value={draft.gateId ?? ''} onChange={(e) => set('gateId', e.target.value || undefined)}>
                  <MenuItem value=""><em>Unassigned</em></MenuItem>
                  {gates.map((g) => <MenuItem key={g.id} value={g.id}>{g.no} · {g.name}</MenuItem>)}
                </Select>
              </FormControl>
              <FormControlLabel
                control={<Switch size="small" checked={!!draft.canDelegate} onChange={(e) => set('canDelegate', e.target.checked)} />}
                label={<Typography sx={{ fontSize: fontScale.s86 }}>Allow delegation — may hand work to other agents</Typography>}
              />
              {draft.canDelegate && (
                <FormControl fullWidth size="small">
                  <InputLabel id="handoff-label">Can hand off to</InputLabel>
                  <Select
                    labelId="handoff-label" label="Can hand off to" multiple
                    value={draft.handoffTo ?? []}
                    onChange={(e) => set('handoffTo', typeof e.target.value === 'string' ? e.target.value.split(',') : e.target.value)}
                    renderValue={(sel) => (sel as string[]).map((id) => allAgents.find((a) => a.id === id)?.name ?? id).join(', ')}
                  >
                    {allAgents.filter((a) => a.id !== draft.id).map((a) => <MenuItem key={a.id} value={a.id}>{a.name}</MenuItem>)}
                  </Select>
                </FormControl>
              )}
            </Section>

            <Section title="Schedule" hint="When the agent runs on its own. Manual agents only run when dispatched.">
              <ScheduleEditor schedule={draft.schedule ?? { mode: 'manual', enabled: true }} onChange={(s) => set('schedule', s)} />
            </Section>
          </Box>
        </Box>

        {/* Right — live preview + dry run */}
        <Box sx={{ borderLeft: { md: 1 }, borderColor: 'divider', bgcolor: 'background.paper', overflowY: 'auto', display: { xs: 'none', md: 'block' } }}>
          <PreviewPane agent={draft} gates={gates} />
        </Box>
      </Box>
    </Dialog>
  );
}

function toggle(list: string[], id: string): string[] {
  return list.includes(id) ? list.filter((x) => x !== id) : [...list, id];
}

// --- Section shell ------------------------------------------------------
function Section({ title, hint, children }: { title: string; hint: string; children: ReactNode }) {
  return (
    <Box sx={{ mb: 3.5 }}>
      <Typography variant="h6" sx={{ fontSize: fontScale.s98, mb: 0.25 }}>{title}</Typography>
      <Typography sx={{ color: 'text.secondary', fontSize: fontScale.s80, mb: 1.5 }}>{hint}</Typography>
      <Stack spacing={1.5}>{children}</Stack>
    </Box>
  );
}

// --- Describe-to-scaffold ----------------------------------------------
function ScaffoldBanner({ onScaffold }: { onScaffold: (prompt: string) => void }) {
  const [text, setText] = useState('');
  return (
    <Box sx={(t) => ({ p: 2, mb: 3.5, borderRadius: 3, border: 1, borderColor: 'divider', backgroundImage: t.studio.gradient.soft })}>
      <Typography sx={{ fontWeight: 600, fontSize: fontScale.s92, mb: 0.25 }}>Draft from a sentence</Typography>
      <Typography sx={{ color: 'text.secondary', fontSize: fontScale.s80, mb: 1.5 }}>Describe the agent and we'll pre-fill its skills, tools, guardrails, and gate. Tune from there.</Typography>
      <Stack direction="row" spacing={1}>
        <TextField
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); if (text.trim()) onScaffold(text.trim()); } }}
          fullWidth size="small"
          placeholder="A security agent that scans every patch for secrets and OWASP issues"
        />
        <Button variant="contained" disabled={!text.trim()} onClick={() => onScaffold(text.trim())} sx={{ flexShrink: 0 }}>Draft</Button>
      </Stack>
    </Box>
  );
}

// --- Catalog picker (grouped toggle chips) ------------------------------
function CatalogPicker<T extends { id: string; label: string; category: string; desc: string }>({
  categories, items, selected, onToggle,
}: { categories: readonly string[]; items: T[]; selected: string[]; onToggle: (id: string) => void }) {
  return (
    <Stack spacing={1.5}>
      {categories.map((cat) => {
        const group = items.filter((i) => i.category === cat);
        if (group.length === 0) return null;
        return (
          <Box key={cat}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s64, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 0.75 })}>{cat}</Typography>
            <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75 }}>
              {group.map((i) => {
                const on = selected.includes(i.id);
                return (
                  <Tooltip key={i.id} title={i.desc} arrow placement="top">
                    <Chip
                      label={i.label}
                      size="small"
                      onClick={() => onToggle(i.id)}
                      variant={on ? 'filled' : 'outlined'}
                      sx={(t) => ({
                        cursor: 'pointer',
                        borderColor: on ? 'transparent' : 'divider',
                        bgcolor: on ? `${t.palette.primary.main}26` : 'transparent',
                        color: on ? 'primary.main' : 'text.secondary',
                        fontWeight: on ? 600 : 500,
                        '&:hover': { bgcolor: on ? `${t.palette.primary.main}33` : 'action.hover' },
                      })}
                    />
                  </Tooltip>
                );
              })}
            </Stack>
          </Box>
        );
      })}
    </Stack>
  );
}

// --- Free-text adder ----------------------------------------------------
function FreeAdd({ placeholder, onAdd }: { placeholder: string; onAdd: (v: string) => void }) {
  const [v, setV] = useState('');
  const commit = () => { const t = v.trim(); if (t) { onAdd(t); setV(''); } };
  return (
    <TextField
      value={v}
      onChange={(e) => setV(e.target.value)}
      onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); commit(); } }}
      fullWidth size="small"
      placeholder={placeholder}
    />
  );
}

// --- Selected chips row -------------------------------------------------
function SelectedRow({ ids, label, mono, onRemove }: { ids: string[]; label?: (id: string) => string; mono?: boolean; onRemove: (id: string) => void }) {
  if (ids.length === 0) return null;
  return (
    <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.75 }}>
      {ids.map((id) => (
        <Chip
          key={id}
          label={label ? label(id) : id}
          size="small"
          onDelete={() => onRemove(id)}
          sx={(t) => ({ bgcolor: 'action.hover', fontFamily: mono ? t.brand.font.mono : undefined, fontSize: mono ? fontScale.s72 : undefined })}
        />
      ))}
    </Stack>
  );
}

// --- Preview + dry-run --------------------------------------------------
function PreviewPane({ agent, gates }: { agent: Agent; gates: Gate[] }) {
  const [probe, setProbe] = useState('');
  const [result, setResult] = useState<DryRun | null>(null);

  const run = () => setResult(simulate(agent, probe.trim()));

  return (
    <Box sx={{ p: 2.5 }}>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s66, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 1.25 })}>Live preview</Typography>
      <AgentCard agent={agent} gates={gates} builtIn />

      <Divider sx={{ my: 2.5 }} />

      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s66, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 0.75 })}>Dry run</Typography>
      <Typography sx={{ color: 'text.secondary', fontSize: fontScale.s80, mb: 1.5 }}>Hand the agent a task and see how the orchestrator would route it — model, tools, gate, and guardrails applied.</Typography>
      <TextField
        value={probe}
        onChange={(e) => setProbe(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); run(); } }}
        fullWidth size="small" multiline minRows={2}
        placeholder="e.g. The Stripe webhook accepts unsigned events — fix it."
      />
      <Button variant="outlined" color="inherit" fullWidth onClick={run} disabled={!probe.trim()} sx={{ mt: 1, borderColor: 'divider' }}>Dry run</Button>

      {result && (
        <Stack spacing={1.25} sx={{ mt: 2 }}>
          <ResultLine k="Model" v={modelLabel(agent.model)} />
          <ResultLine k="Autonomy" v={autonomyLabel(agent.autonomy)} />
          {result.gate && <ResultLine k="Routes to gate" v={result.gate} />}
          <ResultLine k="Skills engaged" v={result.skills.length ? result.skills.map(skillLabel).join(', ') : 'general reasoning'} />
          {result.tools.length > 0 && <ResultLine k="Tools used" v={result.tools.map(toolLabel).join(', ')} />}
          {result.guardrails.length > 0 && <ResultLine k="Guardrails applied" v={result.guardrails} />}
          <Box sx={{ p: 1.5, mt: 0.5, borderRadius: 2, bgcolor: 'background.default', border: 1, borderColor: 'divider' }}>
            <Typography sx={{ fontSize: fontScale.s82, color: 'text.secondary' }}>{result.plan}</Typography>
          </Box>
          <Typography sx={{ fontSize: fontScale.s72, color: 'text.disabled' }}>Offline simulation. Connect the orchestrator to dispatch a real run.</Typography>
        </Stack>
      )}
    </Box>
  );
}

function ResultLine({ k, v }: { k: string; v: string }) {
  return (
    <Stack direction="row" spacing={1.5} alignItems="baseline">
      <Typography sx={{ fontSize: fontScale.s76, color: 'text.disabled', minWidth: 116, flexShrink: 0 }}>{k}</Typography>
      <Typography sx={{ fontSize: fontScale.s82, color: 'text.primary' }}>{v}</Typography>
    </Stack>
  );
}

interface DryRun {
  gate?: string;
  skills: string[];
  tools: string[];
  guardrails: string;
  plan: string;
}

// Deterministic local routing: match the probe against the agent's configured
// skills/tools so the preview reflects exactly what was configured, not a guess.
function simulate(agent: Agent, probe: string): DryRun {
  const lower = probe.toLowerCase();
  const skills = (agent.skills ?? []).filter((s) => {
    const head = skillLabel(s).toLowerCase().split(' ')[0] ?? '';
    return (head && lower.includes(head)) || (agent.skills ?? []).length <= 3;
  });
  const tools = (agent.tools ?? []).slice(0, 4);
  const gate = agent.gateId;
  const guardrails = (agent.guardrails ?? []).length ? `${(agent.guardrails ?? []).length} active` : 'none';
  const verb = agent.autonomy === 'autonomous' ? 'apply' : agent.autonomy === 'suggest' ? 'propose' : 'apply (pending approval)';
  const plan = `${agent.name || 'The agent'} would ground itself, then ${verb} small patches addressing "${probe}". ${gate ? `Work lands behind the ${gate} gate.` : 'Unassigned — runs as a cross-cutting helper.'}`;
  return { gate, skills: skills.length ? skills : (agent.skills ?? []).slice(0, 3), tools, guardrails, plan };
}
