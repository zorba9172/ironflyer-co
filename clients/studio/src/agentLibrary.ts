// The building blocks an operator composes an agent from: skills, tools,
// models, guardrails, autonomy levels — plus a describe-to-scaffold helper that
// turns one sentence into a draft agent. These mirror the orchestrator's real
// capability router + patch lifecycle so a configured agent maps to something
// the finisher can actually run. No backend yet — fixture catalogs.

import type { Agent, AgentAutonomy } from './studioData';

// --- Skills (capabilities) ---------------------------------------------
export type SkillCategory =
  | 'Engineering' | 'Data' | 'Security' | 'Product' | 'Ops' | 'Research' | 'Growth';

export interface SkillDef {
  id: string;
  label: string;
  category: SkillCategory;
  desc: string;
}

export const SKILL_CATEGORIES: SkillCategory[] = [
  'Engineering', 'Data', 'Security', 'Product', 'Ops', 'Research', 'Growth',
];

export const SKILL_LIBRARY: SkillDef[] = [
  // Engineering
  { id: 'planning', label: 'Planning', category: 'Product', desc: 'Turn an idea into the minimum coherent product with testable stories.' },
  { id: 'patch', label: 'Code patches', category: 'Engineering', desc: 'Write reviewable patches end-to-end through the lifecycle gates.' },
  { id: 'review', label: 'Code review', category: 'Engineering', desc: 'Catch unhandled errors, missing auth checks, stale types.' },
  { id: 'refactor', label: 'Refactoring', category: 'Engineering', desc: 'Restructure code without changing behavior; reduce diff debt.' },
  { id: 'frontend', label: 'Frontend', category: 'Engineering', desc: 'React/MUI screens with empty, loading, and error states.' },
  { id: 'backend', label: 'Backend', category: 'Engineering', desc: 'Routes, handlers, validators, auth wiring.' },
  { id: 'api-design', label: 'API design', category: 'Engineering', desc: 'GraphQL/REST contracts, versioning, pagination.' },
  // Data
  { id: 'migrations', label: 'Migrations', category: 'Data', desc: 'Reversible schema evolution with a down() side.' },
  { id: 'indexes', label: 'Indexing', category: 'Data', desc: 'Query plans, indexes, hot-path tuning.' },
  { id: 'sql', label: 'SQL', category: 'Data', desc: 'Author and audit queries against the project schema.' },
  { id: 'analytics', label: 'Analytics', category: 'Data', desc: 'Event modeling, funnels, retention cohorts.' },
  { id: 'backups', label: 'Backups', category: 'Data', desc: 'Snapshot, restore, and retention policy.' },
  // Security
  { id: 'secrets', label: 'Secret hygiene', category: 'Security', desc: 'Find committed secrets; route them to the vault.' },
  { id: 'sast', label: 'SAST', category: 'Security', desc: 'Static analysis with false-positive tie-breaking in context.' },
  { id: 'policy', label: 'Policy', category: 'Security', desc: 'Deny-by-default policy decisions with obligations.' },
  { id: 'authz', label: 'AuthZ', category: 'Security', desc: 'Ownership checks, scoped access, role confusion.' },
  { id: 'threat-model', label: 'Threat modeling', category: 'Security', desc: 'OWASP Top-10 reasoning over the attack surface.' },
  // Product
  { id: 'ux', label: 'UX', category: 'Product', desc: 'Screens with one primary action and explicit states.' },
  { id: 'copy', label: 'Product copy', category: 'Product', desc: 'Precise, senior, builder-facing UI and marketing copy.' },
  { id: 'spec', label: 'Spec writing', category: 'Product', desc: 'Acceptance criteria a tester can verify by observation.' },
  // Ops
  { id: 'deploy', label: 'Deploy', category: 'Ops', desc: 'Ship to a domain you own with a rollback path.' },
  { id: 'rollback', label: 'Rollback', category: 'Ops', desc: 'Safe revert with readiness signals.' },
  { id: 'dns', label: 'DNS', category: 'Ops', desc: 'Records, certs, domain wiring.' },
  { id: 'observability', label: 'Observability', category: 'Ops', desc: 'Health endpoints, metrics, structured logs.' },
  { id: 'expo', label: 'Mobile builds', category: 'Ops', desc: 'Expo / Gradle / Xcode build & signing.' },
  // Research
  { id: 'research', label: 'Research', category: 'Research', desc: 'Ground the build with sourced, summarized findings.' },
  { id: 'summarize', label: 'Summarize', category: 'Research', desc: 'Distill long context into decision-ready notes.' },
  { id: 'competitive', label: 'Competitive scan', category: 'Research', desc: 'Survey competitors and extract copyable patterns.' },
  // Growth
  { id: 'seo', label: 'SEO', category: 'Growth', desc: 'Metadata, structured data, content structure.' },
  { id: 'lifecycle', label: 'Lifecycle', category: 'Growth', desc: 'Onboarding, retention, transactional flows.' },
];

