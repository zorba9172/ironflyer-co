'use client';

import { Button, ButtonProps, CircularProgress } from '@mui/material';
import Link from 'next/link';
import { useState } from 'react';

import { api } from '../lib/api';
import { auth } from '../lib/auth';

type Props = ButtonProps & {
  tier: 'pro' | 'team' | 'enterprise';
  label: string;
};

// UpgradeButton drives the Pro/Team upgrade CTA. Unauthenticated visitors are
// routed to /login (so they sign in first); authenticated users start a
// Stripe Checkout session and the browser follows the redirect URL Stripe
// returns. Enterprise stays as a plain marketing link.
export function UpgradeButton({ tier, label, ...rest }: Props) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  if (tier === 'enterprise') {
    return (
      <Button component={Link} href="/enterprise" variant="contained" {...rest}>
        {label}
      </Button>
    );
  }

  const handle = async () => {
    if (!auth.token()) {
      window.location.href = `/login?next=${encodeURIComponent('/pricing')}`;
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const { url } = await api.startCheckout(tier);
      window.location.href = url;
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'checkout failed');
      setBusy(false);
    }
  };

  return (
    <>
      <Button
        variant="contained"
        onClick={handle}
        disabled={busy}
        startIcon={busy ? <CircularProgress size={16} color="inherit" /> : undefined}
        {...rest}
      >
        {label}
      </Button>
      {err && (
        <span style={{ color: '#b91c1c', fontSize: 12, marginTop: 8 }}>{err}</span>
      )}
    </>
  );
}
