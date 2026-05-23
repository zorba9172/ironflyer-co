'use client';

// DiffViewer — renders a single FileChange as side-by-side coloured diff.
// Pure presentation: the parent passes the change and (optionally) the
// previous content of the file. Logic is local to keep the bundle small;
// we deliberately avoid a heavy diff library since:
//
//   1. create/delete have trivial visualisations (all + or all -)
//   2. replace / insert_after carry their own anchor + replacement, which
//      makes the diff a single contiguous hunk we can render directly
//   3. update is the only case that needs a real text diff — we use a
//      compact LCS line-diff that's adequate for human review and runs
//      in O(n*m) over the file (acceptable for files under a few thousand
//      lines, which is the patch.Engine cap anyway).
//
// Output: a two-column grid of line numbers + content where removed lines
// are red, added lines are green, and unchanged lines are muted.

import { Box, Stack, Typography } from '@mui/material';
import { tokens } from '../../lib/theme';
import { FileChange } from '../../lib/api/patches';
import { VirtualList } from '../performance/VirtualList';

interface Props {
  change: FileChange;
  previousContent?: string;
  // Hard cap on lines rendered — patches against very large files become
  // illegible past a few hundred lines, so we truncate with a marker.
  maxLines?: number;
}

type DiffLine = { kind: 'context' | 'add' | 'remove'; left?: string; right?: string; ln?: number; rn?: number };
const LCS_CELL_LIMIT = 240_000;

export function DiffViewer({ change, previousContent, maxLines = 600 }: Props) {
  let lines: DiffLine[] = [];
  let estimatedLineCount: number | null = null;
  let addCount: number | null = null;
  let removeCount: number | null = null;

  switch (change.op) {
    case 'create':
      lines = (change.content ?? '').split('\n').map((s, i) => ({
        kind: 'add', right: s, rn: i + 1,
      }));
      break;
    case 'delete':
      lines = (previousContent ?? '').split('\n').map((s, i) => ({
        kind: 'remove', left: s, ln: i + 1,
      }));
      break;
    case 'update': {
      const prev = (previousContent ?? '').split('\n');
      const next = (change.content ?? '').split('\n');
      addCount = next.length;
      removeCount = prev.length;
      if (prev.length * next.length > LCS_CELL_LIMIT) {
        estimatedLineCount = prev.length + next.length;
        lines = coarseUpdateDiff(prev, next, maxLines);
      } else {
        lines = lcsLineDiff(prev, next);
      }
      break;
    }
    case 'replace': {
      const anchor = (change.anchor ?? '').split('\n');
      const repl = (change.replacement ?? '').split('\n');
      lines = [
        ...anchor.map((s, i) => ({ kind: 'remove' as const, left: s, ln: i + 1 })),
        ...repl.map((s, i) => ({ kind: 'add' as const, right: s, rn: i + 1 })),
      ];
      break;
    }
    case 'insert_after': {
      const anchor = (change.anchor ?? '').split('\n');
      const repl = (change.replacement ?? '').split('\n');
      lines = [
        ...anchor.map((s, i) => ({ kind: 'context' as const, left: s, right: s, ln: i + 1, rn: i + 1 })),
        ...repl.map((s, i) => ({ kind: 'add' as const, right: s, rn: anchor.length + i + 1 })),
      ];
      break;
    }
  }

  const truncated = lines.length > maxLines || (estimatedLineCount !== null && estimatedLineCount > lines.length);
  const visible = truncated ? lines.slice(0, maxLines) : lines;

  let adds = addCount ?? 0;
  let removes = removeCount ?? 0;
  if (addCount === null || removeCount === null) {
    for (const l of lines) {
      if (l.kind === 'add') adds += 1;
      if (l.kind === 'remove') removes += 1;
    }
  }

  return (
    <Box sx={{
      mt: 0.8,
      borderRadius: 1,
      border: '1px solid rgba(17,17,17,0.10)',
      bgcolor: '#0d0e0f',
      fontFamily: tokens.font.mono,
      fontSize: 11.5,
      overflow: 'hidden',
    }}>
      <Stack direction="row" spacing={1} sx={{ px: 1, py: 0.6, borderBottom: '1px solid #1a1c1e' }}>
        <Typography variant="caption" sx={{ color: tokens.color.accent.success, fontFamily: tokens.font.mono }}>
          +{adds}
        </Typography>
        <Typography variant="caption" sx={{ color: tokens.color.accent.danger, fontFamily: tokens.font.mono }}>
          −{removes}
        </Typography>
        {truncated && (
          <Typography variant="caption" sx={{ color: '#928e83', ml: 'auto' }}>
            showing first {maxLines} of {estimatedLineCount ?? lines.length}
          </Typography>
        )}
      </Stack>

      <VirtualList
        items={visible}
        itemHeight={24}
        height={Math.min(360, Math.max(48, visible.length * 24))}
        keyExtractor={(_, index) => index}
        ariaLabel={`Diff for ${change.path}`}
        itemOverflow="visible"
        sx={{ overflowX: 'auto' }}
        renderItem={(line) => <DiffRow line={line} />}
      />
    </Box>
  );
}