export const SKILL_BY_ID: Record<string, SkillDef> = Object.fromEntries(SKILL_LIBRARY.map((s) => [s.id, s]));

export function skillLabel(id: string): string {
  return SKILL_BY_ID[id]?.label ?? id;
}

// --- Tools / connectors ------------------------------------------------
export type ToolCategory = 'Workspace' | 'Integrations' | 'Knowledge' | 'Web';

export interface ToolDef {
  id: string;
  label: string;
  category: ToolCategory;
  desc: string;
}

export const TOOL_LIBRARY: ToolDef[] = [
  { id: 'fs_read', label: 'Read files', category: 'Workspace', desc: 'Read the project tree in the sandbox.' },
  { id: 'fs_write', label: 'Write patches', category: 'Workspace', desc: 'Propose patches through the lifecycle gates.' },
  { id: 'shell', label: 'Run commands', category: 'Workspace', desc: 'Execute build/test/lint commands in the sandbox.' },
  { id: 'browser', label: 'Headless browser', category: 'Web', desc: 'Drive a chromium to verify the running preview.' },
  { id: 'web_search', label: 'Web search', category: 'Web', desc: 'Search the public web for grounding.' },
  { id: 'http', label: 'HTTP requests', category: 'Web', desc: 'Call external APIs with timeouts + bounded retries.' },
  { id: 'github', label: 'GitHub', category: 'Integrations', desc: 'Branches, PRs, issues, Actions.' },
  { id: 'stripe', label: 'Stripe', category: 'Integrations', desc: 'Payments, webhooks, reconciliation.' },
  { id: 'postgres', label: 'Postgres', category: 'Knowledge', desc: 'Query and migrate the project database.' },
  { id: 'vector', label: 'Vector memory', category: 'Knowledge', desc: 'Semantic search over project knowledge.' },
  { id: 'atlas', label: 'Code Atlas', category: 'Knowledge', desc: 'Reuse-first search over existing symbols.' },
  { id: 'figma', label: 'Figma', category: 'Integrations', desc: 'Pull a design extract and translate to code.' },
];

export const TOOL_BY_ID: Record<string, ToolDef> = Object.fromEntries(TOOL_LIBRARY.map((t) => [t.id, t]));

export function toolLabel(id: string): string {
  return TOOL_BY_ID[id]?.label ?? id;
}

// --- Models (mirrors the orchestrator capability router) ---------------
export interface ModelOption {
  id: string;
  label: string;
  tier: string;
  note: string;
}

export const MODEL_OPTIONS: ModelOption[] = [
  { id: 'opus', label: 'Claude Opus 4.7', tier: 'Quality', note: 'Deep reasoning, planning, hard reviews. Priciest per token.' },
  { id: 'sonnet', label: 'Claude Sonnet 4.6', tier: 'Balanced', note: 'The default for general build work.' },
  { id: 'haiku', label: 'Claude Haiku 4.5', tier: 'Fast', note: 'Cheap, fast passes — inline checks, summaries.' },
];

export const MODEL_BY_ID: Record<string, ModelOption> = Object.fromEntries(MODEL_OPTIONS.map((m) => [m.id, m]));

export function modelLabel(id?: string): string {
  return id ? MODEL_BY_ID[id]?.label ?? id : 'Inherit from orchestrator';
}

// --- Guardrails (deny-by-default posture) ------------------------------
export interface GuardrailDef {
  id: string;
  label: string;
  desc: string;
}

export const GUARDRAILS: GuardrailDef[] = [
  { id: 'patch_review', label: 'Patches reviewed before apply', desc: 'Every change goes through the lifecycle gates — no direct writes.' },
  { id: 'no_secrets', label: 'No secrets in source', desc: 'Block any patch that commits a credential; route to the vault.' },
  { id: 'budget_cap', label: 'Respect the wallet', desc: 'ProfitGuard refuses calls that would push the wallet negative.' },
  { id: 'owner_check', label: 'Owner check', desc: 'Only touch resources owned by the current user.' },
  { id: 'deny_policy', label: 'Deny-by-default policy', desc: 'High-risk actions need an explicit allow decision + obligations.' },
  { id: 'redact_pii', label: 'Redact PII', desc: 'Strip personal data from model context before sending.' },
  { id: 'scope_lock', label: 'Stay in scope', desc: 'Only edit files inside the assigned areas of responsibility.' },
  { id: 'human_deploy', label: 'Human approves deploys', desc: 'A person signs off before anything ships to production.' },
];

export const GUARDRAIL_BY_ID: Record<string, GuardrailDef> = Object.fromEntries(GUARDRAILS.map((g) => [g.id, g]));

// --- Autonomy ----------------------------------------------------------
export const AUTONOMY_LEVELS: { value: AgentAutonomy; label: string; desc: string }[] = [
  { value: 'suggest', label: 'Suggest only', desc: 'Proposes patches and findings; never applies anything.' },
  { value: 'approval', label: 'Apply with approval', desc: 'Applies its work once a human approves the gate.' },
  { value: 'autonomous', label: 'Autonomous', desc: 'Applies within its budget + guardrails without waiting.' },
];

