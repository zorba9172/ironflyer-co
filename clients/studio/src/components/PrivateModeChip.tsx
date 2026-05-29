import { useEffect, useState } from 'react';
import { Box, Stack, Tooltip, Typography } from '@mui/material';
import { useDataConfig } from '@ironflyer/data';
import { text } from '@ironflyer/design-tokens/brand';

interface Posture { private: boolean; selfHosted: boolean; privateModel: string }

// Surfaces the privacy moat: when the orchestrator runs inference on a
// self-hosted endpoint (or pins a private model), show an always-visible
// "Private" badge so the operator knows their code never leaves their infra —
// the one thing cloud studios (Cursor/Lovable/Replit) can't offer. Reads the
// deployment posture from GET /version (no GraphQL round trip).
export function PrivateModeChip() {
  const cfg = useDataConfig();
  const [posture, setPosture] = useState<Posture | null>(null);

  useEffect(() => {
    if (!cfg.endpoint) return;
    const base = cfg.endpoint.replace(/\/graphql\/?$/, '');
    let alive = true;
    fetch(`${base}/version`)
      .then((r) => r.json())
      .then((d) => { if (alive && d?.inference) setPosture(d.inference as Posture); })
      .catch(() => { /* offline / no posture — chip stays hidden */ });
    return () => { alive = false; };
  }, [cfg.endpoint]);

  if (!posture?.private) return null;
  const label = posture.selfHosted ? 'Private' : 'Private model';
  const tip = `Inference runs on your own infrastructure${posture.privateModel ? ` (${posture.privateModel})` : ''} — your code never leaves your infra.`;

  return (
    <Tooltip title={tip} arrow>
      <Stack
        direction="row"
        alignItems="center"
        spacing={0.75}
        sx={(t) => ({ px: 1, py: 0.5, borderRadius: 99, border: 1, borderColor: 'divider', bgcolor: `${t.palette.success.main}14`, color: 'success.main', userSelect: 'none' })}
      >
        <Box sx={{ width: 7, height: 7, borderRadius: 99, bgcolor: 'currentColor', flexShrink: 0 }} />
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s68, fontWeight: 600 })}>{label}</Typography>
      </Stack>
    </Tooltip>
  );
}
