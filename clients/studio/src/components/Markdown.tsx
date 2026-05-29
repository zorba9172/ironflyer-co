import { useState } from 'react';
import { Box, Collapse, Link, Stack, Typography } from '@mui/material';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { VscChevronRight, VscChevronDown, VscFileCode, VscCopy } from 'react-icons/vsc';
import { toast } from '@ironflyer/ui-web/fx';
import { TechIcon } from '../lib/techIcons';
import type { ReactNode } from 'react';
import { text } from '@ironflyer/design-tokens/brand';

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

// Pull the "lang path" pair off a fenced block's info string so the header can
// show a real filename + matching file-type icon (e.g. ```tsx src/App.tsx).
function parseInfo(className?: string, raw?: string): { lang?: string; path?: string } {
  const lang = /language-([\w.+-]+)/.exec(className ?? '')?.[1];
  const tokens = (raw ?? '').trim().split(/\s+/).filter(Boolean);
  const path = tokens.find((t) => /[\\/]/.test(t) || /\.\w{1,8}$/.test(t));
  return { lang, path };
}

// Collapsed-by-default code block: the operator sees a compact file chip, not a
// wall of open files. Click the header to expand; copy without expanding.
function CodeBlock({ className, children, meta }: { className?: string; children?: ReactNode; meta?: string }) {
  const code = String(children ?? '').replace(/\n$/, '');
  const { lang, path } = parseInfo(className, meta);
  const title = path ?? lang ?? 'code';
  const iconKey = path ? path.split('.').pop() ?? 'file' : lang ?? 'file';
  const lines = code.split('\n').length;
  const [open, setOpen] = useState(false);

  return (
    <Box sx={{ my: 1, borderRadius: 2, border: 1, borderColor: 'divider', overflow: 'hidden' }}>
      <Stack
        direction="row" alignItems="center" spacing={1}
        onClick={() => setOpen((o) => !o)}
        sx={{ px: 1.25, py: 0.75, bgcolor: 'action.hover', cursor: 'pointer', '&:hover': { bgcolor: 'action.selected' } }}
      >
        <Box sx={{ display: 'inline-flex', color: 'text.secondary' }}>{open ? <VscChevronDown size={14} /> : <VscChevronRight size={14} />}</Box>
        <Box sx={{ display: 'inline-flex', color: 'text.secondary' }}>{path ? <TechIcon name={iconKey} size={14} title={title} /> : <VscFileCode size={14} />}</Box>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s72, color: 'text.primary', flex: 1 })} noWrap>{title}</Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s64, color: 'text.disabled' })}>{lines} ln</Typography>
        <Box
          component="button"
          onClick={(e) => { e.stopPropagation(); navigator.clipboard?.writeText(code); toast('Copied', 'success'); }}
          sx={{ display: 'inline-flex', alignItems: 'center', border: 0, background: 'none', color: 'text.secondary', cursor: 'pointer', p: 0.25, '&:hover': { color: 'text.primary' } }}
          aria-label="Copy code"
        >
          <VscCopy size={14} />
        </Box>
      </Stack>
      <Collapse in={open} unmountOnExit>
        <Box component="pre" sx={(t) => ({ m: 0, p: 1.5, overflow: 'auto', bgcolor: 'background.default', borderTop: 1, borderColor: 'divider', fontFamily: t.brand.font.mono, fontSize: text.s78, lineHeight: 1.5 })}>
          <code>{code}</code>
        </Box>
      </Collapse>
    </Box>
  );
}

// Renders the agent's markdown cleanly, theme-mapped. Code blocks collapse to a
// file chip; images render inline (click to open full size); inline code,
// headings, lists, and links are styled.
export function Markdown({ children }: { children: string }) {
  return (
    <Box sx={{ '& > :first-of-type': { mt: 0 }, '& > :last-child': { mb: 0 } }}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h1: ({ children }) => <Typography variant="h6" sx={{ fontSize: text.s105, mt: 2, mb: 1 }}>{children}</Typography>,
          h2: ({ children }) => <Typography variant="h6" sx={{ fontSize: text.s100, mt: 2, mb: 1 }}>{children}</Typography>,
          h3: ({ children }) => <Typography sx={{ fontWeight: 700, mt: 1.5, mb: 0.75 }}>{children}</Typography>,
          p: ({ children }) => <Typography sx={{ fontSize: text.s90, lineHeight: 1.6, my: 1 }}>{children}</Typography>,
          ul: ({ children }) => <Box component="ul" sx={{ pl: 2.5, my: 1, '& li': { fontSize: text.s90, lineHeight: 1.6, mb: 0.5 } }}>{children}</Box>,
          ol: ({ children }) => <Box component="ol" sx={{ pl: 2.5, my: 1, '& li': { fontSize: text.s90, lineHeight: 1.6, mb: 0.5 } }}>{children}</Box>,
          a: ({ children, href }) => {
            const safe = safeMarkdownUrl(href, 'link');
            return <Link href={safe} target={safe?.startsWith('http') ? '_blank' : undefined} rel="noreferrer" sx={{ color: 'primary.main' }}>{children}</Link>;
          },
          strong: ({ children }) => <Box component="strong" sx={{ fontWeight: 700 }}>{children}</Box>,
          img: ({ src, alt }) => (
            <Box
              component="a" href={safeMarkdownUrl(typeof src === 'string' ? src : undefined, 'image')} target="_blank" rel="noreferrer"
              sx={{ display: 'block', my: 1.25 }}
            >
              <Box component="img" src={safeMarkdownUrl(typeof src === 'string' ? src : undefined, 'image')} alt={alt ?? ''} loading="lazy" sx={{ maxWidth: '100%', borderRadius: 2, border: 1, borderColor: 'divider', display: 'block' }} />
            </Box>
          ),
          code: ({ node, className, children }) => {
            const meta = (node?.data as { meta?: unknown } | undefined)?.meta;
            return className?.startsWith('language-') ? (
              <CodeBlock className={className} meta={typeof meta === 'string' ? meta : undefined}>{children}</CodeBlock>
            ) : (
              <Box component="code" sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.82em', bgcolor: 'action.hover', px: 0.5, py: 0.1, borderRadius: 0.5 })}>{children}</Box>
            );
          },
          pre: ({ children }) => <>{children}</>,
        }}
      >
        {children}
      </ReactMarkdown>
    </Box>
  );
}
