'use client';

import { useState } from 'react';
import { Box, Button, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { ArrowForward } from '@mui/icons-material';
import { api } from '../lib/api';
import { tokens } from '../lib/theme';

const teamSizes = ['1-5', '6-25', '26-100', '101-500', '500+'];
const budgets = ['$5k-$25k', '$25k-$100k', '$100k-$500k', '$500k+'];
const timelines = ['This month', 'This quarter', 'Next quarter', 'Exploring'];

export function EnterpriseLeadForm() {
  const [form, setForm] = useState({
    name: '',
    email: '',
    company: '',
    teamSize: '26-100',
    useCase: '',
    budget: '$25k-$100k',
    timeline: 'This quarter',
  });
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<'idle' | 'sent' | 'error'>('idle');
  const [error, setError] = useState('');

  function setField(key: keyof typeof form, value: string) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  async function submit() {
    setBusy(true);
    setStatus('idle');
    setError('');
    try {
      await api.submitEnterpriseLead({ ...form, source: 'enterprise-page' });
      setStatus('sent');
    } catch (err) {
      setStatus('error');
      setError(err instanceof Error ? err.message : 'Could not submit lead');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Box sx={{
      borderRadius: { xs: 3, md: 5 },
      bgcolor: '#111',
      color: tokens.color.bg.alabaster,
      p: { xs: 2, md: 3 },
    }}>
      <Stack spacing={2}>
        <Box>
          <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900 }}>Enterprise intake</Typography>
          <Typography variant="h3" sx={{ mt: 0.8, fontSize: { xs: '2rem', md: '2.8rem' }, lineHeight: 0.95 }}>
            Turn evaluation into a sales conversation.
          </Typography>
          <Typography variant="body2" sx={{ mt: 1.2, color: '#cfc7b8', fontWeight: 600 }}>
            Capture company, team size, budget, and timeline so qualified buyers do not disappear after reading the page.
          </Typography>
        </Box>

        <Box sx={{
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
          gap: 1,
          '&& .MuiOutlinedInput-root': {
            bgcolor: '#f8f4ec',
            color: '#111',
            borderRadius: '8px',
            '& fieldset': { borderColor: 'rgba(244,240,232,0.24)' },
            '&:hover fieldset': { borderColor: tokens.color.accent.lime },
            '&.Mui-focused fieldset': { borderColor: tokens.color.accent.lime },
          },
          '&& .MuiInputBase-input': { color: '#111' },
          '&& .MuiInputBase-input::placeholder': { color: '#6b645b', opacity: 1 },
          '&& .MuiInputLabel-root': { color: '#5b554b' },
          '&& .MuiInputLabel-root.Mui-focused': { color: tokens.color.accent.lime },
          '&& .MuiSelect-icon': { color: '#111' },
        }}>
          <TextField label="Name" value={form.name} onChange={(event) => setField('name', event.target.value)} />
          <TextField label="Work email" value={form.email} onChange={(event) => setField('email', event.target.value)} required />
          <TextField label="Company" value={form.company} onChange={(event) => setField('company', event.target.value)} required />
          <SelectField label="Team size">
            <TextField select aria-label="Team size" value={form.teamSize} onChange={(event) => setField('teamSize', event.target.value)}>
              {teamSizes.map((item) => <MenuItem key={item} value={item}>{item}</MenuItem>)}
            </TextField>
          </SelectField>
          <SelectField label="Budget range">
            <TextField select aria-label="Budget range" value={form.budget} onChange={(event) => setField('budget', event.target.value)}>
              {budgets.map((item) => <MenuItem key={item} value={item}>{item}</MenuItem>)}
            </TextField>
          </SelectField>
          <SelectField label="Timeline">
            <TextField select aria-label="Timeline" value={form.timeline} onChange={(event) => setField('timeline', event.target.value)}>
              {timelines.map((item) => <MenuItem key={item} value={item}>{item}</MenuItem>)}
            </TextField>
          </SelectField>
          <TextField
            label="What do you want to build or govern?"
            value={form.useCase}
            onChange={(event) => setField('useCase', event.target.value)}
            multiline
            minRows={3}
            sx={{ gridColumn: { md: '1 / -1' } }}
          />
        </Box>

        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ xs: 'stretch', sm: 'center' }}>
          <Button
            variant="contained"
            disabled={busy || !form.email || !form.company}
            onClick={submit}
            endIcon={<ArrowForward />}
            sx={{ minHeight: 44 }}
          >
            {busy ? 'Sending...' : 'Request enterprise demo'}
          </Button>
          {status === 'sent' && <Typography variant="body2" sx={{ color: tokens.color.accent.lime, fontWeight: 800 }}>Lead captured. Sales can follow up.</Typography>}
          {status === 'error' && <Typography variant="body2" sx={{ color: tokens.color.accent.coral, fontWeight: 800 }}>{error}</Typography>}
        </Stack>
      </Stack>
    </Box>
  );
}

function SelectField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <Stack spacing={0.45}>
      <Typography variant="caption" sx={{ color: '#cfc7b8', fontWeight: 800 }}>
        {label}
      </Typography>
      {children}
    </Stack>
  );
}
