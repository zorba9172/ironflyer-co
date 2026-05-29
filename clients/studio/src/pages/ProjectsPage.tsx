import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { Box, Button, Card, CircularProgress, IconButton, InputBase, Stack, Typography } from '@mui/material';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { confirmAction, toast } from '@ironflyer/ui-web/fx';
import { useStudio } from '../store';
import { mockProject } from '../studioData';

interface ApiProject { id: string; name: string; description?: string | null; status: string; idea?: string | null; updatedAt?: string | null }
interface ApiFile { path: string; content?: string | null }

function toneFor(status: string): string {
  const s = status.toLowerCase();
  if (s.includes('ship') || s.includes('done') || s.includes('complete')) return 'success.main';
  if (s.includes('error') || s.includes('block') || s.includes('fail')) return 'error.main';
  return 'warning.main';
}

export function ProjectsPage() {
  const navigate = useNavigate();
  const request = useRequest();
  const qc = useQueryClient();
  const openLiveProject = useStudio((s) => s.openLiveProject);
  const [q, setQ] = useState('');
  const [openingId, setOpeningId] = useState<string | null>(null);

  const { data: projects, isLoading } = useGraphQLQuery<ApiProject[], { projects: ApiProject[] }>({
    key: ['projects'],
    operationName: 'Projects', query: operations.PROJECTS,
    fallbackData: [], map: (r) => r.projects ?? [],
  });

  const filtered = projects.filter((p) => p.name.toLowerCase().includes(q.toLowerCase()));

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

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1100, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 4, flexWrap: 'wrap', gap: 2 }}>
        <Typography variant="h3" sx={{ fontSize: '2.25rem' }}>Projects</Typography>
        <Button variant="contained" onClick={() => navigate('/')} startIcon={<span>+</span>}>Create project</Button>
      </Stack>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, border: 1, borderColor: 'divider', borderRadius: 2, px: 2, py: 1, mb: 3, maxWidth: 420, bgcolor: 'background.paper' }}>
        <Box component="span" sx={{ color: 'text.disabled' }}>⌕</Box>
        <InputBase fullWidth placeholder="Search projects" value={q} onChange={(e) => setQ(e.target.value)} sx={{ fontSize: '0.9rem' }} />
      </Box>

      {isLoading ? (
        <Stack alignItems="center" sx={{ py: 8 }}><CircularProgress size={24} /></Stack>
      ) : filtered.length === 0 ? (
        <Card sx={{ p: 5, textAlign: 'center', borderStyle: 'dashed' }}>
          <Typography variant="h6" sx={{ mb: 1 }}>{projects.length === 0 ? 'No projects yet' : 'No matches'}</Typography>
          <Typography sx={{ color: 'text.secondary', mb: 3 }}>
            {projects.length === 0 ? 'Describe what you want to build on the home screen, then Save to keep it here.' : 'Try a different search.'}
          </Typography>
          {projects.length === 0 && <Button variant="contained" onClick={() => navigate('/')}>Start building</Button>}
        </Card>
      ) : (
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 2 }}>
          {filtered.map((p) => (
            <Card key={p.id} sx={{ p: 3, position: 'relative', transition: (t) => `border-color ${t.brand.motion.fast}`, '&:hover': { borderColor: 'text.disabled' }, '&:hover .if-del': { opacity: 1 } }}>
              <IconButton
                className="if-del"
                size="small"
                aria-label="Delete project"
                onClick={(e) => { e.stopPropagation(); void remove(p); }}
                sx={{ position: 'absolute', top: 8, right: 8, opacity: 0, transition: (t) => `opacity ${t.brand.motion.fast}`, color: 'text.disabled' }}
              >
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" /></svg>
              </IconButton>
              <Box onClick={() => void open(p)} sx={{ cursor: 'pointer' }}>
                <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 1.5, pr: 3 }}>
                  <Box sx={{ width: 10, height: 10, borderRadius: 99, bgcolor: toneFor(p.status), flexShrink: 0 }} />
                  <Typography variant="h6" sx={{ fontSize: '1.1rem' }} noWrap>{p.name}</Typography>
                  {openingId === p.id && <CircularProgress size={14} />}
                </Stack>
                <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem', mb: 2, minHeight: '2.6em' }}>
                  {p.description || p.idea || 'No description yet.'}
                </Typography>
                <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.disabled' })}>
                  {p.status}{p.updatedAt ? ` · updated ${new Date(p.updatedAt).toLocaleDateString()}` : ''}
                </Typography>
              </Box>
            </Card>
          ))}
        </Box>
      )}
    </Box>
  );
}
