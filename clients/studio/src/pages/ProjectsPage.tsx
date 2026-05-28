import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Button, Card, InputBase, Stack, Typography } from '@mui/material';
import { useStudio } from '../store';
import { mockProject, type StudioProject } from '../studioData';

const sample: { project: StudioProject; desc: string; tone: string }[] = [
  { project: mockProject, desc: 'Checkout flow imported from Lovable. 2 gates open.', tone: 'warning.main' },
  {
    project: { ...mockProject, id: 'p_math', name: 'MathQuest', source: 'shipped', completion: 1, gates: mockProject.gates.map((g) => ({ ...g, status: 'closed', blocking: '', level: 1 })) },
    desc: 'Gamified math learning platform. Shipped — 0 gates open.',
    tone: 'success.main',
  },
];

export function ProjectsPage() {
  const navigate = useNavigate();
  const { openProject, startFromPrompt } = useStudio();
  const [q, setQ] = useState('');

  const open = (p: StudioProject) => { openProject(p); navigate('/build'); };
  const create = () => { startFromPrompt('Finish my product'); navigate('/build'); };
  const filtered = sample.filter((s) => s.project.name.toLowerCase().includes(q.toLowerCase()));

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1100, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 4, flexWrap: 'wrap', gap: 2 }}>
        <Typography variant="h3" sx={{ fontSize: '2.25rem' }}>Projects</Typography>
        <Button variant="contained" onClick={create} startIcon={<span>+</span>}>Create project</Button>
      </Stack>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, border: 1, borderColor: 'divider', borderRadius: 2, px: 2, py: 1, mb: 3, maxWidth: 420, bgcolor: 'background.paper' }}>
        <Box component="span" sx={{ color: 'text.disabled' }}>⌕</Box>
        <InputBase fullWidth placeholder="Search projects" value={q} onChange={(e) => setQ(e.target.value)} sx={{ fontSize: '0.9rem' }} />
      </Box>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 2 }}>
        {filtered.map(({ project, desc, tone }) => (
          <Card key={project.id} onClick={() => open(project)} sx={{ p: 3, cursor: 'pointer', transition: (t) => `border-color ${t.brand.motion.fast}`, '&:hover': { borderColor: 'text.disabled' } }}>
            <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 1.5 }}>
              <Box sx={{ width: 10, height: 10, borderRadius: 99, bgcolor: tone }} />
              <Typography variant="h6" sx={{ fontSize: '1.1rem' }}>{project.name}</Typography>
            </Stack>
            <Typography sx={{ color: 'text.secondary', fontSize: '0.9rem', mb: 2 }}>{desc}</Typography>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.72rem', color: 'text.disabled' })}>{project.source} · {Math.round(project.completion * 100)}% to shippable</Typography>
          </Card>
        ))}
      </Box>
    </Box>
  );
}
