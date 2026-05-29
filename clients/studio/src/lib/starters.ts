// Instant-start scaffolds. Each is a self-contained, dependency-free React
// component that renders in the in-browser Sandpack preview in ~2 seconds —
// no LLM round trip. The home seeds these so a customer sees a *running* app
// immediately, then the agent enhances it on top (with gates/cost/private
// visible). This is the speed-feel of Base44 et al., on the finisher engine.
//
// Files are kept to a single src/App.tsx with inline styles so the bundler has
// nothing to resolve and the preview can never fail on a missing dependency.

export interface Starter {
  id: string;
  name: string;
  meta: string;
  /** The prompt seeded into the chat so the agent continues from the scaffold. */
  prompt: string;
  files: { path: string; content: string }[];
}

const dashboardApp = `import React from 'react';

const card = { background: '#13131a', border: '1px solid #26263a', borderRadius: 14, padding: 20 };
const stats = [
  { label: 'MRR', value: '$48.2k', delta: '+12.4%' },
  { label: 'Active users', value: '8,914', delta: '+3.1%' },
  { label: 'Churn', value: '1.8%', delta: '-0.4%' },
  { label: 'NPS', value: '62', delta: '+5' },
];
const rows = [
  ['Acme Inc', 'Scale', '$1,200/mo', 'Active'],
  ['Globex', 'Pro', '$300/mo', 'Active'],
  ['Initech', 'Free', '$0', 'Trial'],
  ['Umbrella', 'Scale', '$1,200/mo', 'Past due'],
];

export default function App() {
  return (
    <div style={{ display: 'flex', minHeight: '100vh', fontFamily: 'Inter, system-ui, sans-serif', color: '#e7e7ef', background: '#0b0b10' }}>
      <aside style={{ width: 220, borderRight: '1px solid #26263a', padding: 24 }}>
        <div style={{ fontWeight: 800, fontSize: 18, marginBottom: 28, background: 'linear-gradient(90deg,#5b8cff,#22d3ee)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>Northwind</div>
        {['Overview', 'Customers', 'Billing', 'Analytics', 'Settings'].map((x, i) => (
          <div key={x} style={{ padding: '10px 12px', borderRadius: 8, marginBottom: 4, background: i === 0 ? '#1c1c28' : 'transparent', color: i === 0 ? '#fff' : '#9a9ab0', cursor: 'pointer' }}>{x}</div>
        ))}
      </aside>
      <main style={{ flex: 1, padding: 32 }}>
        <h1 style={{ fontSize: 26, margin: '0 0 24px' }}>Overview</h1>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,1fr)', gap: 16, marginBottom: 28 }}>
          {stats.map((s) => (
            <div key={s.label} style={card}>
              <div style={{ fontSize: 12, color: '#9a9ab0', textTransform: 'uppercase', letterSpacing: 1 }}>{s.label}</div>
              <div style={{ fontSize: 28, fontWeight: 700, margin: '6px 0' }}>{s.value}</div>
              <div style={{ fontSize: 13, color: s.delta.startsWith('-') ? '#f87171' : '#34d399' }}>{s.delta}</div>
            </div>
          ))}
        </div>
        <div style={{ ...card, padding: 0 }}>
          <div style={{ padding: '16px 20px', borderBottom: '1px solid #26263a', fontWeight: 600 }}>Customers</div>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead><tr style={{ color: '#9a9ab0', textAlign: 'left' }}>{['Account', 'Plan', 'MRR', 'Status'].map((h) => <th key={h} style={{ padding: '12px 20px', fontWeight: 500 }}>{h}</th>)}</tr></thead>
            <tbody>{rows.map((r, i) => (<tr key={i} style={{ borderTop: '1px solid #1c1c28' }}>{r.map((c, j) => <td key={j} style={{ padding: '14px 20px' }}>{c}</td>)}</tr>))}</tbody>
          </table>
        </div>
      </main>
    </div>
  );
}
`;

