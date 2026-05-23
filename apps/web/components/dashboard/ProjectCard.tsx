'use client';

import Link from 'next/link';
import { Box, Stack, Typography } from '@mui/material';
import { PlayArrow, OpenInNew } from '@mui/icons-material';
import { Project } from '../../lib/api';
import { tokens } from '../../lib/theme';
import { StatusPill, statusKindFromGate } from './StatusPill';

function lastActivity(project: Project) {
  const events = Array.isArray(project.events) ? project.events : [];
  if (events.length === 0) return undefined;
  return events.reduce((latest, ev) => (Date.parse(ev.createdAt) > Date.parse(latest.createdAt) ? ev : latest));
}

function formatDate(iso: string) {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return iso;
  return new Date(t).toLocaleDateString('en-US', { day: '2-digit', month: 'short' });
}

export function ProjectGridCard({ project }: { project: Project }) {
  const last = lastActivity(project);
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        minHeight: 196,
        border: '1px solid rgba(17,17,17,0.12)',
        borderRadius: '8px',
        bgcolor: '#f8f4ec',
        color: tokens.color.text.inverse,
        overflow: 'hidden',
        transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, border-color ${tokens.motion.base} ${tokens.motion.curve}, background-color ${tokens.motion.base} ${tokens.motion.curve}`,
        '&:hover': { borderColor: 'rgba(17,17,17,0.28)', bgcolor: '#fffaf1', transform: 'translateY(-2px)' },
      }}
    >
      <Box component={Link} href={`/projects/${project.id}`} sx={{ p: 2.2, color: 'inherit', textDecoration: 'none', flex: 1, display: 'flex', flexDirection: 'column' }}>
        <Stack direction="row" justifyContent="space-between" spacing={1}>
          <Typography variant="subtitle1" sx={{ fontWeight: 900, minWidth: 0 }} noWrap>{project.name}</Typography>
          <StatusPill kind={statusKindFromGate(project.status)} label={project.status || 'idle'} />
        </Stack>
        <Typography
          variant="body2"
          sx={{
            mt: 0.9,
            color: '#686158',
            display: '-webkit-box',
            WebkitLineClamp: 3,
            WebkitBoxOrient: 'vertical',
            overflow: 'hidden',
            minHeight: 56,
          }}
        >
          {project.description || project.spec?.idea || 'A description will appear after the first run.'}
        </Typography>
        <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: 'auto', pt: 1.4 }}>
          <Typography variant="caption" sx={{ color: '#86807a' }} noWrap>
            {last ? last.message : 'Not run yet'}
          </Typography>
          <Typography variant="caption" sx={{ color: '#86807a', fontFamily: tokens.font.mono }}>
            {formatDate(project.updatedAt)}
          </Typography>
        </Stack>
      </Box>
      <Stack direction="row" sx={{ borderTop: '1px solid rgba(17,17,17,0.08)' }}>
        <Box component={Link} href={`/projects/${project.id}`} sx={cardActionSx}>
          <OpenInNew fontSize="small" />
          <Typography variant="caption" sx={cardActionLabelSx}>Open</Typography>
        </Box>
        <Box component={Link} href={`/projects/${project.id}?action=run`} sx={{ ...cardActionSx, borderLeft: '1px solid rgba(17,17,17,0.08)', color: tokens.color.text.inverse }}>
          <PlayArrow fontSize="small" />
          <Typography variant="caption" sx={cardActionLabelSx}>Run</Typography>
        </Box>
      </Stack>
    </Box>
  );
}

const cardActionSx = {
  flex: 1,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: 0.7,
  py: 1.1,
  color: tokens.color.text.inverse,
  textDecoration: 'none',
  transition: 'background-color 160ms',
  '&:hover': { bgcolor: 'rgba(229,255,0,0.18)' },
};

const cardActionLabelSx = { fontWeight: 800, letterSpacing: 0.2 };
