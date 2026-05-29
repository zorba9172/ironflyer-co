import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Card, Chip, Stack, Typography } from '@mui/material';
import { useStudio } from '../store';
import { text } from '@ironflyer/design-tokens/brand';

const categories = ['All', 'SaaS', 'Commerce', 'AI', 'Internal', 'Marketing'];

const templates = [
  { name: 'SaaS dashboard', cat: 'SaaS', desc: 'Auth, billing, team roles, and an admin panel.', stack: 'React · Go · Postgres' },
  { name: 'Marketplace', cat: 'Commerce', desc: 'Listings, Stripe payments, and seller payouts.', stack: 'React · Stripe · Postgres' },
  { name: 'AI chatbot', cat: 'AI', desc: 'Streaming chat, memory, and usage metering.', stack: 'React · streaming · ledger' },
  { name: 'Booking app', cat: 'Commerce', desc: 'Calendar, reminders, and Stripe checkout.', stack: 'React · Stripe · email' },
  { name: 'Internal tool', cat: 'Internal', desc: 'Tables, roles, and an audit log.', stack: 'React · RBAC · audit log' },
  { name: 'Landing + waitlist', cat: 'Marketing', desc: 'SEO pages, email capture, and analytics.', stack: 'React · SEO · analytics' },
];

export function TemplatesPage() {
  const navigate = useNavigate();
  const startFromPrompt = useStudio((s) => s.startFromPrompt);
  const [cat, setCat] = useState('All');

  const use = (name: string) => { startFromPrompt(`Start from the ${name} template`); navigate('/build'); };
  const list = cat === 'All' ? templates : templates.filter((t) => t.cat === cat);

  return (
    <Box sx={{ p: { xs: 3, md: 5 }, maxWidth: 1100, mx: 'auto' }}>
      <Typography variant="h3" sx={{ fontSize: text.s250, mb: 1 }}>Templates</Typography>
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
                <Typography variant="h6" sx={{ fontSize: text.s105 }}>{t.name}</Typography>
                <Chip size="small" label={t.cat} sx={{ height: 20, fontSize: text.s66, bgcolor: 'action.hover' }} />
              </Stack>
              <Typography sx={{ color: 'text.secondary', fontSize: text.s86, mb: 1.5 }}>{t.desc}</Typography>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s70, color: 'text.disabled' })}>{t.stack}</Typography>
            </Box>
          </Card>
        ))}
      </Box>
    </Box>
  );
}