function DiffRow({ line }: { line: DiffLine }) {
  const leftBg = line.kind === 'remove' ? 'rgba(229,79,79,0.16)' : 'transparent';
  const rightBg = line.kind === 'add' ? 'rgba(106,209,118,0.18)' : 'transparent';
  return (
    <Box sx={{
      display: 'grid',
      gridTemplateColumns: '36px minmax(0, 1fr) 36px minmax(0, 1fr)',
      minWidth: 960,
      minHeight: 24,
    }}>
      <Box sx={{
        px: 0.6, color: '#5a5750', textAlign: 'right',
        bgcolor: leftBg, userSelect: 'none', borderRight: '1px solid #18191a',
      }}>
        {line.ln ?? ''}
      </Box>
      <Box sx={{
        px: 0.8, color: line.kind === 'remove' ? '#ffb3b3' : '#d7d4cc',
        bgcolor: leftBg, whiteSpace: 'pre', overflow: 'hidden', textOverflow: 'ellipsis',
      }}>
        {line.kind === 'remove' ? '−' : ' '} {line.left ?? ''}
      </Box>
      <Box sx={{
        px: 0.6, color: '#5a5750', textAlign: 'right',
        bgcolor: rightBg, userSelect: 'none', borderRight: '1px solid #18191a',
      }}>
        {line.rn ?? ''}
      </Box>
      <Box sx={{
        px: 0.8, color: line.kind === 'add' ? '#bef0c4' : '#d7d4cc',
        bgcolor: rightBg, whiteSpace: 'pre', overflow: 'hidden', textOverflow: 'ellipsis',
      }}>
        {line.kind === 'add' ? '+' : ' '} {line.right ?? ''}
      </Box>
    </Box>
  );
}

// lcsLineDiff returns a sequence of DiffLines describing how to turn `a`
// (previous) into `b` (next). It's a textbook LCS DP that runs in O(n*m)
// time and O(n*m) memory — fine for files up to a few thousand lines.
// We could swap in a Myers diff later if patches against bigger files
// become common; the public DiffViewer signature wouldn't change.
function coarseUpdateDiff(a: string[], b: string[], maxLines: number): DiffLine[] {
  const removeBudget = Math.ceil(maxLines / 2);
  const addBudget = Math.max(1, maxLines - removeBudget);
  return [
    ...a.slice(0, removeBudget).map((s, i) => ({ kind: 'remove' as const, left: s, ln: i + 1 })),
    ...b.slice(0, addBudget).map((s, i) => ({ kind: 'add' as const, right: s, rn: i + 1 })),
  ];
}

function lcsLineDiff(a: string[], b: string[]): DiffLine[] {
  const n = a.length;
  const m = b.length;
  if (n === 0) return b.map((s, i) => ({ kind: 'add', right: s, rn: i + 1 }));
  if (m === 0) return a.map((s, i) => ({ kind: 'remove', left: s, ln: i + 1 }));

  // dp[i][j] = LCS length of a[0..i) and b[0..j)
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
  for (let i = 0; i < n; i++) {
    for (let j = 0; j < m; j++) {
      if (a[i] === b[j]) {
        dp[i + 1][j + 1] = dp[i][j] + 1;
      } else {
        dp[i + 1][j + 1] = Math.max(dp[i + 1][j], dp[i][j + 1]);
      }
    }
  }

  // Backtrack to build the diff.
  const out: DiffLine[] = [];
  let i = n;
  let j = m;
  while (i > 0 && j > 0) {
    if (a[i - 1] === b[j - 1]) {
      out.push({ kind: 'context', left: a[i - 1], right: b[j - 1], ln: i, rn: j });
      i -= 1;
      j -= 1;
    } else if (dp[i - 1][j] >= dp[i][j - 1]) {
      out.push({ kind: 'remove', left: a[i - 1], ln: i });
      i -= 1;
    } else {
      out.push({ kind: 'add', right: b[j - 1], rn: j });
      j -= 1;
    }
  }
  while (i > 0) {
    out.push({ kind: 'remove', left: a[i - 1], ln: i });
    i -= 1;
  }
  while (j > 0) {
    out.push({ kind: 'add', right: b[j - 1], rn: j });
    j -= 1;
  }
  out.reverse();
  return out;
}
