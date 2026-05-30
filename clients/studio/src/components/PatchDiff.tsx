import { useState } from 'react';
import { Box, Button, Chip, Collapse, Stack, Tooltip, Typography } from '@mui/material';
import { VscFiles, VscShield, VscSymbolMethod, VscGitCommit, VscCheck, VscCircleFilled } from 'react-icons/vsc';
import type { Patch } from '../studioData';
import { text } from '@ironflyer/design-tokens/brand';
import { studioTokens } from '../theme';

// ── Reviewable patch diff ──────────────────────────────────────────────────────
// The differentiator is "patches that can be reviewed," not just applied. This is
// the review surface the operator reads BEFORE the patch goes through the real
// applyPatch mutation: an expandable, themed hunk view co-located with the apply
// action.
//
// The live `applyPatch`/gate `patches` payload currently exposes only
// { id, title, state, lines } — there is no hunk/diff content on the wire yet
// (FLAGGED in the report). So we render the reviewable summary we DO have
// (file/title + line delta) and, when a patch carries diff text, the themed
// hunk view below. The moment the patch query gains a `diff`/`hunks` field this
// component renders it with zero call-site changes.

// ── Types ──────────────────────────────────────────────────────────────────────
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

// ── Hunk line ──────────────────────────────────────────────────────────────────
function HunkLine({ kind, text: line }: { kind: 'add' | 'del' | 'ctx'; text: string }) {
  return (
    <Box
      sx={(t) => ({
        display: 'flex',
        fontFamily: t.brand.font.mono,
        fontSize: text.s68,
        lineHeight: 1.55,
        px: 1.25,
        py: 0.05,
        color: kind === 'add' ? 'success.main' : kind === 'del' ? 'error.main' : 'text.secondary',
        bgcolor:
          kind === 'add'
            ? `${t.palette.success.main}12`
            : kind === 'del'
              ? `${t.palette.error.main}12`
              : 'transparent',
      })}
    >
      <Box component="span" sx={{ width: 16, flexShrink: 0, color: 'text.disabled', userSelect: 'none', fontWeight: 600 }}>
        {kind === 'add' ? '+' : kind === 'del' ? '-' : ' '}
      </Box>
      <Box component="span" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', minWidth: 0 }}>
        {line}
      </Box>
    </Box>
  );
}

// ── State badge ────────────────────────────────────────────────────────────────
function StateBadge({ state }: { state: string }) {
  const isApplied = state === 'applied';
  return (
    <Stack direction="row" alignItems="center" spacing={0.5}
      sx={(t) => ({
        height: 19, px: 0.75,
        display: 'inline-flex', alignItems: 'center',
        borderRadius: `${t.studio.radius.sm / 2}px`,
        fontFamily: t.brand.font.mono,
        fontSize: text.s60,
        textTransform: 'uppercase',
        letterSpacing: '0.06em',
        color: isApplied ? 'success.main' : state === 'proposed' ? 'primary.main' : 'text.secondary',
        bgcolor: isApplied
          ? `${t.palette.success.main}1a`
          : state === 'proposed'
            ? `${t.palette.primary.main}1a`
            : 'action.hover',
        border: `1px solid ${isApplied ? `${t.palette.success.main}33` : state === 'proposed' ? `${t.palette.primary.main}33` : t.palette.divider}`,
      })}
    >
      {isApplied ? <VscCheck size={9} /> : state === 'proposed' ? <VscCircleFilled size={8} /> : <VscGitCommit size={9} />}
      <Box component="span">{state}</Box>
    </Stack>
  );
}

