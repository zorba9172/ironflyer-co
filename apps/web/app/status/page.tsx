'use client';

import { useCallback, useEffect, useState } from 'react';

// Public status page. Polls the orchestrator + runtime healthz endpoints
// every 30 seconds and renders a row per dependency. No auth — anyone
// can see whether the platform is up.
//
// We never throw on a fetch failure; instead we surface the failure as
// the "down" state for that service. This page is the user's last line
// of trust when something is broken, so it has to keep rendering even
// when the backend is on fire.

type ServiceState = 'ok' | 'degraded' | 'down' | 'unknown';

type OrchestratorHealth = {
  status?: string;
  services?: Record<string, boolean>;
  version?: string;
  uptime?: string;
};

type RuntimeHealth = {
  status?: string;
  services?: { driver?: string; preview?: boolean };
  version?: string;
  uptime?: string;
};

type Row = {
  name: string;
  state: ServiceState;
  detail?: string;
};

const ORCH_URL =
  process.env.NEXT_PUBLIC_ORCH_URL ?? '/api/orchestrator';
const RUNTIME_URL =
  process.env.NEXT_PUBLIC_RUNTIME_URL ?? '/api/runtime';

const POLL_MS = 30_000;

export default function StatusPage() {
  const [rows, setRows] = useState<Row[]>(initialRows());
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [overall, setOverall] = useState<ServiceState>('unknown');

  const refresh = useCallback(async () => {
    const next = await fetchAll();
    setRows(next.rows);
    setOverall(next.overall);
    setLastUpdated(new Date());
  }, []);

  useEffect(() => {
    void refresh();
    const id = setInterval(refresh, POLL_MS);
    return () => clearInterval(id);
  }, [refresh]);

  return (
    <main
      dir="rtl"
      style={{
        minHeight: '100vh',
        background: '#f4f0e8',
        color: '#0d0e0f',
        padding: '64px 24px',
        fontFamily: 'var(--font-body), Inter, sans-serif',
      }}
    >
      <div style={{ maxWidth: 760, margin: '0 auto' }}>
        <header style={{ display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 32 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div
              aria-hidden
              style={{
                width: 40,
                height: 40,
                background: '#e5ff00',
                borderRadius: 8,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontFamily: 'var(--font-display), Arial Black, sans-serif',
                fontSize: 26,
                lineHeight: 1,
              }}
            >
              I
            </div>
            <h1
              style={{
                margin: 0,
                fontFamily: 'var(--font-display), Arial Black, sans-serif',
                fontSize: '2rem',
                letterSpacing: -0.5,
              }}
            >
              סטטוס Ironflyer
            </h1>
          </div>
          <p style={{ margin: 0, color: '#3a3a36', lineHeight: 1.5 }}>
            הדף הזה מתעדכן אוטומטית כל 30 שניות. אם משהו לא תקין — תראו את זה כאן.
          </p>
        </header>

        <section
          style={{
            background: '#fff',
            border: '1px solid rgba(13,14,15,0.08)',
            borderRadius: 16,
            padding: 24,
            marginBottom: 24,
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 16,
              flexWrap: 'wrap',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <StatusDot state={overall} />
              <strong style={{ fontSize: '1.125rem' }}>{overallLabel(overall)}</strong>
            </div>
            <span style={{ color: '#77736b', fontSize: '0.875rem' }}>
              {lastUpdated
                ? `עודכן ב-${lastUpdated.toLocaleTimeString('he-IL')}`
                : 'טוען…'}
            </span>
          </div>
        </section>

        <section
          style={{
            background: '#fff',
            border: '1px solid rgba(13,14,15,0.08)',
            borderRadius: 16,
            padding: 8,
          }}
        >
          <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
            {rows.map((row) => (
              <li
                key={row.name}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '16px 16px',
                  borderBottom: '1px solid rgba(13,14,15,0.06)',
                  gap: 16,
                }}
              >
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <strong style={{ fontSize: '1rem' }}>{row.name}</strong>
                  {row.detail && (
                    <span style={{ color: '#77736b', fontSize: '0.8125rem' }}>{row.detail}</span>
                  )}
                </div>
                <StatusPill state={row.state} />
              </li>
            ))}
          </ul>
        </section>

        <p style={{ marginTop: 24, color: '#77736b', fontSize: '0.8125rem' }}>
          הנתונים נשלפים מ-{ORCH_URL}/healthz ומ-{RUNTIME_URL}/healthz.
        </p>
      </div>
    </main>
  );
}

function initialRows(): Row[] {
  return [
    { name: 'Orchestrator', state: 'unknown' },
    { name: 'Runtime', state: 'unknown' },
    { name: 'Anthropic', state: 'unknown' },
    { name: 'Postgres', state: 'unknown' },
    { name: 'Stripe', state: 'unknown' },
  ];
}

async function fetchAll(): Promise<{ rows: Row[]; overall: ServiceState }> {
  const [orch, rt] = await Promise.all([
    safeFetchJson<OrchestratorHealth>(`${ORCH_URL}/healthz`),
    safeFetchJson<RuntimeHealth>(`${RUNTIME_URL}/healthz`),
  ]);

  const orchReachable = orch !== null;
  const rtReachable = rt !== null;

  const services = orch?.services ?? {};
  const orchState: ServiceState = orchReachable
    ? orch?.status === 'ok'
      ? 'ok'
      : 'degraded'
    : 'down';
  const rtState: ServiceState = rtReachable
    ? rt?.status === 'ok'
      ? 'ok'
      : 'degraded'
    : 'down';

  const rows: Row[] = [
    {
      name: 'Orchestrator',
      state: orchState,
      detail: orch?.version
        ? `גרסה ${orch.version}${orch.uptime ? ` · ${orch.uptime}` : ''}`
        : undefined,
    },
    {
      name: 'Runtime',
      state: rtState,
      detail: rt?.services?.driver
        ? `Driver: ${rt.services.driver}${rt.uptime ? ` · ${rt.uptime}` : ''}`
        : undefined,
    },
    {
      name: 'Anthropic',
      state: boolState(services.anthropic, orchReachable),
      detail: 'נתב מודלים ראשי',
    },
    {
      name: 'Postgres',
      state: boolState(services.postgres, orchReachable),
      detail: 'מסד נתונים ראשי',
    },
    {
      name: 'Stripe',
      state: boolState(services.stripe, orchReachable),
      detail: 'תשלומים ומנויים',
    },
  ];

  const worst = rows.reduce<ServiceState>((acc, r) => worstOf(acc, r.state), 'ok');
  return { rows, overall: worst };
}

async function safeFetchJson<T>(url: string): Promise<T | null> {
  try {
    const res = await fetch(url, { cache: 'no-store' });
    if (!res.ok) return null;
    return (await res.json()) as T;
  } catch {
    return null;
  }
}

function boolState(v: boolean | undefined, reachable: boolean): ServiceState {
  if (!reachable) return 'down';
  if (v === undefined) return 'unknown';
  return v ? 'ok' : 'degraded';
}

// worstOf collapses two states into the more pessimistic of the two so
// the global "overall" pill always reflects the worst dependency.
function worstOf(a: ServiceState, b: ServiceState): ServiceState {
  const rank: Record<ServiceState, number> = { ok: 0, unknown: 1, degraded: 2, down: 3 };
  return rank[a] >= rank[b] ? a : b;
}

function overallLabel(state: ServiceState): string {
  switch (state) {
    case 'ok':
      return 'כל המערכות פעילות';
    case 'degraded':
      return 'תקלה חלקית';
    case 'down':
      return 'שירות לא זמין';
    default:
      return 'בודקים…';
  }
}

function StatusDot({ state }: { state: ServiceState }) {
  return (
    <span
      aria-hidden
      style={{
        display: 'inline-block',
        width: 14,
        height: 14,
        borderRadius: '50%',
        background: dotColor(state),
      }}
    />
  );
}

function StatusPill({ state }: { state: ServiceState }) {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 8,
        padding: '6px 12px',
        background: pillBg(state),
        color: pillFg(state),
        borderRadius: 999,
        fontSize: '0.8125rem',
        fontWeight: 600,
      }}
    >
      <span
        aria-hidden
        style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          background: dotColor(state),
        }}
      />
      {pillLabel(state)}
    </span>
  );
}

function dotColor(state: ServiceState): string {
  switch (state) {
    case 'ok':
      return '#79e07a';
    case 'degraded':
      return '#ffc400';
    case 'down':
      return '#ff1818';
    default:
      return '#b9b3a8';
  }
}

function pillBg(state: ServiceState): string {
  switch (state) {
    case 'ok':
      return '#eafbeb';
    case 'degraded':
      return '#fff5d6';
    case 'down':
      return '#ffe1e1';
    default:
      return '#eeeae0';
  }
}

function pillFg(state: ServiceState): string {
  switch (state) {
    case 'ok':
      return '#1c5d1d';
    case 'degraded':
      return '#7a5500';
    case 'down':
      return '#7a0000';
    default:
      return '#3a3a36';
  }
}

function pillLabel(state: ServiceState): string {
  switch (state) {
    case 'ok':
      return 'תקין';
    case 'degraded':
      return 'תקלה חלקית';
    case 'down':
      return 'לא זמין';
    default:
      return 'בודקים';
  }
}
