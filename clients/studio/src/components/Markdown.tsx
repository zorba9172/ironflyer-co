import { useState } from 'react';
import { Box, Collapse, Link, Stack, Typography } from '@mui/material';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { VscChevronRight, VscChevronDown, VscFileCode, VscCopy } from 'react-icons/vsc';
import { toast } from '@ironflyer/ui-web/fx';
import { TechIcon } from '../lib/techIcons';
import type { ReactNode } from 'react';
import { text } from '@ironflyer/design-tokens/brand';

// ── URL safety guard ───────────────────────────────────────────────────────────
export function safeMarkdownUrl(raw?: string | null, kind: 'link' | 'image' = 'link'): string | undefined {
  if (!raw) return undefined;
  const value = raw.trim();
  if (value === '' || value.startsWith('#') || value.startsWith('/')) return value;
  try {
    const url = new URL(value);
    if (url.protocol === 'https:' || url.protocol === 'http:') return url.toString();
    if (kind === 'link' && url.protocol === 'mailto:') return url.toString();
  } catch {
    return undefined;
  }
  return undefined;
}

// ── Language/path parser for code block headers ────────────────────────────────
// Pulls the "lang path" pair off a fenced block's info string so the header can
// show a real filename + matching file-type icon (e.g. ```tsx src/App.tsx).
function parseInfo(className?: string, raw?: string): { lang?: string; path?: string } {
  const lang = /language-([\w.+-]+)/.exec(className ?? '')?.[1];
  const tokens = (raw ?? '').trim().split(/\s+/).filter(Boolean);
  const path = tokens.find((t) => /[\\/]/.test(t) || /\.\w{1,8}$/.test(t));
  return { lang, path };
}

// ── Code block ─────────────────────────────────────────────────────────────────
// Collapsed-by-default code block: the operator sees a compact file chip, not a
// wall of open code. Click the header to expand; copy without expanding.
function CodeBlock({ className, children, meta }: { className?: string; children?: ReactNode; meta?: string }) {
  const code = String(children ?? '').replace(/\n$/, '');
  const { lang, path } = parseInfo(className, meta);
  const title = path ?? lang ?? 'code';
  const iconKey = path ? path.split('.').pop() ?? 'file' : lang ?? 'file';
  const lines = code.split('\n').length;
  const [open, setOpen] = useState(false);

  return (
    <Box sx={(t) => ({
      my: 1, borderRadius: `${t.studio.radius.sm}px`,
      border: `1px solid ${t.palette.divider}`,
      overflow: 'hidden',
      transition: `border-color ${t.studio.motion.fast}`,
      '&:hover': { borderColor: t.palette.primary.main },
    })}>
      {/* Collapsible header */}
      <Stack
        direction="row" alignItems="center" spacing={0.875}
        onClick={() => setOpen((o) => !o)}
        sx={(t) => ({
          px: 1.25, py: 0.7,
          bgcolor: `${t.palette.text.primary}06`,
          cursor: 'pointer',
          '&:hover': { bgcolor: `${t.palette.text.primary}0b` },
          transition: `background-color ${t.studio.motion.fast}`,
          userSelect: 'none',
        })}
      >
        <Box sx={{ display: 'inline-flex', color: 'text.disabled' }}>
          {open ? <VscChevronDown size={13} /> : <VscChevronRight size={13} />}
        </Box>
        <Box sx={{ display: 'inline-flex', color: 'text.secondary' }}>
          {path ? <TechIcon name={iconKey} size={13} title={title} /> : <VscFileCode size={13} />}
        </Box>
        <Typography
          sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s72, color: 'text.primary', flex: 1, fontWeight: 500 })}
          noWrap
        >
          {title}
        </Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled' })}>
          {lines} ln
        </Typography>
        <Box
          component="button"
          onClick={(e) => { e.stopPropagation(); navigator.clipboard?.writeText(code); toast('Copied', 'success'); }}
          sx={(t) => ({
            display: 'inline-flex', alignItems: 'center',
            border: 0, background: 'none',
            color: 'text.secondary', cursor: 'pointer',
            p: 0.3, borderRadius: `${t.studio.radius.sm / 3}px`,
            '&:hover': { color: 'text.primary', bgcolor: `${t.palette.text.primary}0f` },
            transition: `color ${t.studio.motion.fast}`,
          })}
          aria-label="Copy code"
        >
          <VscCopy size={13} />
        </Box>
      </Stack>
      {/* Code content */}
      <Collapse in={open} unmountOnExit>
        <Box
          component="pre"
          sx={(t) => ({
            m: 0, p: 1.5,
            overflow: 'auto',
            bgcolor: 'background.default',
            borderTop: `1px solid ${t.palette.divider}`,
            fontFamily: t.brand.font.mono,
            fontSize: text.s76,
            lineHeight: 1.55,
          })}
        >
          <code>{code}</code>
        </Box>
      </Collapse>
    </Box>
  );
}