const landingApp = `import React from 'react';

const features = [
  { t: 'Ships, not demos', d: 'Gates block fake-shipping until it actually works.' },
  { t: 'No surprise bills', d: 'Live cost + a prepaid wallet you can see burn down.' },
  { t: 'Your infra', d: 'Run on private models — your code never leaves.' },
];

export default function App() {
  return (
    <div style={{ fontFamily: 'Inter, system-ui, sans-serif', color: '#e7e7ef', background: 'radial-gradient(1200px 600px at 50% -10%, #1a1a2e, #0b0b10)', minHeight: '100vh' }}>
      <nav style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '20px 40px' }}>
        <div style={{ fontWeight: 800, fontSize: 20 }}>Ironflyer</div>
        <button style={{ background: 'linear-gradient(90deg,#5b8cff,#9b5bff)', border: 0, color: '#fff', padding: '10px 18px', borderRadius: 10, fontWeight: 600, cursor: 'pointer' }}>Start building</button>
      </nav>
      <header style={{ textAlign: 'center', padding: '80px 24px 60px', maxWidth: 820, margin: '0 auto' }}>
        <div style={{ fontSize: 13, color: '#9b5bff', fontWeight: 600, letterSpacing: 1, marginBottom: 16 }}>THE PRODUCT FINISHER</div>
        <h1 style={{ fontSize: 56, lineHeight: 1.05, margin: '0 0 20px' }}>They generate fast.<br/>We ship profitably.</h1>
        <p style={{ fontSize: 18, color: '#9a9ab0', margin: '0 0 32px' }}>The only engine where you can't fake-ship and can't get a surprise bill — every execution carries a margin.</p>
        <button style={{ background: 'linear-gradient(90deg,#ff6b6b,#9b5bff)', border: 0, color: '#fff', padding: '14px 28px', borderRadius: 12, fontWeight: 700, fontSize: 16, cursor: 'pointer' }}>Build something →</button>
      </header>
      <section style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 16, maxWidth: 920, margin: '0 auto', padding: '0 24px 80px' }}>
        {features.map((f) => (
          <div key={f.t} style={{ background: '#13131a', border: '1px solid #26263a', borderRadius: 14, padding: 24 }}>
            <div style={{ fontWeight: 700, fontSize: 17, marginBottom: 8 }}>{f.t}</div>
            <div style={{ color: '#9a9ab0', fontSize: 14, lineHeight: 1.5 }}>{f.d}</div>
          </div>
        ))}
      </section>
    </div>
  );
}
`;

const chatApp = `import React, { useState } from 'react';

interface Msg { role: 'user' | 'ai'; text: string; }

export default function App() {
  const [msgs, setMsgs] = useState<Msg[]>([
    { role: 'ai', text: 'Hi! I run on private inference — your data never leaves your infra. Ask me anything.' },
  ]);
  const [draft, setDraft] = useState('');
  const send = () => {
    const t = draft.trim();
    if (!t) return;
    setMsgs((m) => [...m, { role: 'user', text: t }, { role: 'ai', text: 'Grounding against live docs… here is a verified answer to: ' + t }]);
    setDraft('');
  };
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', fontFamily: 'Inter, system-ui, sans-serif', background: '#0b0b10', color: '#e7e7ef' }}>
      <div style={{ padding: '16px 20px', borderBottom: '1px solid #26263a', fontWeight: 700 }}>Assistant <span style={{ fontSize: 12, color: '#34d399', marginLeft: 8 }}>● private</span></div>
      <div style={{ flex: 1, overflowY: 'auto', padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
        {msgs.map((m, i) => (
          <div key={i} style={{ alignSelf: m.role === 'user' ? 'flex-end' : 'flex-start', maxWidth: '70%', background: m.role === 'user' ? 'linear-gradient(90deg,#5b8cff,#9b5bff)' : '#13131a', border: m.role === 'user' ? 0 : '1px solid #26263a', color: '#fff', padding: '12px 16px', borderRadius: 14, fontSize: 14, lineHeight: 1.5 }}>{m.text}</div>
        ))}
      </div>
      <div style={{ display: 'flex', gap: 10, padding: 16, borderTop: '1px solid #26263a' }}>
        <input value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') send(); }} placeholder="Message…" style={{ flex: 1, background: '#13131a', border: '1px solid #26263a', borderRadius: 10, padding: '12px 14px', color: '#e7e7ef', fontSize: 14, outline: 'none' }} />
        <button onClick={send} style={{ background: 'linear-gradient(90deg,#5b8cff,#9b5bff)', border: 0, color: '#fff', padding: '0 20px', borderRadius: 10, fontWeight: 600, cursor: 'pointer' }}>Send</button>
      </div>
    </div>
  );
}
`;

