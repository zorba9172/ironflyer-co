// Language identity colors used by file-tree badges. These are the
// canonical brand colors per language (the same palette GitHub
// Linguist publishes), not Ironflyer's UI palette — they live in the
// design-tokens package so the constitutional "no raw hex outside
// tokens" rule still holds for components that render extension
// badges.
//
// `badge.color` is the chip background. The chip foreground always
// uses `tokens.color.text.primary` so contrast stays readable on the
// dark theme regardless of the chip color.

export interface LanguageBadge {
  label: string;
  color: string;
}

export const languageBadges: Record<string, LanguageBadge> = {
  ts: { label: 'TS', color: '#3178C6' },
  tsx: { label: 'TSX', color: '#3178C6' },
  js: { label: 'JS', color: '#E0BC2A' },
  jsx: { label: 'JSX', color: '#E0BC2A' },
  json: { label: 'JSON', color: '#8A6A1A' },
  go: { label: 'GO', color: '#00ADD8' },
  py: { label: 'PY', color: '#3572A5' },
  rs: { label: 'RS', color: '#B7410E' },
  md: { label: 'MD', color: '#6B7280' },
  yml: { label: 'YML', color: '#9333EA' },
  yaml: { label: 'YML', color: '#9333EA' },
  toml: { label: 'TOML', color: '#9333EA' },
  html: { label: 'HTM', color: '#E34F26' },
  css: { label: 'CSS', color: '#1572B6' },
  scss: { label: 'SCSS', color: '#CC6699' },
  sh: { label: 'SH', color: '#4EAA25' },
  sql: { label: 'SQL', color: '#336791' },
  env: { label: 'ENV', color: '#A16207' },
  lock: { label: 'LCK', color: '#6B7280' },
  graphql: { label: 'GQL', color: '#E10098' },
  gql: { label: 'GQL', color: '#E10098' },
  dockerfile: { label: 'DOCK', color: '#2496ED' },
};