export function autonomyLabel(a?: AgentAutonomy): string {
  return AUTONOMY_LEVELS.find((x) => x.value === a)?.label ?? 'Apply with approval';
}

// --- Describe-to-scaffold ----------------------------------------------
// One sentence in, a draft agent out. Heuristic keyword routing — good enough
// to beat a blank canvas. The real product asks a model; this stays offline.
interface ScaffoldRule {
  match: RegExp;
  skills: string[];
  tools?: string[];
  gateId?: string;
  guardrails?: string[];
}

const SCAFFOLD_RULES: ScaffoldRule[] = [
  { match: /secur|secret|vuln|owasp|pentest|cve/i, skills: ['secrets', 'sast', 'threat-model', 'policy'], tools: ['fs_read', 'atlas'], gateId: 'security', guardrails: ['no_secrets', 'deny_policy'] },
  { match: /pay|stripe|billing|checkout|invoice|webhook/i, skills: ['backend', 'sql'], tools: ['stripe', 'fs_write', 'http'], gateId: 'money', guardrails: ['no_secrets', 'owner_check'] },
  { match: /migrat|schema|database|index|postgres|sql/i, skills: ['migrations', 'indexes', 'sql'], tools: ['postgres', 'fs_write'], gateId: 'data' },
  { match: /deploy|ship|release|rollback|domain|dns|infra/i, skills: ['deploy', 'rollback', 'dns', 'observability'], tools: ['shell', 'github'], gateId: 'deploy', guardrails: ['human_deploy'] },
  { match: /research|investigat|explore|ground|einstein|study|paper/i, skills: ['research', 'summarize', 'competitive'], tools: ['web_search', 'vector'] },
  { match: /design|ux|figma|screen|layout|ui\b/i, skills: ['ux', 'frontend', 'copy'], tools: ['figma', 'fs_write'] },
  { match: /mobile|ios|android|expo|app store/i, skills: ['expo', 'frontend'], tools: ['shell', 'fs_write'], gateId: 'signal' },
  { match: /auth|login|session|rbac|identity|role/i, skills: ['authz', 'backend'], tools: ['fs_write', 'atlas'], gateId: 'identity', guardrails: ['owner_check'] },
  { match: /seo|growth|onboard|retention|marketing|funnel/i, skills: ['seo', 'lifecycle', 'analytics', 'copy'], tools: ['web_search'] },
  { match: /test|verif|qa|preview|e2e/i, skills: ['review'], tools: ['browser', 'shell'] },
];

const TITLECASE = (s: string) => s.replace(/\b\w/g, (c) => c.toUpperCase());

// Pull a short, human name out of the prompt — first proper noun or the first
// few words, capped. "Build an Einstein research agent" → "Einstein".
function deriveName(prompt: string): string {
  const quoted = prompt.match(/["“']([^"”']{2,30})["”']/)?.[1];
  if (quoted) return TITLECASE(quoted.trim());
  const named = prompt.match(/\b(?:called|named)\s+([A-Z][\w-]+)/)?.[1];
  if (named) return TITLECASE(named);
  const proper = prompt.match(/\b([A-Z][a-z]{2,})\b/)?.[1];
  if (proper && !/^(Build|Make|Create|Add|The|Research|Agent|Find)$/.test(proper)) return proper;
  const words = prompt.trim().replace(/^(build|make|create|add|set up|an?|the)\s+/i, '').split(/\s+/).slice(0, 2).join(' ');
  return words ? TITLECASE(words) : 'New agent';
}

export function scaffoldAgent(prompt: string, base: Agent): Agent {
  const text = prompt.trim();
  if (!text) return base;
  const skills = new Set<string>();
  const tools = new Set<string>(['fs_read']);
  const guardrails = new Set<string>(['patch_review', 'no_secrets']);
  let gateId: string | undefined;

  for (const rule of SCAFFOLD_RULES) {
    if (rule.match.test(text)) {
      rule.skills.forEach((s) => skills.add(s));
      rule.tools?.forEach((t) => tools.add(t));
      rule.guardrails?.forEach((g) => guardrails.add(g));
      gateId = gateId ?? rule.gateId;
    }
  }
  // Always-on baseline if nothing matched.
  if (skills.size === 0) { skills.add('research'); skills.add('summarize'); tools.add('web_search'); }

  const name = deriveName(text);
  const role = text.length > 90 ? `${text.slice(0, 88)}…` : text;

  return {
    ...base,
    name,
    role: TITLECASE(role.replace(/^(build|make|create|add|set up)\s+(an?|the)?\s*/i, '').trim()) || role,
    description: `Use when: ${text.replace(/\.$/, '')}.`,
    instructions: `${name} should: ${text}\n\nWork in small, reviewable patches. Report what is still open end-to-end before reporting done.`,
    skills: [...skills],
    tools: [...tools],
    guardrails: [...guardrails],
    gateId,
  };
}