const marketplaceApp = `import React, { useState } from 'react';

const listings = [
  { t: 'Vintage Film Camera', p: 220, cat: 'Photography', e: '📷' },
  { t: 'Mechanical Keyboard', p: 145, cat: 'Tech', e: '⌨️' },
  { t: 'Mid-century Chair', p: 380, cat: 'Furniture', e: '🪑' },
  { t: 'Mountain Bike', p: 640, cat: 'Outdoors', e: '🚲' },
  { t: 'Acoustic Guitar', p: 310, cat: 'Music', e: '🎸' },
  { t: 'Espresso Machine', p: 540, cat: 'Home', e: '☕' },
];

export default function App() {
  const [cart, setCart] = useState(0);
  return (
    <div style={{ fontFamily: 'Inter, system-ui, sans-serif', background: '#0b0b10', color: '#e7e7ef', minHeight: '100vh' }}>
      <nav style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '18px 32px', borderBottom: '1px solid #26263a' }}>
        <div style={{ fontWeight: 800, fontSize: 20 }}>Bazaar</div>
        <div style={{ background: '#13131a', border: '1px solid #26263a', borderRadius: 99, padding: '8px 16px', fontSize: 14 }}>🛒 {cart}</div>
      </nav>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 16, padding: 32, maxWidth: 1000, margin: '0 auto' }}>
        {listings.map((l) => (
          <div key={l.t} style={{ background: '#13131a', border: '1px solid #26263a', borderRadius: 14, overflow: 'hidden' }}>
            <div style={{ height: 120, display: 'grid', placeItems: 'center', fontSize: 48, background: 'linear-gradient(135deg,#1c1c28,#13131a)' }}>{l.e}</div>
            <div style={{ padding: 16 }}>
              <div style={{ fontSize: 11, color: '#9b5bff', textTransform: 'uppercase', letterSpacing: 1 }}>{l.cat}</div>
              <div style={{ fontWeight: 600, margin: '6px 0' }}>{l.t}</div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 10 }}>
                <span style={{ fontSize: 18, fontWeight: 700 }}>\${l.p}</span>
                <button onClick={() => setCart((c) => c + 1)} style={{ background: 'linear-gradient(90deg,#5b8cff,#9b5bff)', border: 0, color: '#fff', padding: '8px 14px', borderRadius: 8, fontWeight: 600, cursor: 'pointer' }}>Add</button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
`;

const bookingApp = `import React, { useState } from 'react';

const slots = ['09:00', '09:30', '10:00', '10:30', '11:00', '13:00', '13:30', '14:00', '15:00', '16:00'];
const days = ['Mon 12', 'Tue 13', 'Wed 14', 'Thu 15', 'Fri 16'];

export default function App() {
  const [day, setDay] = useState(0);
  const [picked, setPicked] = useState<string | null>(null);
  return (
    <div style={{ fontFamily: 'Inter, system-ui, sans-serif', background: '#0b0b10', color: '#e7e7ef', minHeight: '100vh', padding: 32 }}>
      <div style={{ maxWidth: 560, margin: '0 auto' }}>
        <h1 style={{ fontSize: 26, marginBottom: 6 }}>Book a session</h1>
        <p style={{ color: '#9a9ab0', marginTop: 0 }}>30 min · video call · confirmation by email</p>
        <div style={{ display: 'flex', gap: 8, margin: '24px 0', flexWrap: 'wrap' }}>
          {days.map((d, i) => (
            <button key={d} onClick={() => { setDay(i); setPicked(null); }} style={{ flex: 1, minWidth: 80, padding: '12px 0', borderRadius: 10, border: '1px solid ' + (i === day ? '#5b8cff' : '#26263a'), background: i === day ? '#1c1c28' : 'transparent', color: i === day ? '#fff' : '#9a9ab0', cursor: 'pointer', fontWeight: 600 }}>{d}</button>
          ))}
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5,1fr)', gap: 8 }}>
          {slots.map((s) => (
            <button key={s} onClick={() => setPicked(s)} style={{ padding: '12px 0', borderRadius: 8, border: '1px solid ' + (picked === s ? '#34d399' : '#26263a'), background: picked === s ? 'rgba(52,211,153,0.12)' : '#13131a', color: picked === s ? '#34d399' : '#e7e7ef', cursor: 'pointer', fontSize: 14 }}>{s}</button>
          ))}
        </div>
        {picked && (
          <button style={{ width: '100%', marginTop: 24, padding: 14, borderRadius: 12, border: 0, background: 'linear-gradient(90deg,#5b8cff,#9b5bff)', color: '#fff', fontWeight: 700, fontSize: 16, cursor: 'pointer' }}>Confirm {days[day]} at {picked} →</button>
        )}
      </div>
    </div>
  );
}
`;

