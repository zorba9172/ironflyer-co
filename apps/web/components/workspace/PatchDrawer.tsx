'use client';

// PatchDrawer — slide-in panel that renders a single patch: title, summary,
// change list, validation issues, and an Apply CTA when the patch is in a
// non-terminal state. Pure presentation; the parent owns the action call.

import { Box, Button, Chip, Drawer, IconButton, Stack, Typography } from '@mui/material';
import { Close, PlayArrow } from '@mui/icons-material';
import { Patch } from '../../lib/api/patches';
import { tokens } from '../../lib/theme';

interface Props {
  open: boolean;
  patch: Patch | null;
  applying: boolean;
  onClose: () => void;
  onApply: () => void;
}

export function PatchDrawer({ open, patch, applying, onClose, onApply }: Props) {
  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      PaperProps={{
        sx: {
          width: { xs: '100%', sm: 460 },
          bgcolor: tokens.color.bg.surface,
          color: tokens.color.text.primary,
          borderLeft: '1px solid rgba(17,17,17,0.12)',
        },
      }}
    >
      <Box sx={{ p: 2, display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="overline" color="text.secondary">Patch</Typography>
            <Typography variant="h6" sx={{ fontWeight: 800 }} noWrap>
              {patch?.title || (patch ? `Patch ${patch.id.slice(-6)}` : '')}
            </Typography>
          </Box>
          <IconButton onClick={onClose}><Close /></IconButton>
        </Stack>

        {patch && (
          <>
            <Stack direction="row" spacing={1} sx={{ mt: 1.2, flexWrap: 'wrap' }}>
              <Chip label={patch.status} size="small" sx={{
                bgcolor: tokens.color.bg.inset, color: tokens.color.text.primary,
                fontWeight: 800, textTransform: 'uppercase',
              }} />
              <Chip label={patch.author || 'agent'} size="small" sx={{ bgcolor: tokens.color.bg.inset }} />
              <Chip
                label={`${patch.changes?.length ?? 0} files`}
                size="small"
                sx={{ bgcolor: tokens.color.bg.inset }}
              />
            </Stack>

            {patch.summary && (
              <Typography variant="body2" sx={{ mt: 1.4, color: tokens.color.text.secondary }}>
                {patch.summary}
              </Typography>
            )}

            <Typography variant="overline" sx={{ mt: 2, color: 'text.secondary' }}>שינויים</Typography>
            <Stack spacing={0.6} sx={{ mt: 0.8, flex: 1, minHeight: 0, overflowY: 'auto', pr: 0.4 }}>
              {(patch.changes ?? []).map((c, i) => (
                <Box key={`${c.path}-${i}`} sx={{
                  px: 1.2, py: 0.9, borderRadius: 1.2,
                  bgcolor: tokens.color.bg.inset,
                  border: '1px solid rgba(17,17,17,0.08)',
                }}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Chip
                      label={c.op}
                      size="small"
                      sx={{
                        height: 18, fontSize: 10, textTransform: 'uppercase', fontWeight: 800,
                        bgcolor: opColour(c.op) + '22',
                        color: opColour(c.op),
                        border: `1px solid ${opColour(c.op)}55`,
                        '& .MuiChip-label': { px: 0.9 },
                      }}
                    />
                    <Typography
                      variant="caption"
                      sx={{ fontFamily: tokens.font.mono, flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                      title={c.path}
                    >
                      {c.path}
                    </Typography>
                  </Stack>
                  {c.content && c.op !== 'delete' && (
                    <Box component="pre" sx={{
                      mt: 0.8, mb: 0, p: 1, borderRadius: 0.8,
                      bgcolor: '#0d0e0f', color: '#d7d4cc',
                      fontFamily: tokens.font.mono, fontSize: 11.5,
                      maxHeight: 220, overflow: 'auto',
                      whiteSpace: 'pre',
                    }}>
                      {(c.content.length > 4000 ? c.content.slice(0, 4000) + '\n…truncated' : c.content)}
                    </Box>
                  )}
                </Box>
              ))}
            </Stack>

            {patch.issues && patch.issues.length > 0 && (
              <Box sx={{ mt: 1.4 }}>
                <Typography variant="overline" color="text.secondary">בעיות תיקוף</Typography>
                <Stack spacing={0.5} sx={{ mt: 0.6 }}>
                  {patch.issues.map((iss, i) => (
                    <Typography key={i} variant="caption" sx={{
                      px: 1, py: 0.5, borderRadius: 1,
                      bgcolor: 'rgba(229,79,79,0.06)',
                      color: tokens.color.accent.danger,
                      display: 'block',
                    }}>
                      {iss.gate}: {iss.message}
                    </Typography>
                  ))}
                </Stack>
              </Box>
            )}

            <Button
              fullWidth
              variant="contained"
              startIcon={<PlayArrow />}
              disabled={applying || patch.status === 'applied' || patch.status === 'rejected'}
              onClick={onApply}
              sx={{ mt: 2, borderRadius: '10px', minHeight: 44 }}
            >
              {patch.status === 'applied' ? 'Already applied' : applying ? 'מַחיל…' : 'Apply patch'}
            </Button>
          </>
        )}
      </Box>
    </Drawer>
  );
}

function opColour(op: string): string {
  switch (op) {
    case 'create': return tokens.color.accent.success;
    case 'update': return tokens.color.accent.lime;
    case 'delete': return tokens.color.accent.danger;
    default:       return tokens.color.text.muted;
  }
}