// ── Diff surface: single hunk block ───────────────────────────────────────────
function HunkBlock({ hunk, index, total: _total }: { hunk: DiffHunk; index: number; total: number }) {
  return (
    <Box sx={(t) => ({
      '&:not(:last-of-type)': { borderBottom: `1px solid ${t.palette.divider}` },
    })}>
      {(hunk.path || hunk.header) && (
        <Box sx={(t) => ({
          px: 1.25, py: 0.6,
          bgcolor: `${t.palette.text.primary}07`,
          borderBottom: `1px solid ${t.palette.divider}`,
        })}>
          {hunk.path && (
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.secondary', fontWeight: 500 })} noWrap>
              {hunk.path}
            </Typography>
          )}
          {hunk.header && (
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled', mt: hunk.path ? 0.25 : 0 })} noWrap>
              {hunk.header}
            </Typography>
          )}
        </Box>
      )}
      {hunk.lines.map((ln, li) => (
        <HunkLine key={`${index}-${li}`} kind={ln.kind} text={ln.text} />
      ))}
    </Box>
  );
}

// ── Empty diff placeholder ─────────────────────────────────────────────────────
function NoDiffPlaceholder({ patch }: { patch: ReviewablePatch }) {
  return (
    <Box sx={(t) => ({ p: 1.75, borderRadius: `0 0 ${t.studio.radius.sm}px ${t.studio.radius.sm}px` })}>
      <Typography sx={{ fontSize: text.s78, color: 'text.secondary', mb: 0.5, fontWeight: 500 }}>
        Diff content is not yet on the patch payload.
      </Typography>
      <Typography sx={{ fontSize: text.s74, color: 'text.disabled', lineHeight: 1.6 }}>
        This patch reports a {patch.lines}-line change
        {patch.files && patch.files.length > 0 ? ` across ${patch.files.length} file(s)` : ''}. Review the
        summary above, then apply — the patch lifecycle re-runs the gate server-side.
      </Typography>
    </Box>
  );
}

