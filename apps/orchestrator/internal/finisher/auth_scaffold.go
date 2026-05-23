// Package finisher — auth scaffold step. Most generated apps need a
// real auth flow (signup, login, session) and "hope the Coder remembers
// to wire it" is not a finisher-grade behaviour. This step writes a
// deterministic, framework-matched auth skeleton into the project before
// the Coder runs, so the Coder fills in product code on top of a
// guaranteed-correct foundation instead of inventing the recipe each
// time.
//
// Supported recipes today:
//   - Supabase Auth on Next.js (app router)
//   - "none": skipped (the Coder is told no auth is in scope)
//
// The choice is driven by ProductSpec.Stack.Auth + Stack.Frontend. Empty
// Auth defaults to "supabase" when the stack is Next.js (the canonical
// finisher-recommended pairing); other framework combinations fall
// through to "none" until a recipe lands here.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

// AuthScaffolder is the operator-replaceable contract. The default
// implementation (DefaultAuthScaffolder) writes Supabase + Next.js files;
// operators that ship Clerk/Auth.js/NextAuth recipes can swap in their
// own. Returning an empty FileMap with a nil error means "no auth needed"
// — the pipeline treats it as a no-op.
type AuthScaffolder interface {
	Scaffold(ctx context.Context, p *domain.Project) (AuthScaffold, error)
}

// AuthScaffold is one bundle of files + a contract the Coder reads as
// context.
type AuthScaffold struct {
	// Provider is a human label, e.g. "supabase", "clerk", "none". Used in
	// SSE events and the Coder's system prompt.
	Provider string
	// Files keyed by relative project path → file body. The pipeline upserts
	// every entry into the project's file tree.
	Files map[string]string
	// Contract is a short markdown block the Coder receives via the project
	// context describing what the scaffold provides and what the agent is
	// expected to do on top of it (e.g. "auth routes already exist, do not
	// reimplement").
	Contract string
}

// DefaultAuthScaffolder picks a recipe based on Stack.Auth + Stack.Frontend.
type DefaultAuthScaffolder struct{}

func (DefaultAuthScaffolder) Scaffold(_ context.Context, p *domain.Project) (AuthScaffold, error) {
	if p == nil {
		return AuthScaffold{}, nil
	}
	authChoice := strings.ToLower(strings.TrimSpace(p.Spec.Stack.Auth))
	frontend := strings.ToLower(strings.TrimSpace(p.Spec.Stack.Frontend))
	if authChoice == "" && strings.Contains(frontend, "next") {
		authChoice = "supabase"
	}
	switch authChoice {
	case "supabase":
		return supabaseNextScaffold(), nil
	case "none", "":
		return AuthScaffold{Provider: "none"}, nil
	default:
		// Unknown auth choice — drop a contract-only stub so the Coder
		// knows the operator picked one we don't have a recipe for, and
		// must implement it itself.
		return AuthScaffold{
			Provider: authChoice,
			Contract: "Auth provider `" + authChoice + "` is selected but no scaffold recipe shipped; implement signup/login/session-check yourself.",
		}, nil
	}
}

