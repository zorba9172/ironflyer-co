import { Box, Link, Typography } from '@mui/material';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { toast } from '@ironflyer/ui-web/fx';
import type { ReactNode } from 'react';

function CodeBlock({ className, children }: { className?: string; children?: ReactNode }) {
  const lang = /language-(\w+)/.exec(className ?? '')?.[1];
  const code = String(children ?? '').replace(/\n$/, '');
  return (
    <Box sx={{ position: 'relative', my: 1.5 }}>
      <Box sx={(t) => ({ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 1.5, py: 0.5, bgcolor: 'action.hover', borderTopLeftRadius: 8, borderTopRightRadius: 8, border: 1, borderColor: 'divider', borderBottom: 0, fontFamily: t.brand.font.mono, fontSize: '0.66rem', color: 'text.disabled' })}>
        <span>{lang ?? 'code'}</span>
        <Box component="button" onClick={() => { navigator.clipboard?.writeText(code); toast('Copied', 'success'); }} sx={{ border: 0, background: 'none', color: 'text.secondary', cursor: 'pointer', fontSize: '0.66rem', fontFamily: 'inherit', '&:hover': { color: 'text.primary' } }}>copy</Box>
      </Box>
      <Box component="pre" sx={(t) => ({ m: 0, p: 1.5, overflow: 'auto', bgcolor: 'background.default', border: 1, borderColor: 'divider', borderBottomLeftRadius: 8, borderBottomRightRadius: 8, fontFamily: t.brand.font.mono, fontSize: '0.78rem', lineHeight: 1.5 })}>
        <code>{code}</code>
      </Box>
    </Box>
  );
}

// Renders the agent's markdown cleanly, theme-mapped. Code blocks get a header
// + copy button; inline code, headings, lists, and links are styled.
export function Markdown({ children }: { children: string }) {
  return (
    <Box sx={{ '& > :first-of-type': { mt: 0 }, '& > :last-child': { mb: 0 } }}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h1: ({ children }) => <Typography variant="h6" sx={{ fontSize: '1.05rem', mt: 2, mb: 1 }}>{children}</Typography>,
          h2: ({ children }) => <Typography variant="h6" sx={{ fontSize: '1rem', mt: 2, mb: 1 }}>{children}</Typography>,
          h3: ({ children }) => <Typography sx={{ fontWeight: 700, mt: 1.5, mb: 0.75 }}>{children}</Typography>,
          p: ({ children }) => <Typography sx={{ fontSize: '0.9rem', lineHeight: 1.6, my: 1 }}>{children}</Typography>,
          ul: ({ children }) => <Box component="ul" sx={{ pl: 2.5, my: 1, '& li': { fontSize: '0.9rem', lineHeight: 1.6, mb: 0.5 } }}>{children}</Box>,
          ol: ({ children }) => <Box component="ol" sx={{ pl: 2.5, my: 1, '& li': { fontSize: '0.9rem', lineHeight: 1.6, mb: 0.5 } }}>{children}</Box>,
          a: ({ children, href }) => <Link href={href} target="_blank" rel="noreferrer" sx={{ color: 'primary.main' }}>{children}</Link>,
          strong: ({ children }) => <Box component="strong" sx={{ fontWeight: 700 }}>{children}</Box>,
          code: ({ className, children }) =>
            className?.startsWith('language-') ? (
              <CodeBlock className={className}>{children}</CodeBlock>
            ) : (
              <Box component="code" sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.82em', bgcolor: 'action.hover', px: 0.5, py: 0.1, borderRadius: 0.5 })}>{children}</Box>
            ),
          pre: ({ children }) => <>{children}</>,
        }}
      >
        {children}
      </ReactMarkdown>
    </Box>
  );
}