// ── Main component ─────────────────────────────────────────────────────────────
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

  // Line delta: addedLines - removedLines for a visual +/- summary
  const addedLines = hunks.reduce((n, h) => n + h.lines.filter((l) => l.kind === 'add').length, 0);
  const removedLines = hunks.reduce((n, h) => n + h.lines.filter((l) => l.kind === 'del').length, 0);

  return (
    <Box sx={(t) => ({
      border: `1px solid ${t.palette.divider}`,
      borderRadius: `${t.studio.radius.sm}px`,
      overflow: 'hidden',
      transition: `border-color ${t.studio.motion.fast}, box-shadow ${t.studio.motion.fast}`,
      ...(proposed && {
        borderColor: `${studioTokens.neon.violet}40`,
        boxShadow: `0 0 0 1px ${studioTokens.neon.violet}18, 0 4px 24px ${studioTokens.neon.violet}14`,
      }),
    })}>
      {/* Header row */}
      <Stack direction="row" alignItems="center" spacing={1}
        sx={(t) => ({
          px: 1.5, py: 1,
          bgcolor: 'background.paper',
          borderBottom: open || (proposed && !open) ? `1px solid ${t.palette.divider}` : 'none',
        })}
      >
        <StateBadge state={patch.state} />
        <Typography sx={{ fontSize: text.s86, flex: 1, minWidth: 0, fontWeight: 500 }} noWrap>
          {patch.title}
        </Typography>
        {/* Line delta indicators */}
        <Stack direction="row" spacing={0.75} alignItems="center">
          {hasDiff ? (
            <>
              {addedLines > 0 && (
                <Typography sx={{ fontFamily: (t) => t.brand.font.mono, fontSize: text.s70, color: 'success.main', fontWeight: 600 }}>
                  +{addedLines}
                </Typography>
              )}
              {removedLines > 0 && (
                <Typography sx={{ fontFamily: (t) => t.brand.font.mono, fontSize: text.s70, color: 'error.main', fontWeight: 600 }}>
                  -{removedLines}
                </Typography>
              )}
            </>
          ) : (
            <Typography sx={{ fontFamily: (t) => t.brand.font.mono, fontSize: text.s70, color: 'success.main', fontWeight: 600 }}>
              +{patch.lines}
            </Typography>
          )}
        </Stack>
      </Stack>

      {/* File list (when explicitly on the payload) */}
      {patch.files && patch.files.length > 0 && (
        <Box sx={(t) => ({ px: 1.5, py: 0.75, bgcolor: `${t.palette.text.primary}04`, borderBottom: `1px solid ${t.palette.divider}` })}>
          <Stack spacing={0.2}>
            {patch.files.map((f) => (
              <Typography key={f} sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, color: 'text.disabled' })} noWrap>
                {f}
              </Typography>
            ))}
          </Stack>
        </Box>
      )}

      {/* Chip strip: file count + review surfaces + gate badge */}
      <Stack direction="row" sx={(t) => ({
        flexWrap: 'wrap', gap: 0.5, px: 1.5, py: 0.75,
        bgcolor: 'background.paper',
        borderBottom: proposed || open ? `1px solid ${t.palette.divider}` : 'none',
      })}>
        {fileCount > 0 && (
          <Tooltip title={patch.files?.join('\n') ?? 'Files inferred from diff hunks'} arrow>
            <Chip size="small" icon={<VscFiles size={10} />} label={`${fileCount} file${fileCount === 1 ? '' : 's'}`}
              sx={(t) => ({ height: 18, fontFamily: t.brand.font.mono, fontSize: text.s58, bgcolor: 'action.hover', color: 'text.secondary', border: `1px solid ${t.palette.divider}`, '& .MuiChip-icon': { ml: 0.5 } })} />
          </Tooltip>
        )}
        <Chip size="small" icon={<VscSymbolMethod size={10} />} label={`${reviewSurfaceCount} review surface${reviewSurfaceCount === 1 ? '' : 's'}`}
          sx={(t) => ({ height: 18, fontFamily: t.brand.font.mono, fontSize: text.s58, bgcolor: 'action.hover', color: 'text.secondary', border: `1px solid ${t.palette.divider}`, '& .MuiChip-icon': { ml: 0.5 } })} />
        <Tooltip title="Apply runs through the patch lifecycle and gate checks server-side." arrow>
          <Chip size="small" icon={<VscShield size={10} />} label="gate checked"
            sx={(t) => ({ height: 18, fontFamily: t.brand.font.mono, fontSize: text.s58, bgcolor: `${t.palette.warning.main}14`, color: 'text.secondary', border: `1px solid ${t.palette.warning.main}33`, '& .MuiChip-icon': { ml: 0.5, color: 'warning.main' } })} />
        </Tooltip>
      </Stack>

      {/* Expandable diff surface */}
      <Collapse in={open} unmountOnExit>
        <Box sx={(t) => ({
          borderBottom: proposed ? `1px solid ${t.palette.divider}` : 'none',
          overflow: 'hidden',
          bgcolor: 'background.default',
        })}>
          {hasDiff ? (
            hunks.map((h, hi) => <HunkBlock key={hi} hunk={h} index={hi} total={hunks.length} />)
          ) : (
            <NoDiffPlaceholder patch={patch} />
          )}
        </Box>
      </Collapse>

      {/* Action row for proposed patches */}
      {proposed && (
        <Stack direction="row" spacing={1} alignItems="center"
          sx={{
            px: 1.5, py: 1,
            bgcolor: 'background.paper',
          }}
        >
          <Button
            size="small" variant="text"
            onClick={() => setOpen((v) => !v)}
            sx={(t) => ({
              color: 'text.secondary', fontSize: text.s78,
              '&:hover': { color: 'text.primary' },
              transition: `color ${t.studio.motion.fast}`,
            })}
          >
            {open ? 'Hide diff' : hasDiff ? 'Review diff' : 'Review patch'}
          </Button>
          <Box sx={{ flex: 1 }} />
          <Button
            size="small" variant="contained" color="primary"
            disabled={busy}
            onClick={onApply}
            sx={{ fontSize: text.s78, fontWeight: 600 }}
          >
            Apply patch
          </Button>
        </Stack>
      )}
    </Box>
  );
}
