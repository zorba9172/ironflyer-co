// DashboardScaffolder — admin / back-office shell on Next.js. Wraps
// every /(admin) route in an auth guard backed by the existing
// supabase server.ts helper and exposes KPI helpers that read from
// the project's Postgres.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type DashboardScaffolder struct{}

func (DashboardScaffolder) Name() string { return "admin-dashboard" }

func (DashboardScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	if strings.Contains(desc, "admin") || strings.Contains(desc, "dashboard") ||
		strings.Contains(desc, "internal tool") || strings.Contains(desc, "back office") ||
		strings.Contains(desc, "back-office") || strings.Contains(desc, "backoffice") ||
		strings.Contains(desc, "metrics") || strings.Contains(desc, "analytics") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "kpi") || strings.Contains(body, "report") ||
			strings.Contains(body, "table") || strings.Contains(body, "chart") ||
			strings.Contains(body, "admin") || strings.Contains(body, "dashboard") {
			return true
		}
	}
	return false
}

func (DashboardScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"app/(admin)/layout.tsx": `// Admin shell. Every page under /(admin) inherits this layout:
//   1. Auth guard — bounces unauthenticated visitors to /login.
//   2. Role check — only users with metadata.role === 'admin' may
//      enter. Customise the predicate when your roles model grows.
//   3. Sidebar nav + content area.
import Link from 'next/link';
import { redirect } from 'next/navigation';
import { createClient } from '../../lib/supabase/server';

const NAV = [
  { href: '/', label: 'Overview' },
  { href: '/users', label: 'Users' },
];

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const supabase = await createClient();
  const { data: { user } } = await supabase.auth.getUser();
  if (!user) redirect('/login?next=/');
  const role = (user.app_metadata as { role?: string } | null)?.role;
  if (role !== 'admin') redirect('/');

  return (
    <div style={{
      display: 'grid', gridTemplateColumns: '240px 1fr',
      minHeight: '100vh', background: '#0d0e0f', color: '#fff',
    }}>
      <aside style={{ borderRight: '1px solid #1a1b1d', padding: 24 }}>
        <div style={{ color: '#c7ff00', fontWeight: 700, marginBottom: 24 }}>Admin</div>
        <nav style={{ display: 'grid', gap: 8 }}>
          {NAV.map((n) => (
            <Link key={n.href} href={n.href} style={{ color: '#fff' }}>
              {n.label}
            </Link>
          ))}
        </nav>
      </aside>
      <main style={{ padding: 32 }}>{children}</main>
    </div>
  );
}
`,
		"app/(admin)/page.tsx": `// /(admin) overview. Four KPI cards + a recent-activity table.
// Numbers come from lib/admin/kpis.ts so the page itself stays a
// thin presentation layer.
import { getOverviewKPIs, getRecentActivity } from '../../lib/admin/kpis';

export default async function AdminOverview() {
  const [kpis, activity] = await Promise.all([getOverviewKPIs(), getRecentActivity(20)]);
  return (
    <>
      <h1 style={{ color: '#c7ff00', marginTop: 0 }}>Overview</h1>
      <section style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16 }}>
        <KPICard label="Users (total)"      value={kpis.totalUsers.toLocaleString()} />
        <KPICard label="Active 7d"          value={kpis.activeLast7Days.toLocaleString()} />
        <KPICard label="Signups today"      value={kpis.signupsToday.toLocaleString()} />
        <KPICard label="Revenue MTD (USD)"  value={(kpis.revenueMtdCents / 100).toLocaleString()} />
      </section>
      <h2 style={{ marginTop: 32 }}>Recent activity</h2>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ textAlign: 'left', color: '#888' }}>
            <th style={{ padding: 8 }}>When</th>
            <th style={{ padding: 8 }}>Actor</th>
            <th style={{ padding: 8 }}>Action</th>
            <th style={{ padding: 8 }}>Target</th>
          </tr>
        </thead>
        <tbody>
          {activity.map((a) => (
            <tr key={a.id} style={{ borderTop: '1px solid #1a1b1d' }}>
              <td style={{ padding: 8 }}>{a.createdAt}</td>
              <td style={{ padding: 8 }}>{a.actor}</td>
              <td style={{ padding: 8 }}>{a.action}</td>
              <td style={{ padding: 8 }}>{a.target}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

function KPICard({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ padding: 16, border: '1px solid #1a1b1d', borderRadius: 8 }}>
      <div style={{ color: '#888', fontSize: 12 }}>{label}</div>
      <div style={{ color: '#fff', fontSize: 28, fontWeight: 700, marginTop: 4 }}>{value}</div>
    </div>
  );
}
`,
		"app/(admin)/users/page.tsx": `// /(admin)/users — paginated user table. Keep filtering on the
// server: large datasets must never round-trip every row to the
// client.
import { listUsers } from '../../../lib/admin/kpis';

export default async function UsersAdminPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string; page?: string }>;
}) {
  const sp = await searchParams;
  const q = sp.q?.trim() ?? '';
  const page = Math.max(1, Number(sp.page ?? '1') | 0);
  const { rows, total, pageSize } = await listUsers({ q, page, pageSize: 25 });
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <>
      <h1 style={{ color: '#c7ff00', marginTop: 0 }}>Users</h1>
      <form method="get" style={{ marginBottom: 16 }}>
        <input
          name="q"
          defaultValue={q}
          placeholder="Search email or id..."
          style={{ padding: 8, background: '#0d0e0f', color: '#fff', border: '1px solid #1a1b1d', borderRadius: 6 }}
        />
        <button type="submit" style={{ marginLeft: 8 }}>Search</button>
      </form>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ textAlign: 'left', color: '#888' }}>
            <th style={{ padding: 8 }}>ID</th>
            <th style={{ padding: 8 }}>Email</th>
            <th style={{ padding: 8 }}>Created</th>
            <th style={{ padding: 8 }}>Role</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((u) => (
            <tr key={u.id} style={{ borderTop: '1px solid #1a1b1d' }}>
              <td style={{ padding: 8, fontFamily: 'monospace' }}>{u.id}</td>
              <td style={{ padding: 8 }}>{u.email}</td>
              <td style={{ padding: 8 }}>{u.createdAt}</td>
              <td style={{ padding: 8 }}>{u.role ?? '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p style={{ color: '#888', marginTop: 16 }}>
        Page {page} of {totalPages} · {total.toLocaleString()} users total
      </p>
    </>
  );
}
`,
		"lib/admin/kpis.ts": `// Server-only KPI helpers. Every function returns a typed shape so
// the page components stay thin; swap the pg client out for your
// ORM of choice without rewriting the page tree.
import 'server-only';
import { Pool } from 'pg';

let pool: Pool | null = null;
function db() {
  if (!pool) pool = new Pool({ connectionString: process.env.DATABASE_URL });
  return pool;
}

export interface OverviewKPIs {
  totalUsers: number;
  activeLast7Days: number;
  signupsToday: number;
  revenueMtdCents: number;
}

// getOverviewKPIs — four numbers shown on the dashboard home page.
// activeLast7Days counts distinct users with any event in the last
// 7 days; revenueMtdCents sums paid invoices since the 1st of the
// current month. Schema assumptions are documented in the contract.
export async function getOverviewKPIs(): Promise<OverviewKPIs> {
  const sqlUsers   = "SELECT count(*)::text AS c FROM users";
  const sqlActive  =
    "SELECT count(DISTINCT user_id)::text AS c FROM events " +
    "WHERE created_at >= now() - interval '7 days'";
  const sqlSignups =
    "SELECT count(*)::text AS c FROM users " +
    "WHERE created_at >= date_trunc('day', now())";
  const sqlRevenue =
    "SELECT coalesce(sum(amount_cents), 0)::text AS s FROM payments " +
    "WHERE status = 'paid' AND paid_at >= date_trunc('month', now())";
  const [u, a, s, r] = await Promise.all([
    db().query<{ c: string }>(sqlUsers),
    db().query<{ c: string }>(sqlActive),
    db().query<{ c: string }>(sqlSignups),
    db().query<{ s: string }>(sqlRevenue),
  ]);
  return {
    totalUsers:       Number(u.rows[0]?.c ?? '0'),
    activeLast7Days:  Number(a.rows[0]?.c ?? '0'),
    signupsToday:     Number(s.rows[0]?.c ?? '0'),
    revenueMtdCents:  Number(r.rows[0]?.s ?? '0'),
  };
}

export interface ActivityRow {
  id: string;
  createdAt: string;
  actor: string;
  action: string;
  target: string;
}

// getRecentActivity — newest-first audit feed for the overview page.
export async function getRecentActivity(limit: number): Promise<ActivityRow[]> {
  const sql =
    "SELECT id, " +
    "       to_char(created_at, 'YYYY-MM-DD HH24:MI') AS \"createdAt\", " +
    "       actor, action, target " +
    "  FROM events " +
    " ORDER BY created_at DESC " +
    " LIMIT $1";
  const r = await db().query<ActivityRow>(sql, [limit]);
  return r.rows;
}

export interface UserRow {
  id: string;
  email: string;
  createdAt: string;
  role: string | null;
}

export interface UserPage {
  rows: UserRow[];
  total: number;
  page: number;
  pageSize: number;
}

// listUsers — server-paginated, server-filtered user table.
export async function listUsers(opts: { q: string; page: number; pageSize: number }): Promise<UserPage> {
  const { q, page, pageSize } = opts;
  const offset = (page - 1) * pageSize;
  if (q) {
    const sql =
      "SELECT id, email, " +
      "       to_char(created_at, 'YYYY-MM-DD') AS \"createdAt\", " +
      "       role " +
      "  FROM users " +
      " WHERE email ILIKE $1 OR id::text = $2 " +
      " ORDER BY created_at DESC " +
      " LIMIT $3 OFFSET $4";
    const rows = await db().query<UserRow>(sql, ['%' + q + '%', q, pageSize, offset]);
    const totalRes = await db().query<{ c: string }>(
      "SELECT count(*)::text AS c FROM users WHERE email ILIKE $1 OR id::text = $2",
      ['%' + q + '%', q],
    );
    return { rows: rows.rows, total: Number(totalRes.rows[0]?.c ?? '0'), page, pageSize };
  }
  const sql =
    "SELECT id, email, " +
    "       to_char(created_at, 'YYYY-MM-DD') AS \"createdAt\", " +
    "       role " +
    "  FROM users " +
    " ORDER BY created_at DESC " +
    " LIMIT $1 OFFSET $2";
  const rows = await db().query<UserRow>(sql, [pageSize, offset]);
  const totalRes = await db().query<{ c: string }>("SELECT count(*)::text AS c FROM users");
  return { rows: rows.rows, total: Number(totalRes.rows[0]?.c ?? '0'), page, pageSize };
}
`,
	}
	contract := `Admin dashboard scaffold: Next.js (app router) + Supabase auth + Postgres.

Already provisioned:
- /app/(admin)/layout.tsx        → sidebar shell + auth + role guard
- /app/(admin)/page.tsx          → overview: 4 KPI cards + activity table
- /app/(admin)/users/page.tsx    → paginated user table with search
- /lib/admin/kpis.ts             → server helpers (KPIs, activity, users)

Role-based access:
- The (admin) layout calls createClient() from /lib/supabase/server.ts
  (the existing auth scaffold) and checks user.app_metadata.role
  === 'admin'. Promote a user with:

      UPDATE auth.users SET raw_app_meta_data =
        jsonb_set(coalesce(raw_app_meta_data, '{}'::jsonb), '{role}', '"admin"')
        WHERE email = '<you@example.com>';

  When you grow more roles ('admin', 'support', 'analyst'), centralise
  the predicate in lib/admin/auth.ts and reuse it from every layout.

Registering new admin pages:
- Drop a file into app/(admin)/<segment>/page.tsx. It inherits the
  guard + sidebar automatically. Add the route to the NAV array at
  the top of layout.tsx so it shows up in the sidebar.

KPI helper contract (lib/admin/kpis.ts):
- getOverviewKPIs(): { totalUsers, activeLast7Days, signupsToday,
                       revenueMtdCents } — single round-trip via
  Promise.all. Returns plain numbers (cents stay as cents).
- getRecentActivity(limit): newest-first events for the overview feed.
- listUsers({ q, page, pageSize }): server-paginated, server-filtered
  user table — never round-trip every row to the client.

Schema assumptions (Postgres):
    users(id uuid pk, email text, created_at timestamptz, role text)
    events(id uuid pk, user_id uuid, actor text, action text,
           target text, created_at timestamptz)
    payments(id uuid pk, user_id uuid, amount_cents int, status text,
             paid_at timestamptz)
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
