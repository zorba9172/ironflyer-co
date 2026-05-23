'use client';

import Link from 'next/link';
import { Box, Stack, Typography } from '@mui/material';
import { AutoAwesome, BoltOutlined, CheckCircleOutline, ErrorOutline, HourglassEmpty, RocketLaunch } from '@mui/icons-material';
import { ExecutionEvent, Project } from '../../lib/api';
import { tokens } from '../../lib/theme';
import { StatusPill, statusKindFromGate } from './StatusPill';

export interface ActivityRow {
  id: string;
  projectId: string;
  projectName: string;
  message: string;
  status: string;
  agent?: string;
  step: string;
  createdAt: string;
}

export function flattenActivity(projects: Project[], limit = 10): ActivityRow[] {
  const rows: ActivityRow[] = [];
  for (const project of projects) {
    const events: ExecutionEvent[] = Array.isArray(project.events) ? project.events : [];
    for (const ev of events) {
      rows.push({
        id: `${project.id}:${ev.id}`,
        projectId: project.id,
        projectName: project.name,
        message: ev.message,
        status: ev.status,
        agent: ev.agent,
        step: ev.step,
        createdAt: ev.createdAt,
      });
    }
  }
  rows.sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
  return rows.slice(0, limit);
}

function iconFor(step: string, status: string) {
  const s = (status ?? '').toLowerCase();
  if (s === 'failed' || s === 'error') return <ErrorOutline fontSize="small" />;
  if (s === 'passed' || s === 'success' || s === 'completed') return <CheckCircleOutline fontSize="small" />;
  if (s === 'running' || s === 'pending') return <HourglassEmpty fontSize="small" />;
  const lower = (step ?? '').toLowerCase();
  if (lower.includes('deploy')) return <RocketLaunch fontSize="small" />;
  if (lower.includes('patch')) return <BoltOutlined fontSize="small" />;
  return <AutoAwesome fontSize="small" />;
}

function relative(time: string) {
  const t = Date.parse(time);
  if (Number.isNaN(t)) return time;
  const diff = Date.now() - t;
  const min = Math.round(diff / 60_000);
  if (min < 1) return 'Just now';
  if (min < 60) return `${min}m ago`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const days = Math.round(hr / 24);
  if (days < 7) return `${days}d ago`;
  return new Date(t).toLocaleDateString('en-US');
}

export function ActivityTimeline({ rows, emptyHint }: { rows: ActivityRow[]; emptyHint?: string }) {
  if (rows.length === 0) {
    return (
      <Box sx={{ py: 3, px: 1.6, textAlign: 'center' }}>
        <Typography variant="body2" sx={{ color: '#686158' }}>
          {emptyHint ?? 'No project activity yet'}
        </Typography>
      </Box>
    );
  }
  return (
    <Stack spacing={0} sx={{ position: 'relative' }}>
      {rows.map((row, i) => (
        <Stack
          key={row.id}
          component={Link}
          href={`/projects/${row.projectId}`}
          direction="row"
          spacing={1.4}
          alignItems="flex-start"
          sx={{
            position: 'relative',
            px: 1.4,
            py: 1.2,
            color: 'inherit',
            textDecoration: 'none',
            borderTop: i === 0 ? 'none' : '1px solid rgba(17,17,17,0.06)',
            transition: `background-color ${tokens.motion.fast} ${tokens.motion.curve}`,
            '&:hover': { bgcolor: 'rgba(229,255,0,0.10)' },
          }}
        >
          <Box
            sx={{
              flex: '0 0 auto',
              mt: 0.25,
              width: 30,
              height: 30,
              borderRadius: '8px',
              display: 'grid',
              placeItems: 'center',
              bgcolor: '#fffaf1',
              color: tokens.color.text.inverse,
              border: '1px solid rgba(17,17,17,0.12)',
            }}
          >
            {iconFor(row.step, row.status)}
          </Box>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Stack direction="row" spacing={1} alignItems="center" sx={{ minWidth: 0 }}>
              <Typography variant="subtitle2" noWrap sx={{ flex: '0 1 auto', fontWeight: 800 }}>
                {row.projectName}
              </Typography>
              <Typography variant="caption" sx={{ color: '#86807a', flex: '0 0 auto' }}>·</Typography>
              <Typography variant="caption" sx={{ color: '#86807a', flex: '0 0 auto' }} noWrap>
                {row.agent || row.step}
              </Typography>
            </Stack>
            <Typography variant="body2" sx={{
              color: '#4a453e',
              mt: 0.3,
              display: '-webkit-box',
              WebkitLineClamp: 2,
              WebkitBoxOrient: 'vertical',
              overflow: 'hidden',
            }}>
              {row.message}
            </Typography>
          </Box>
          <Stack alignItems="flex-end" spacing={0.6} sx={{ flex: '0 0 auto' }}>
            <StatusPill kind={statusKindFromGate(row.status)} label={row.status || row.step} />
            <Typography variant="caption" sx={{ color: '#86807a' }}>{relative(row.createdAt)}</Typography>
          </Stack>
        </Stack>
      ))}
    </Stack>
  );
}
