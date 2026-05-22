'use client';

// CodeBlock — a self-contained code surface for docs pages. Renders with a
// language label at the top-left and a Copy button at the top-right. We
// avoid pulling in a heavy syntax highlighter (Prism / Shiki) because docs
// pages should stay slim; the in-house tokenizer below covers the four
// languages we actually use in docs (bash, ts/tsx, json, http).

import { useState } from 'react';
import { Box, IconButton, Stack, Typography, Tooltip } from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import CheckIcon from '@mui/icons-material/Check';
import { tokens } from '../../../../packages/design-tokens';

export type CodeLanguage = 'bash' | 'typescript' | 'tsx' | 'json' | 'http' | 'go' | 'text';

export interface CodeBlockProps {
  language?: CodeLanguage;
  children: string;
  filename?: string;
}

const LANG_LABEL: Record<CodeLanguage, string> = {
  bash: 'bash',
  typescript: 'ts',
  tsx: 'tsx',
  json: 'json',
  http: 'http',
  go: 'go',
  text: 'text',
};

// Minimal tokenizer — returns spans we can colour. Strings, comments,
// keywords, numbers, and HTTP verbs are enough to give the eye structure
// without dragging in a 200 KB syntax library.
function tokenize(src: string, lang: CodeLanguage): Array<{ kind: string; value: string }> {
  if (lang === 'text') return [{ kind: 'text', value: src }];

  const keywords = new Set<string>(
    lang === 'typescript' || lang === 'tsx'
      ? [
          'import', 'from', 'export', 'const', 'let', 'var', 'function', 'return',
          'async', 'await', 'if', 'else', 'for', 'while', 'switch', 'case', 'break',
          'continue', 'new', 'class', 'extends', 'implements', 'interface', 'type',
          'enum', 'true', 'false', 'null', 'undefined', 'this', 'super', 'try',
          'catch', 'finally', 'throw',
        ]
      : lang === 'go'
      ? [
          'package', 'import', 'func', 'var', 'const', 'type', 'struct', 'interface',
          'return', 'if', 'else', 'for', 'range', 'switch', 'case', 'break',
          'continue', 'go', 'defer', 'chan', 'map', 'true', 'false', 'nil',
        ]
      : []
  );

  const httpVerbs = new Set(['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']);

  const out: Array<{ kind: string; value: string }> = [];
  let i = 0;
  const n = src.length;

  while (i < n) {
    const ch = src[i];

    // Line comments
    if ((lang === 'bash' && ch === '#') || ((lang === 'typescript' || lang === 'tsx' || lang === 'go') && ch === '/' && src[i + 1] === '/')) {
      let j = i;
      while (j < n && src[j] !== '\n') j++;
      out.push({ kind: 'comment', value: src.slice(i, j) });
      i = j;
      continue;
    }

    // Strings (single, double, backtick)
    if (ch === '"' || ch === "'" || ch === '`') {
      const quote = ch;
      let j = i + 1;
      while (j < n && src[j] !== quote) {
        if (src[j] === '\\') j++;
        j++;
      }
      out.push({ kind: 'string', value: src.slice(i, j + 1) });
      i = j + 1;
      continue;
    }

    // Numbers
    if (/[0-9]/.test(ch)) {
      let j = i;
      while (j < n && /[0-9._]/.test(src[j])) j++;
      out.push({ kind: 'number', value: src.slice(i, j) });
      i = j;
      continue;
    }

    // Identifiers / keywords / verbs
    if (/[A-Za-z_$]/.test(ch)) {
      let j = i;
      while (j < n && /[A-Za-z0-9_$]/.test(src[j])) j++;
      const word = src.slice(i, j);
      if (lang === 'http' && httpVerbs.has(word)) {
        out.push({ kind: 'keyword', value: word });
      } else if (keywords.has(word)) {
        out.push({ kind: 'keyword', value: word });
      } else {
        out.push({ kind: 'ident', value: word });
      }
      i = j;
      continue;
    }

    // Everything else
    let j = i;
    while (j < n && !/[A-Za-z0-9_$"'`#/\n]/.test(src[j])) j++;
    if (j === i) j = i + 1;
    out.push({ kind: 'punct', value: src.slice(i, j) });
    i = j;
  }

  return out;
}

const COLORS: Record<string, string> = {
  comment: '#8b8478',
  string: '#5c6300',
  number: '#671dfc',
  keyword: '#9c1a86',
  ident: '#1a1a1a',
  punct: '#444',
  text: '#1a1a1a',
};

export function CodeBlock({ language = 'text', children, filename }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  async function onCopy() {
    try {
      await navigator.clipboard.writeText(children);
      setCopied(true);
      setTimeout(() => setCopied(false), 1400);
    } catch {
      // clipboard unavailable in sandboxed iframes; fail silently
    }
  }

  const segments = tokenize(children, language);

  return (
    <Box
      sx={{
        my: 3,
        borderRadius: 2.5,
        bgcolor: '#fbf8f1',
        border: '1px solid rgba(17,17,17,0.10)',
        overflow: 'hidden',
        transition: `border-color ${tokens.motion.base} ${tokens.motion.curve}, box-shadow ${tokens.motion.base} ${tokens.motion.curve}`,
        '&:hover': {
          borderColor: tokens.color.border.accent,
          boxShadow: '0 6px 18px rgba(229,255,0,0.10)',
        },
      }}
    >
      <Stack
        direction="row"
        justifyContent="space-between"
        alignItems="center"
        sx={{
          px: 2,
          py: 1,
          borderBottom: '1px solid rgba(17,17,17,0.08)',
          bgcolor: 'rgba(17,17,17,0.03)',
        }}
      >
        <Typography
          sx={{
            fontFamily: tokens.font.mono,
            fontSize: 11,
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            color: '#77736b',
            fontWeight: 700,
          }}
        >
          {filename ? `${filename} · ${LANG_LABEL[language]}` : LANG_LABEL[language]}
        </Typography>
        <Tooltip title={copied ? 'Copied' : 'Copy'}>
          <IconButton
            size="small"
            onClick={onCopy}
            aria-label="Copy code"
            sx={{
              color: copied ? tokens.color.accent.success : '#77736b',
              '&:hover': { color: '#111', bgcolor: 'rgba(229,255,0,0.18)' },
            }}
          >
            {copied ? <CheckIcon sx={{ fontSize: 16 }} /> : <ContentCopyIcon sx={{ fontSize: 16 }} />}
          </IconButton>
        </Tooltip>
      </Stack>
      <Box
        component="pre"
        sx={{
          m: 0,
          p: 2.5,
          fontFamily: tokens.font.mono,
          fontSize: 13.5,
          lineHeight: 1.65,
          overflowX: 'auto',
          whiteSpace: 'pre',
          color: '#1a1a1a',
          textAlign: 'left',
          direction: 'ltr',
        }}
      >
        <code>
          {segments.map((s, idx) => (
            <span key={idx} style={{ color: COLORS[s.kind] ?? '#1a1a1a' }}>
              {s.value}
            </span>
          ))}
        </code>
      </Box>
    </Box>
  );
}

export default CodeBlock;
