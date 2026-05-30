import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { Box, Button, Chip, CircularProgress, IconButton, InputBase, Stack, Typography } from '@mui/material';
import { LuFolderPlus, LuRefreshCw, LuRocket, LuSearch, LuSparkles, LuTriangleAlert, LuX } from 'react-icons/lu';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useStudio } from '../store';
import { mockProject } from '../studioData';
import { AmbientBackdrop } from './home/AmbientBackdrop';
import { PortfolioStrip } from './projects/PortfolioStrip';
import { ProjectCard } from './projects/ProjectCard';
import { BUCKET_LABEL, BUCKET_ORDER, bucketColor, bucketFor, type StatusBucket } from './projects/projectStatus';

interface ApiProject { id: string; name: string; description?: string | null; status: string; idea?: string | null; updatedAt?: string | null }
interface ApiFile { path: string; content?: string | null }

type Filter = StatusBucket | 'all';

export function ProjectsPage() {
  const navigate = useNavigate();
  const request = useRequest();
  const qc = useQueryClient();
  const openLiveProject = useStudio((s) => s.openLiveProject);
  const [q, setQ] = useState('');
  const [filter, setFilter] = useState<Filter>('all');
  const [openingId, setOpeningId] = useState<string | null>(null);

  const { data: projects, isLoading, error } = useGraphQLQuery<ApiProject[], { projects: ApiProject[] }>({
    key: ['projects'],
    operationName: 'Projects', query: operations.PROJECTS,
    fallbackData: [], map: (r) => r.projects ?? [],
  });

  const counts = useMemo(() => {
    const c: Record<StatusBucket, number> = { building: 0, shipped: 0, blocked: 0 };
    for (const p of projects) c[bucketFor(p.status)] += 1;
    return c;
  }, [projects]);

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return projects.filter((p) => {
      if (filter !== 'all' && bucketFor(p.status) !== filter) return false;
      if (!needle) return true;
      return (
        p.name.toLowerCase().includes(needle) ||
        (p.description ?? '').toLowerCase().includes(needle) ||
        (p.idea ?? '').toLowerCase().includes(needle)
      );
    });
  }, [projects, q, filter]);

  const open = async (p: ApiProject) => {
    if (!request) { toast('Connect the orchestrator to open a project.', 'error'); return; }
    setOpeningId(p.id);
    try {
      const d = await request<{ projectFiles: ApiFile[] }>('ProjectFiles', operations.PROJECT_FILES, { id: p.id });
      const files = (d.projectFiles ?? []).filter((f) => typeof f.content === 'string').map((f) => ({ path: f.path, content: f.content as string }));
      openLiveProject({ ...mockProject, id: p.id, name: p.name }, p.id, files);
      navigate('/build');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Could not open project.', 'error');
    } finally {
      setOpeningId(null);
    }
  };

  const remove = async (p: ApiProject) => {
    const go = await confirmAction({ title: `Delete “${p.name}”?`, text: 'This removes the project and its saved files. This cannot be undone.', confirmText: 'Delete', danger: true });
    if (!go || !request) return;
    try {
      await request('DeleteProject', operations.DELETE_PROJECT, { id: p.id });
      void qc.invalidateQueries({ queryKey: ['projects'] });
      toast('Project deleted.', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Delete failed.', 'error');
    }
  };

  const hasProjects = projects.length > 0;

  return (
    <Box sx={{ position: 'relative', minHeight: '100%', isolation: 'isolate' }}>
      <AmbientBackdrop />

      <Box sx={{ position: 'relative', zIndex: 1, p: { xs: 3, md: 5 }, maxWidth: 1180, mx: 'auto' }}>
        {/* Header */}
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" sx={{ mb: 4, flexWrap: 'wrap', gap: 2 }}>
          <Stack spacing={1}>
            <Chip
              icon={<LuSparkles size={15} />}
              label="Your workspace"
              sx={(theme) => ({
                alignSelf: 'flex-start',
                height: 30,
                borderRadius: theme.studio.radius.pill,
                border: `1px solid ${theme.palette.divider}`,
                backgroundColor: theme.palette.cardBg,
                backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
                color: theme.palette.text.secondary,
                fontWeight: theme.typography.fontWeightMedium,
                '& .MuiChip-icon': { color: theme.studio.neon.blue, ml: 0.5 },
                '& .MuiChip-label': { px: 1 },
              })}
            />
            <Typography variant="h2" sx={{ fontWeight: 800, fontSize: { xs: '2rem', md: '2.6rem' } }}>
              Projects
            </Typography>
            <Typography color="text.secondary" sx={{ maxWidth: 560 }}>
              Every idea you have shipped, are building, or have parked — in one place.
            </Typography>
          </Stack>
          <Button variant="contained" color="primary" onClick={() => navigate('/')} startIcon={<LuFolderPlus size={18} />}>
            Create project
          </Button>
        </Stack>

        {/* Portfolio mirror */}
        {hasProjects && (
          <Box sx={{ mb: 3 }}>
            <PortfolioStrip counts={counts} total={projects.length} />
          </Box>
        )}

        {/* Search + filter row */}
        {hasProjects && (
          <Stack
            direction={{ xs: 'column', md: 'row' }}
            alignItems={{ xs: 'stretch', md: 'center' }}
            justifyContent="space-between"
            spacing={2}
            sx={{ mb: 3 }}
          >
            <Box
              sx={(theme) => ({
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                width: { xs: '100%', md: 380 },
                px: 2,
                py: 1,
                borderRadius: `${theme.studio.radius.sm}px`,
                border: `1px solid ${theme.palette.divider}`,
                backgroundColor: theme.palette.cardBg,
                backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
                transition: `border-color ${theme.studio.motion.fast}`,
                '&:focus-within': { borderColor: theme.studio.neon.blue },
              })}
            >
              <Box component="span" sx={(theme) => ({ display: 'inline-flex', color: theme.palette.text.disabled })}>
                <LuSearch size={17} />
              </Box>
              <InputBase
                fullWidth
                placeholder="Search projects"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                inputProps={{ 'aria-label': 'Search projects' }}
                sx={{ typography: 'body2' }}
              />
              {q && (
                <IconButton size="small" aria-label="Clear search" onClick={() => setQ('')} sx={(theme) => ({ color: theme.palette.text.disabled })}>
                  <LuX size={15} />
                </IconButton>
              )}
            </Box>

            <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap' }} useFlexGap>
              {(['all', ...BUCKET_ORDER] as Filter[]).map((f) => {
                const active = filter === f;
                const label = f === 'all' ? `All ${projects.length}` : `${BUCKET_LABEL[f]} ${counts[f]}`;
                return (
                  <Chip
                    key={f}
                    label={label}
                    onClick={() => setFilter(f)}
                    aria-pressed={active}
                    icon={
                      f === 'all' ? undefined : (
                        <Box sx={(theme) => ({ width: 8, height: 8, borderRadius: theme.studio.radius.pill, backgroundColor: bucketColor(theme, f) })} />
                      )
                    }
                    sx={(theme) => ({
                      borderRadius: theme.studio.radius.pill,
                      fontWeight: 600,
                      border: `1px solid ${active ? theme.palette.text.primary : theme.palette.divider}`,
                      backgroundColor: active ? theme.palette.surfaceHover : theme.palette.cardBg,
                      color: active ? theme.palette.text.primary : theme.palette.text.secondary,
                      backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
                      transition: `border-color ${theme.studio.motion.fast}, background-color ${theme.studio.motion.fast}, color ${theme.studio.motion.fast}`,
                      '& .MuiChip-icon': { ml: 1, mr: -0.5 },
                      '&:hover': { backgroundColor: theme.palette.surfaceHover, color: theme.palette.text.primary },
                    })}
                  />
                );
              })}
            </Stack>
          </Stack>
        )}

        {/* Body */}
        {isLoading ? (
          <Stack alignItems="center" sx={{ py: 10 }}>
            <CircularProgress size={26} />
          </Stack>
        ) : error && projects.length === 0 ? (
          <EmptyPanel
            tone="danger"
            icon={<LuTriangleAlert size={26} strokeWidth={1.8} />}
            title="Couldn't load projects"
            body={error instanceof Error ? error.message : 'Connect the orchestrator and try again.'}
            action={
              <Button variant="contained" color="primary" startIcon={<LuRefreshCw size={17} />} onClick={() => void qc.invalidateQueries({ queryKey: ['projects'] })}>
                Retry
              </Button>
            }
          />
        ) : filtered.length === 0 ? (
          <EmptyPanel
            tone="brand"
            icon={<LuRocket size={26} strokeWidth={1.8} />}
            title={!hasProjects ? 'No projects yet' : 'Nothing matches'}
            body={
              !hasProjects
                ? 'Describe what you want to build on the home screen, then Save to keep it here.'
                : 'Try a different search or clear the active filter.'
            }
            action={
              !hasProjects ? (
                <Button variant="contained" color="primary" startIcon={<LuSparkles size={17} />} onClick={() => navigate('/')}>
                  Start building
                </Button>
              ) : (
                <Button variant="text" color="inherit" onClick={() => { setQ(''); setFilter('all'); }}>
                  Clear filters
                </Button>
              )
            }
          />
        ) : (
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(3, 1fr)' },
              gap: 2.5,
            }}
          >
            {filtered.map((p) => (
              <ProjectCard
                key={p.id}
                project={p}
                opening={openingId === p.id}
                onOpen={() => void open(p)}
                onDelete={() => void remove(p)}
              />
            ))}
          </Box>
        )}
      </Box>
    </Box>
  );
}

