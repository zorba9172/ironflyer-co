import { useState } from 'react';
import { Box, Button, Chip, Collapse, Stack, Tooltip, Typography } from '@mui/material';
import { VscFiles, VscShield, VscSymbolMethod } from 'react-icons/vsc';
import type { Patch } from '../studioData';
import { text } from '@ironflyer/design-tokens/brand';

// H4 — reviewable patch diff. The differentiator is "patches that can be
// reviewed," not just applied. This is the review surface the operator reads
// BEFORE the patch goes through the real applyPatch mutation: an expandable,
// themed hunk view co-located with the apply action.
//
// The live `applyPatch`/gate `patches` payload currently exposes only
// { id, title, state, lines } — there is no hunk/diff content on the wire yet
// (FLAGGED in the report). So we render the reviewable summary we DO have
// (file/title + line delta) and, when a patch carries diff text, the themed
// hunk view below. The moment the patch query gains a `diff`/`hunks` field this
// component renders it with zero call-site changes.

// A single unified-diff hunk, already split into lines. Optional on Patch until
// the backend query exposes diff content.
export interface DiffHunk {
  /** the @@ header, e.g. "@@ -14,6 +14,9 @@ func handler()" */
  header?: string;
  /** the file this hunk belongs to */
  path?: string;
  lines: { kind: 'add' | 'del' | 'ctx'; text: string }[];
}

// Patch may optionally carry parsed hunks once the query exposes them.
type ReviewablePatch = Patch & { hunks?: DiffHunk[]; files?: string[] };

function HunkLine({ kind, text: line }: { kind: 'add' | 'del' | 'ctx'; text: string }) {
  return (
    <Box
      sx={(t) => ({
        display: 'flex',
        fontFamily: t.brand.font.mono,
        fontSize: text.s70,
        lineHeight: 1.55,
        px: 1,
        color: kind === 'add' ? 'success.main' : kind === 'del' ? 'error.main' : 'text.secondary',
        bgcolor:
          kind === 'add'
            ? `${t.palette.success.main}14`
            : kind === 'del'
              ? `${t.palette.error.main}14`
              : 'transparent',
      })}
    >
      <Box component="span" sx={{ width: 14, flexShrink: 0, color: 'text.disabled', userSelect: 'none' }}>
        {kind === 'add' ? '+' : kind === 'del' ? '-' : ' '}
      </Box>
      <Box component="span" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', minWidth: 0 }}>
        {line}
      </Box>
    </Box>
  );
}