const internalToolApp = `import React, { useState } from 'react';

const seed = [
  { name: 'Ava Stone', role: 'Admin', team: 'Platform', status: 'Active' },
  { name: 'Liam Park', role: 'Editor', team: 'Growth', status: 'Active' },
  { name: 'Noa Levi', role: 'Viewer', team: 'Finance', status: 'Invited' },
  { name: 'Kai Reyes', role: 'Editor', team: 'Platform', status: 'Active' },
  { name: 'Mira Costa', role: 'Admin', team: 'Security', status: 'Suspended' },
];
const roleColor: Record<string, string> = { Admin: '#9b5bff', Editor: '#5b8cff', Viewer: '#9a9ab0' };

export default function App() {
  const [q, setQ] = useState('');
  const rows = seed.filter((r) => (r.name + r.team + r.role).toLowerCase().includes(q.toLowerCase()));
  return (
    <div style={{ fontFamily: 'Inter, system-ui, sans-serif', background: '#0b0b10', color: '#e7e7ef', minHeight: '100vh', padding: 32 }}>
      <div style={{ maxWidth: 880, margin: '0 auto' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <h1 style={{ fontSize: 24, margin: 0 }}>Team members</h1>
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search…" style={{ background: '#13131a', border: '1px solid #26263a', borderRadius: 10, padding: '10px 14px', color: '#e7e7ef', outline: 'none' }} />
        </div>
        <div style={{ background: '#13131a', border: '1px solid #26263a', borderRadius: 14, overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
            <thead><tr style={{ color: '#9a9ab0', textAlign: 'left' }}>{['Name', 'Role', 'Team', 'Status'].map((h) => <th key={h} style={{ padding: '14px 18px', fontWeight: 500 }}>{h}</th>)}</tr></thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.name} style={{ borderTop: '1px solid #1c1c28' }}>
                  <td style={{ padding: '14px 18px', fontWeight: 600 }}>{r.name}</td>
                  <td style={{ padding: '14px 18px' }}><span style={{ color: roleColor[r.role] || '#9a9ab0' }}>● {r.role}</span></td>
                  <td style={{ padding: '14px 18px', color: '#9a9ab0' }}>{r.team}</td>
                  <td style={{ padding: '14px 18px' }}><span style={{ fontSize: 12, padding: '3px 10px', borderRadius: 99, background: r.status === 'Active' ? 'rgba(52,211,153,0.14)' : r.status === 'Suspended' ? 'rgba(248,113,113,0.14)' : '#1c1c28', color: r.status === 'Active' ? '#34d399' : r.status === 'Suspended' ? '#f87171' : '#9a9ab0' }}>{r.status}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
`;

const shellApp = `import React from 'react';

const blocks = ['Overview', 'Data', 'Settings'];

export default function App() {
  return (
    <div style={{ display: 'flex', minHeight: '100vh', fontFamily: 'Inter, system-ui, sans-serif', background: '#0b0b10', color: '#e7e7ef' }}>
      <aside style={{ width: 200, borderRight: '1px solid #26263a', padding: 24 }}>
        <div style={{ fontWeight: 800, fontSize: 18, marginBottom: 28, background: 'linear-gradient(90deg,#5b8cff,#9b5bff)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>Your product</div>
        {blocks.map((b, i) => (
          <div key={b} style={{ padding: '10px 12px', borderRadius: 8, marginBottom: 4, background: i === 0 ? '#1c1c28' : 'transparent', color: i === 0 ? '#fff' : '#9a9ab0' }}>{b}</div>
        ))}
      </aside>
      <main style={{ flex: 1, padding: 40 }}>
        <div style={{ display: 'inline-block', fontSize: 12, color: '#9b5bff', border: '1px solid #2a2140', borderRadius: 99, padding: '4px 12px', marginBottom: 18 }}>● building</div>
        <h1 style={{ fontSize: 32, margin: '0 0 10px' }}>Your product is taking shape</h1>
        <p style={{ color: '#9a9ab0', maxWidth: 520, lineHeight: 1.6 }}>This runnable shell rendered instantly. The agent is now wiring your real screens, data, and logic on top — watch the build rail on the right.</p>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 16, marginTop: 32, maxWidth: 720 }}>
          {[0, 1, 2].map((i) => (
            <div key={i} style={{ background: '#13131a', border: '1px solid #26263a', borderRadius: 14, padding: 20, height: 110 }}>
              <div style={{ width: '60%', height: 10, borderRadius: 6, background: '#26263a', marginBottom: 12 }} />
              <div style={{ width: '90%', height: 8, borderRadius: 6, background: '#1c1c28', marginBottom: 8 }} />
              <div style={{ width: '75%', height: 8, borderRadius: 6, background: '#1c1c28' }} />
            </div>
          ))}
        </div>
      </main>
    </div>
  );
}
`;

