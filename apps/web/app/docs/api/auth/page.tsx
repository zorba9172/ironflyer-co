// /docs/api/auth — exhaustive list of the auth surface. Mirrors api.go.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Auth API',
  description: 'Signup, login, /auth/me, and the GitHub OAuth flow.',
  openGraph: { title: 'Auth API · Ironflyer', description: 'Issue JWTs, identify the caller, link GitHub.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'overview', label: 'Overview' },
  { id: 'signup', label: 'POST /auth/signup' },
  { id: 'login', label: 'POST /auth/login' },
  { id: 'me', label: 'GET /auth/me' },
  { id: 'github-login', label: 'GET /auth/github/login/start' },
  { id: 'github-link', label: 'GET /auth/github/start' },
  { id: 'github-callback', label: 'GET /auth/github/callback' },
];

export default function AuthAPIPage() {
  return (
    <DocPage
      eyebrow="API Reference"
      title="Auth"
      description="Three local endpoints plus the GitHub OAuth flow. All JWTs are HS256; tokens go in Authorization: Bearer headers."
      toc={toc}
    >
      <h2 id="overview">Overview</h2>
      <p>
        Auth lives at <code>/auth</code>. <code>/signup</code> and <code>/login</code> are public;
        <code> /me</code> is protected. All authenticated endpoints expect a Bearer JWT obtained from
        signup, login, or the GitHub OAuth callback. Token expiry is 30 days. There is no refresh
        endpoint yet — clients re-login when the token expires.
      </p>

      <h2 id="signup">POST /auth/signup</h2>
      <p>Creates a user, returns the user and a JWT. Rate-limited per IP (signup ratelimiter).</p>
      <CodeBlock language="json">{`{
  "email": "you@example.com",
  "name": "You",
  "password": "at least 8 chars"
}`}</CodeBlock>
      <CodeBlock language="json">{`HTTP/1.1 201 Created
{
  "user":  { "id": "u_…", "email": "…", "name": "…", "plan": "free" },
  "token": "eyJhbGciOi…"
}`}</CodeBlock>
      <CodeBlock language="bash">{`curl -X POST https://api.ironflyer.dev/auth/signup \\
  -H "Content-Type: application/json" \\
  -d '{"email":"you@example.com","name":"You","password":"hunter22-very-long"}'`}</CodeBlock>

      <h2 id="login">POST /auth/login</h2>
      <p>Exchanges email + password for a JWT. Returns 401 on bad credentials.</p>
      <CodeBlock language="bash">{`curl -X POST https://api.ironflyer.dev/auth/login \\
  -H "Content-Type: application/json" \\
  -d '{"email":"you@example.com","password":"hunter22-very-long"}'`}</CodeBlock>

      <h2 id="me">GET /auth/me</h2>
      <p>Returns the user behind the bearer token, or 401.</p>
      <CodeBlock language="bash">{`curl https://api.ironflyer.dev/auth/me \\
  -H "Authorization: Bearer $TOKEN"`}</CodeBlock>

      <h2 id="github-login">GET /auth/github/login/start</h2>
      <p>Public. Redirects an anonymous visitor to GitHub for sign-in / signup. The callback issues a JWT.</p>

      <h2 id="github-link">GET /auth/github/start</h2>
      <p>
        Authenticated. Starts the <em>link</em> flow that connects an existing account to GitHub.
        Accepts <code>?redirect=true</code> to redirect the browser directly; the default returns
        JSON with <code>authUrl</code> + <code>state</code> for SPAs that prefer to open the window
        themselves.
      </p>

      <h2 id="github-callback">GET /auth/github/callback</h2>
      <p>
        Public. GitHub redirects here after the user consents. The handler branches on the flow mode:
        in LINK mode the integration is persisted, in LOGIN mode a JWT is issued and handed back to
        the SPA via the URL fragment (<code>#github=login&amp;token=…</code>) so it never lands in a
        server log.
      </p>
    </DocPage>
  );
}
