import type { IconType } from 'react-icons';
import {
  SiGo, SiReact, SiTypescript, SiJavascript, SiPython, SiRust, SiNodedotjs, SiVite,
  SiDocker, SiKubernetes, SiGraphql, SiCss, SiHtml5, SiEslint,
  SiStripe, SiSlack, SiSentry, SiAuth0, SiPostgresql, SiGithub, SiVercel,
} from 'react-icons/si';
import {
  VscShield, VscBug, VscKey, VscPulse, VscPackage, VscLayers, VscReferences,
  VscTrash, VscCopy, VscSymbolNamespace, VscRocket, VscDashboard, VscGraph,
  VscLaw, VscPass, VscGitPullRequest, VscFileCode, VscCreditCard,
} from 'react-icons/vsc';
import { vendorColor } from '@ironflyer/design-tokens/tech';

// Maps a tech / tool / vendor / gate key to a real brand or product icon.
// Brand icons carry their official color (from design-tokens); tool + gate
// icons are monochrome and inherit the surrounding themed `currentColor`.
interface Entry { Icon: IconType; color?: string }

const REGISTRY: Record<string, Entry> = {
  // languages & frameworks
  go: { Icon: SiGo, color: vendorColor.go },
  golang: { Icon: SiGo, color: vendorColor.go },
  react: { Icon: SiReact, color: vendorColor.react },
  tsx: { Icon: SiReact, color: vendorColor.react },
  jsx: { Icon: SiReact, color: vendorColor.react },
  typescript: { Icon: SiTypescript, color: vendorColor.typescript },
  ts: { Icon: SiTypescript, color: vendorColor.typescript },
  javascript: { Icon: SiJavascript, color: vendorColor.javascript },
  js: { Icon: SiJavascript, color: vendorColor.javascript },
  python: { Icon: SiPython, color: vendorColor.python },
  py: { Icon: SiPython, color: vendorColor.python },
  rust: { Icon: SiRust, color: vendorColor.rust },
  rs: { Icon: SiRust, color: vendorColor.rust },
  node: { Icon: SiNodedotjs, color: vendorColor.node },
  vite: { Icon: SiVite, color: vendorColor.vite },
  docker: { Icon: SiDocker, color: vendorColor.docker },
  dockerfile: { Icon: SiDocker, color: vendorColor.docker },
  kubernetes: { Icon: SiKubernetes, color: vendorColor.kubernetes },
  graphql: { Icon: SiGraphql, color: vendorColor.graphql },
  gql: { Icon: SiGraphql, color: vendorColor.graphql },
  css: { Icon: SiCss, color: vendorColor.css },
  scss: { Icon: SiCss, color: vendorColor.css },
  html: { Icon: SiHtml5, color: vendorColor.html },
  // areas (Performance "where it runs")
  frontend: { Icon: SiReact, color: vendorColor.react },
  backend: { Icon: SiGo, color: vendorColor.go },
  // vendors / integrations
  stripe: { Icon: SiStripe, color: vendorColor.stripe },
  paddle: { Icon: VscCreditCard, color: vendorColor.paddle },
  slack: { Icon: SiSlack, color: vendorColor.slack },
  sentry: { Icon: SiSentry, color: vendorColor.sentry },
  auth0: { Icon: SiAuth0, color: vendorColor.auth0 },
  postgres: { Icon: SiPostgresql, color: vendorColor.postgresql },
  postgresql: { Icon: SiPostgresql, color: vendorColor.postgresql },
  github: { Icon: SiGithub }, // near-black mark → themed currentColor
  vercel: { Icon: SiVercel }, // near-black mark → themed currentColor
  // quality tools (monochrome, themed)
  eslint: { Icon: SiEslint, color: vendorColor.eslint },
  lint: { Icon: SiEslint, color: vendorColor.eslint },
  jscpd: { Icon: VscCopy },
  dedup: { Icon: VscCopy },
  reuse_check: { Icon: VscCopy },
  knip: { Icon: VscTrash },
  deadcode: { Icon: VscTrash },
  madge: { Icon: VscReferences },
  dep_graph: { Icon: VscReferences },
  'dependency-cruiser': { Icon: VscLayers },
  arch_boundary: { Icon: VscLayers },
  'ts-morph': { Icon: VscSymbolNamespace },
  complexity: { Icon: VscSymbolNamespace },
  // performance gates
  lighthouse: { Icon: VscDashboard },
  perf_budget: { Icon: VscDashboard },
  bundle_size: { Icon: VscPackage },
  mobile_size: { Icon: VscPackage },
  mobile_bundle_analyzer: { Icon: VscPackage },
  mem_leak: { Icon: VscPulse },
  // security
  security: { Icon: VscShield },
  sast: { Icon: VscShield },
  vuln_scan: { Icon: VscBug },
  vulnerability: { Icon: VscBug },
  secrets: { Icon: VscKey },
  // log / activity kinds
  deploy: { Icon: VscRocket },
  profitguard: { Icon: VscLaw },
  budget: { Icon: VscLaw },
  patch: { Icon: VscGitPullRequest },
  ledger: { Icon: VscGraph },
  gate: { Icon: VscPass },
};

const DEFAULT: Entry = { Icon: VscFileCode };

export function techEntry(name: string): Entry {
  return REGISTRY[name.trim().toLowerCase()] ?? DEFAULT;
}

// Renders the icon for `name`. Brand color is applied when known; otherwise the
// icon inherits the parent's themed text color, so it reads on dark and light.
export function TechIcon({ name, size = 16, title }: { name: string; size?: number; title?: string }) {
  const { Icon, color } = techEntry(name);
  return <Icon size={size} color={color} title={title ?? name} aria-hidden={!title} />;
}