// matchStarter picks the closest runnable scaffold for a free-text prompt so
// the primary "type and go" flow also lands on a running app in seconds. A
// confident keyword hit returns that starter; everything else gets the neutral
// app shell. Either way the operator's real prompt is what's seeded into chat.
export function matchStarter(prompt: string): Starter {
  const p = prompt.toLowerCase();
  const hit = (...kw: string[]) => kw.some((k) => p.includes(k));
  const byId = (id: string) => STARTERS.find((s) => s.id === id);
  let s: Starter | undefined;
  if (hit('market', 'shop', 'store', 'ecommerce', 'e-commerce', 'listing', 'marketplace')) s = byId('marketplace');
  else if (hit('chat', 'bot', 'assistant', 'llm', 'gpt', 'conversation')) s = byId('ai-chat');
  else if (hit('book', 'appointment', 'calendar', 'schedule', 'reservation', 'slot')) s = byId('booking');
  else if (hit('landing', 'marketing site', 'homepage', 'home page', 'waitlist')) s = byId('landing');
  else if (hit('admin', 'internal', 'crud', 'back office', 'backoffice', 'roles', 'table')) s = byId('internal-tool');
  else if (hit('dashboard', 'saas', 'analytics', 'metrics', 'billing')) s = byId('saas-dashboard');
  const base = s ?? { id: 'shell', name: 'App shell', meta: '', prompt: '', files: [{ path: 'src/App.tsx', content: shellApp }] };
  // Seed the operator's own words as the build prompt, not the starter's canned one.
  return { ...base, prompt };
}

export const STARTERS: Starter[] = [
  {
    id: 'saas-dashboard',
    name: 'SaaS dashboard',
    meta: 'Auth · billing · admin',
    prompt: 'Finish this SaaS dashboard: wire real auth, a billing/subscription flow, and a customers admin backed by a database.',
    files: [{ path: 'src/App.tsx', content: dashboardApp }],
  },
  {
    id: 'landing',
    name: 'Landing page',
    meta: 'Hero · features · CTA',
    prompt: 'Finish this landing page: make the CTA capture leads into a database, add a pricing section, and wire analytics.',
    files: [{ path: 'src/App.tsx', content: landingApp }],
  },
  {
    id: 'ai-chat',
    name: 'AI chatbot',
    meta: 'Streaming · memory · usage',
    prompt: 'Finish this AI chat app: stream real responses from a private model, add conversation memory, and meter usage.',
    files: [{ path: 'src/App.tsx', content: chatApp }],
  },
  {
    id: 'marketplace',
    name: 'Marketplace',
    meta: 'Listings · payments · payouts',
    prompt: 'Finish this marketplace: wire listings to a database, add Stripe checkout + seller payouts, and search.',
    files: [{ path: 'src/App.tsx', content: marketplaceApp }],
  },
  {
    id: 'booking',
    name: 'Booking app',
    meta: 'Calendar · reminders · Stripe',
    prompt: 'Finish this booking app: persist availability, take deposits via Stripe, and send email/SMS reminders.',
    files: [{ path: 'src/App.tsx', content: bookingApp }],
  },
  {
    id: 'internal-tool',
    name: 'Internal tool',
    meta: 'Tables · roles · audit log',
    prompt: 'Finish this internal admin: back the table with a database, add role-based access, and an audit log.',
    files: [{ path: 'src/App.tsx', content: internalToolApp }],
  },
];