// ── Horizontal rule ────────────────────────────────────────────────────────────
function HRule() {
  return (
    <Box component="hr" sx={(t) => ({
      border: 'none',
      borderTop: `1px solid ${t.palette.divider}`,
      my: 1.5,
    })} />
  );
}

// ── Blockquote ─────────────────────────────────────────────────────────────────
function Blockquote({ children }: { children?: ReactNode }) {
  return (
    <Box component="blockquote" sx={(t) => ({
      m: 0, my: 1.25,
      pl: 1.5, py: 0.25,
      borderLeft: `3px solid ${t.palette.primary.main}`,
      bgcolor: `${t.palette.primary.main}08`,
      borderRadius: `0 ${t.studio.radius.sm / 2}px ${t.studio.radius.sm / 2}px 0`,
    })}>
      {children}
    </Box>
  );
}

// ── Main component ─────────────────────────────────────────────────────────────
// Renders the agent's markdown cleanly, entirely theme-mapped. Code blocks
// collapse to a file chip; images render inline (click to open full size);
// inline code, headings, lists, blockquotes, and links are all themed.
export function Markdown({ children }: { children: string }) {
  return (
    <Box sx={{ '& > :first-of-type': { mt: 0 }, '& > :last-child': { mb: 0 } }}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          // ── Headings ──────────────────────────────────────────────────────
          h1: ({ children: c }) => (
            <Typography variant="h6" sx={{ fontSize: text.s105, mt: 2, mb: 1, fontWeight: 700, letterSpacing: '-0.01em' }}>{c}</Typography>
          ),
          h2: ({ children: c }) => (
            <Typography variant="h6" sx={{ fontSize: text.s100, mt: 1.75, mb: 0.875, fontWeight: 700, letterSpacing: '-0.005em' }}>{c}</Typography>
          ),
          h3: ({ children: c }) => (
            <Typography sx={{ fontWeight: 700, fontSize: text.s90, mt: 1.5, mb: 0.625, letterSpacing: 0 }}>{c}</Typography>
          ),
          h4: ({ children: c }) => (
            <Typography sx={{ fontWeight: 600, fontSize: text.s86, mt: 1.25, mb: 0.5 }}>{c}</Typography>
          ),
          // ── Body text ─────────────────────────────────────────────────────
          p: ({ children: c }) => (
            <Typography sx={{ fontSize: text.s90, lineHeight: 1.65, my: 0.875, color: 'text.primary' }}>{c}</Typography>
          ),
          // ── Lists ─────────────────────────────────────────────────────────
          ul: ({ children: c }) => (
            <Box component="ul" sx={(t) => ({
              pl: 2.25, my: 0.875,
              '& li': { fontSize: text.s90, lineHeight: 1.65, mb: 0.375, color: t.palette.text.primary },
              '& li::marker': { color: t.palette.primary.main },
            })}>{c}</Box>
          ),
          ol: ({ children: c }) => (
            <Box component="ol" sx={(t) => ({
              pl: 2.5, my: 0.875,
              '& li': { fontSize: text.s90, lineHeight: 1.65, mb: 0.375, color: t.palette.text.primary },
              '& li::marker': { color: t.palette.primary.main, fontWeight: 600 },
            })}>{c}</Box>
          ),
          // ── Links ─────────────────────────────────────────────────────────
          a: ({ children: c, href }) => {
            const safe = safeMarkdownUrl(href, 'link');
            return (
              <Link
                href={safe}
                target={safe?.startsWith('http') ? '_blank' : undefined}
                rel="noreferrer"
                sx={(t) => ({
                  color: 'primary.main',
                  textDecorationColor: `${t.palette.primary.main}55`,
                  '&:hover': { textDecorationColor: t.palette.primary.main },
                })}
              >
                {c}
              </Link>
            );
          },
          // ── Emphasis ──────────────────────────────────────────────────────
          strong: ({ children: c }) => (
            <Box component="strong" sx={{ fontWeight: 700, color: 'text.primary' }}>{c}</Box>
          ),
          em: ({ children: c }) => (
            <Box component="em" sx={{ fontStyle: 'italic', color: 'text.secondary' }}>{c}</Box>
          ),
          // ── Images ────────────────────────────────────────────────────────
          img: ({ src, alt }) => (
            <Box
              component="a"
              href={safeMarkdownUrl(typeof src === 'string' ? src : undefined, 'image')}
              target="_blank" rel="noreferrer"
              sx={{ display: 'block', my: 1.25 }}
            >
              <Box
                component="img"
                src={safeMarkdownUrl(typeof src === 'string' ? src : undefined, 'image')}
                alt={alt ?? ''}
                loading="lazy"
                sx={(t) => ({
                  maxWidth: '100%',
                  borderRadius: `${t.studio.radius.sm}px`,
                  border: `1px solid ${t.palette.divider}`,
                  display: 'block',
                })}
              />
            </Box>
          ),
          // ── Code ──────────────────────────────────────────────────────────
          code: ({ node, className, children: c }) => {
            const meta = (node?.data as { meta?: unknown } | undefined)?.meta;
            return className?.startsWith('language-') ? (
              <CodeBlock className={className} meta={typeof meta === 'string' ? meta : undefined}>{c}</CodeBlock>
            ) : (
              <Box
                component="code"
                sx={(t) => ({
                  fontFamily: t.brand.font.mono,
                  fontSize: '0.82em',
                  bgcolor: `${t.palette.text.primary}0c`,
                  border: `1px solid ${t.palette.divider}`,
                  px: 0.6, py: 0.1,
                  borderRadius: `${t.studio.radius.sm / 3}px`,
                })}
              >
                {c}
              </Box>
            );
          },
          pre: ({ children: c }) => <>{c}</>,
          // ── Block decorations ──────────────────────────────────────────────
          blockquote: ({ children: c }) => <Blockquote>{c}</Blockquote>,
          hr: () => <HRule />,
          // ── Tables (GFM) ──────────────────────────────────────────────────
          table: ({ children: c }) => (
            <Box sx={{ overflowX: 'auto', my: 1.25 }}>
              <Box component="table" sx={(t) => ({
                width: '100%', borderCollapse: 'collapse',
                fontFamily: t.brand.font.mono, fontSize: text.s78,
                '& td, & th': {
                  px: 1.25, py: 0.625,
                  border: `1px solid ${t.palette.divider}`,
                  textAlign: 'left',
                  verticalAlign: 'top',
                },
                '& th': {
                  fontWeight: 700,
                  bgcolor: `${t.palette.text.primary}07`,
                  color: 'text.primary',
                  letterSpacing: '0.04em',
                  textTransform: 'uppercase',
                  fontSize: text.s66,
                },
                '& tr:hover td': { bgcolor: `${t.palette.text.primary}04` },
              })}>
                {c}
              </Box>
            </Box>
          ),
          thead: ({ children: c }) => <thead>{c}</thead>,
          tbody: ({ children: c }) => <tbody>{c}</tbody>,
          tr: ({ children: c }) => <tr>{c}</tr>,
          th: ({ children: c }) => <th>{c}</th>,
          td: ({ children: c }) => <td>{c}</td>,
        }}
      >
        {children}
      </ReactMarkdown>
    </Box>
  );
}