// Reviewable diff for one patch. Expands inline above the apply action. `busy`
// disables the action while an apply is in flight; `onApply` runs the real
// applyPatch mutation (owned by the caller).
export function PatchDiff({
  patch,
  busy,
  onApply,
}: {
  patch: ReviewablePatch;
  busy: boolean;
  onApply: () => void;
}) {
  const [open, setOpen] = useState(false);
  const hunks = patch.hunks ?? [];
  const hasDiff = hunks.length > 0;
  const proposed = patch.state === 'proposed';
  const fileCount = patch.files?.length ?? [...new Set(hunks.map((h) => h.path).filter(Boolean))].length;
  const reviewSurfaceCount = hunks.length || 1;

  return (
    <Box sx={{ p: 1.5, border: 1, borderColor: 'divider', borderRadius: 2 }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: proposed ? 1 : 0 }}>
        <Box
          component="span"
          sx={(t) => ({
            height: 18,
            px: 0.75,
            display: 'inline-flex',
            alignItems: 'center',
            borderRadius: 1,
            fontFamily: t.brand.font.mono,
            fontSize: text.s62,
            textTransform: 'uppercase',
            letterSpacing: '0.04em',
            color: patch.state === 'applied' ? 'success.main' : 'text.secondary',
            bgcolor: patch.state === 'applied' ? `${t.palette.success.main}22` : 'action.hover',
          })}
        >
          {patch.state}
        </Box>
        <Typography sx={{ fontSize: text.s86, flex: 1, minWidth: 0 }} noWrap>
          {patch.title}
        </Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s70, color: 'success.main' })}>
          +{patch.lines}
        </Typography>
      </Stack>

      {patch.files && patch.files.length > 0 && (
        <Stack spacing={0.25} sx={{ mb: 1 }}>
          {patch.files.map((f) => (
            <Typography
              key={f}
              sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.disabled' })}
              noWrap
            >
              {f}
            </Typography>
          ))}
        </Stack>
      )}

      <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5, mb: proposed || open ? 1 : 0 }}>
        {fileCount > 0 && (
          <Tooltip title={patch.files?.join('\n') ?? 'Files inferred from diff hunks'} arrow>
            <Chip size="small" icon={<VscFiles size={11} />} label={`${fileCount} file${fileCount === 1 ? '' : 's'}`} sx={(t) => ({ height: 19, fontFamily: t.brand.font.mono, fontSize: text.s60, bgcolor: 'action.hover', color: 'text.secondary', '& .MuiChip-icon': { ml: 0.5 } })} />
          </Tooltip>
        )}
        <Chip size="small" icon={<VscSymbolMethod size={11} />} label={`${reviewSurfaceCount} review surface${reviewSurfaceCount === 1 ? '' : 's'}`} sx={(t) => ({ height: 19, fontFamily: t.brand.font.mono, fontSize: text.s60, bgcolor: 'action.hover', color: 'text.secondary', '& .MuiChip-icon': { ml: 0.5 } })} />
        <Tooltip title="Apply runs through the patch lifecycle and gate checks server-side." arrow>
          <Chip size="small" icon={<VscShield size={11} />} label="gate checked" sx={(t) => ({ height: 19, fontFamily: t.brand.font.mono, fontSize: text.s60, bgcolor: `${t.palette.warning.main}16`, color: 'text.secondary', '& .MuiChip-icon': { ml: 0.5, color: 'warning.main' } })} />
        </Tooltip>
      </Stack>

      <Collapse in={open} unmountOnExit>
        <Box sx={{ mb: 1, borderRadius: 1.5, border: 1, borderColor: 'divider', overflow: 'hidden', bgcolor: 'action.hover' }}>
          {hasDiff ? (
            hunks.map((h, hi) => (
              <Box key={hi} sx={{ '&:not(:last-of-type)': { borderBottom: 1, borderColor: 'divider' } }}>
                {(h.path || h.header) && (
                  <Box sx={(t) => ({ px: 1, py: 0.5, bgcolor: `${t.palette.text.primary}0a` })}>
                    {h.path && (
                      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.secondary' })} noWrap>
                        {h.path}
                      </Typography>
                    )}
                    {h.header && (
                      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s64, color: 'text.disabled' })} noWrap>
                        {h.header}
                      </Typography>
                    )}
                  </Box>
                )}
                {h.lines.map((ln, li) => (
                  <HunkLine key={li} kind={ln.kind} text={ln.text} />
                ))}
              </Box>
            ))
          ) : (
            <Box sx={{ p: 1.5 }}>
              <Typography sx={{ fontSize: text.s78, color: 'text.secondary', mb: 0.5 }}>
                Diff content is not yet on the patch payload.
              </Typography>
              <Typography sx={{ fontSize: text.s74, color: 'text.disabled' }}>
                This patch reports a {patch.lines}-line change
                {patch.files && patch.files.length > 0 ? ` across ${patch.files.length} file(s)` : ''}. Review the
                summary above, then apply — the patch lifecycle re-runs the gate server-side.
              </Typography>
            </Box>
          )}
        </Box>
      </Collapse>

      {proposed && (
        <Stack direction="row" spacing={1} alignItems="center">
          <Button
            size="small"
            variant="text"
            onClick={() => setOpen((v) => !v)}
            sx={{ color: 'text.secondary' }}
          >
            {open ? 'Hide diff' : hasDiff ? 'Review diff' : 'Review patch'}
          </Button>
          <Button size="small" variant="contained" disabled={busy} onClick={onApply}>
            Apply patch
          </Button>
        </Stack>
      )}
    </Box>
  );
}