// supabaseNextScaffold returns the canonical Next.js + Supabase Auth
// skeleton. The files are intentionally minimal: a typed client, a server
// helper that reads the cookie session, a middleware that protects /app,
// and a login page. The Coder is expected to add product UI on top —
// these files own the auth contract.
func supabaseNextScaffold() AuthScaffold {
	files := map[string]string{
		"lib/supabase/client.ts": `// Browser-side Supabase client. Reads anon key + URL from
// NEXT_PUBLIC_* env vars; never expose the service-role key to the
// client bundle.
import { createBrowserClient } from '@supabase/ssr';

export function getSupabaseBrowser() {
  return createBrowserClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!,
  );
}
`,
		"lib/supabase/server.ts": `// Server-side Supabase client bound to the per-request cookie store.
// Use this in Server Components, Route Handlers, and Server Actions —
// anything that needs to read the current user's session.
import { cookies } from 'next/headers';
import { createServerClient } from '@supabase/ssr';

export async function getSupabaseServer() {
  const cookieStore = await cookies();
  return createServerClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!,
    {
      cookies: {
        getAll: () => cookieStore.getAll(),
        setAll: (list) => {
          for (const { name, value, options } of list) {
            cookieStore.set(name, value, options);
          }
        },
      },
    },
  );
}

export async function getCurrentUser() {
  const supabase = await getSupabaseServer();
  const { data: { user } } = await supabase.auth.getUser();
  return user;
}
`,
		"middleware.ts": `// Auth middleware: refreshes the Supabase session on every request and
// gates /app/** so only signed-in users see protected pages. Tune the
// matcher to match the routes your app actually needs to protect.
import { NextRequest, NextResponse } from 'next/server';
import { createServerClient } from '@supabase/ssr';

export async function middleware(req: NextRequest) {
  const res = NextResponse.next();
  const supabase = createServerClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!,
    {
      cookies: {
        getAll: () => req.cookies.getAll(),
        setAll: (list) => {
          for (const { name, value, options } of list) {
            res.cookies.set(name, value, options);
          }
        },
      },
    },
  );
  const { data: { user } } = await supabase.auth.getUser();
  if (!user && req.nextUrl.pathname.startsWith('/app')) {
    const url = req.nextUrl.clone();
    url.pathname = '/login';
    url.searchParams.set('next', req.nextUrl.pathname);
    return NextResponse.redirect(url);
  }
  return res;
}

export const config = {
  matcher: ['/app/:path*'],
};
`,
		"app/login/page.tsx": `// Login page. Email + password by default; swap to OAuth providers by
// calling supabase.auth.signInWithOAuth({ provider }) on submit.
'use client';

import { useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { getSupabaseBrowser } from '../../lib/supabase/client';

export default function LoginPage() {
  const router = useRouter();
  const next = useSearchParams().get('next') ?? '/app';
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const supabase = getSupabaseBrowser();
    const { error } = await supabase.auth.signInWithPassword({ email, password });
    if (error) {
      setError(error.message);
      setBusy(false);
      return;
    }
    router.replace(next);
  }

  return (
    <main style={{ maxWidth: 360, margin: '120px auto', fontFamily: 'system-ui' }}>
      <h1>Sign in</h1>
      <form onSubmit={onSubmit}>
        <input type="email" required placeholder="you@example.com"
               value={email} onChange={(e) => setEmail(e.target.value)} />
        <input type="password" required placeholder="password"
               value={password} onChange={(e) => setPassword(e.target.value)} />
        <button type="submit" disabled={busy}>{busy ? 'Signing in…' : 'Sign in'}</button>
        {error && <p style={{ color: 'crimson' }}>{error}</p>}
      </form>
    </main>
  );
}
`,
		"app/auth/signout/route.ts": `// POST /auth/signout — clears the Supabase session cookie and redirects
// back to the marketing root.
import { NextResponse } from 'next/server';
import { getSupabaseServer } from '../../../lib/supabase/server';

export async function POST() {
  const supabase = await getSupabaseServer();
  await supabase.auth.signOut();
  return NextResponse.redirect(new URL('/', process.env.NEXT_PUBLIC_SITE_URL ?? 'http://localhost:3000'));
}
`,
	}

	contract := `Auth scaffold: Supabase Auth on Next.js (app router).

Already provisioned by Ironflyer:
- /lib/supabase/client.ts   → browser-side Supabase client
- /lib/supabase/server.ts   → server-side client + getCurrentUser()
- /middleware.ts            → protects /app/** and refreshes the session
- /app/login/page.tsx       → email/password login form
- /app/auth/signout/route.ts → POST signout endpoint

Required environment variables (already declared on the runtime):
- NEXT_PUBLIC_SUPABASE_URL
- NEXT_PUBLIC_SUPABASE_ANON_KEY
- SUPABASE_SERVICE_ROLE_KEY (server-only, never bundle for client)

Rules for the Coder:
1. Do NOT replace lib/supabase/*, middleware.ts, app/login, app/auth/signout — they are the contract.
2. Use getCurrentUser() (server) or getSupabaseBrowser() (client) when you need the user.
3. New protected routes go under /app/**; the middleware will gate them.
4. Sign-out posts to /auth/signout — link a form, never expose the service-role key in the browser.
`
	return AuthScaffold{Provider: "supabase", Files: files, Contract: contract}
}

// ensureAuth runs the configured scaffolder once per project and upserts
// any generated files. Idempotent: if all scaffold paths already exist in
// the project tree we skip the write so a re-run does not clobber Coder
// edits. The Contract is mirrored into .ironflyer/auth.md so the Coder
// can read it from the project's context like any other artifact.
func (e *Engine) ensureAuth(ctx context.Context, projectID string) {
	if e.authScaffolder == nil {
		return
	}
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return
	}
	scaffold, err := e.authScaffolder.Scaffold(ctx, &proj)
	if err != nil || (len(scaffold.Files) == 0 && scaffold.Contract == "") {
		return
	}
	_, _ = e.projects.Update(projectID, func(p *domain.Project) {
		for path, body := range scaffold.Files {
			// Idempotency: skip a file the Coder has already touched on a
			// prior run so we don't overwrite product code.
			if existing := findFile(p, path); existing != nil && existing.Content != body {
				continue
			}
			writeProjectFile(p, path, body)
		}
		if scaffold.Contract != "" {
			writeProjectFile(p, ".ironflyer/auth.md", scaffold.Contract)
		}
	})
	if scaffold.Provider != "" && scaffold.Provider != "none" {
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRun, Status: StatusDone,
			Message: "auth_scaffolded provider=" + scaffold.Provider,
		})
	}
}

// findFile returns a pointer to the FileNode at `path` or nil. Used by
// ensureAuth to detect Coder-edited scaffolds.
func findFile(p *domain.Project, path string) *domain.FileNode {
	for i := range p.Files {
		if p.Files[i].Path == path {
			return &p.Files[i]
		}
	}
	return nil
}