// Shared empty / error panel — single glass card, semantic neon halo on the
// icon, centered copy and one action. No nested cards, no raw literals.
function EmptyPanel(props: {
  tone: 'brand' | 'danger';
  icon: React.ReactNode;
  title: string;
  body: string;
  action: React.ReactNode;
}) {
  const { tone, icon, title, body, action } = props;
  return (
    <Box
      sx={(theme) => {
        const accent = tone === 'danger' ? theme.studio.neon.danger : theme.studio.neon.violet;
        return {
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          textAlign: 'center',
          gap: 2,
          px: 3,
          py: { xs: 6, md: 8 },
          backgroundColor: theme.palette.cardBg,
          border: `1px dashed ${theme.palette.divider}`,
          borderRadius: `${theme.studio.effect.card.radius}px`,
          backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
          '& .if-empty-icon': {
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 64,
            height: 64,
            borderRadius: `${theme.studio.radius.lg}px`,
            color: accent,
            border: `1px solid ${accent}33`,
            background: `radial-gradient(120% 120% at 30% 20%, ${accent}29, ${accent}0D 70%)`,
          },
        };
      }}
    >
      <Box className="if-empty-icon" aria-hidden>
        {icon}
      </Box>
      <Stack spacing={1} sx={{ maxWidth: 460 }}>
        <Typography variant="h5" sx={{ fontWeight: 700 }}>
          {title}
        </Typography>
        <Typography color="text.secondary">{body}</Typography>
      </Stack>
      <Box sx={{ mt: 1 }}>{action}</Box>
    </Box>
  );
}
