import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Card, Chip, Stack, Typography } from '@mui/material';
import { useStudio } from '../store';

const categories = ['All', 'SaaS', 'Commerce', 'AI', 'Internal', 'Marketing'];

const templates = [
  { name: 'SaaS dashboard', cat: 'SaaS', desc: 'Auth, billing, team roles, and an admin panel.', usages: '24.1k' },
  { name: 'Marketplace', cat: 'Commerce', desc: 'Listings, Stripe payments, and seller payouts.', usages: '9.3k' },
  { name: 'AI chatbot', cat: 'AI', desc: 'Streaming chat, memory, and usage metering.', usages: '18.7k' },
  { name: 'Booking app', cat: 'Commerce', desc: 'Calendar, reminders, and Stripe checkout.', usages: '6.2k' },
  { name: 'Internal tool', cat: 'Internal', desc: 'Tables, roles, and an audit log.', usages: '4.8k' },
  { name: 'Landing + waitlist', cat: 'Marketing', desc: 'SEO pages, email capture, and analytics.', usages: '12.0k' },
];

export function TemplatesPage() {
  const navigate = useNavigate();
  const startFromPrompt = useStudio((s) => s.startFromPrompt);
  const [cat, setCat] = useState('All');

  const use = (name: string) => { startFromPrompt(`Start from the ${name} template`); navigate('/build'); };
  const list = cat === 'All' ? templates : templates.filter((t) => t.cat === cat);

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1100, mx: 'auto' }}>
      <Typography variant="h3" sx={{ fontSize: '2.5rem', mb: 1 }}>Templates</Typography>
      <Typography sx={{ color: 'text.secondary', mb: 3 }}>Start from a proven build and let the finisher take it to production.</Typography>

      <Stack direction="row" spacing={1} sx={{ mb: 3, flexWrap: 'wrap', gap: 1 }}>
        {categories.map((c) => (
          <Chip key={c} label={c} onClick={() => setCat(c)} variant={c === cat ? 'filled' : 'outlined'} sx={{ borderColor: 'divider', ...(c === cat ? { bgcolor: 'action.selected' } : {}) }} />
        ))}
      </Stack>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(3, 1fr)' }, gap: 2 }}>
        {list.map((t) => (
          <Card key={t.name} onClick={() => use(t.name)} sx={{ overflow: 'hidden', cursor: 'pointer', transition: (th) => `border-color ${th.brand.motion.fast}`, '&:hover': { borderColor: 'text.disabled' } }}>
            <Box sx={(th) => ({ height: 120, backgroundImage: th.brand.gradient.signatureSoft, borderBottom: 1, borderColor: 'divider' })} />
            <Box sx={{ p: 2.5 }}>
              <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 0.75 }}>
                <Typography variant="h6" sx={{ fontSize: '1.05rem' }}>{t.name}</Typography>
                <Chip size="small" label={t.cat} sx={{ height: 20, fontSize: '0.66rem', bgcolor: 'action.hover' }} />
              </Stack>
              <Typography sx={{ color: 'text.secondary', fontSize: '0.86rem', mb: 1.5 }}>{t.desc}</Typography>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.7rem', color: 'text.disabled' })}>{t.usages} uses</Typography>
            </Box>
          </Card>
        ))}
      </Box>
    </Box>
  );
}
