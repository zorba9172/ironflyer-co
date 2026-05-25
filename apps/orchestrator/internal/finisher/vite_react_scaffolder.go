// ViteReactScaffolder — pure-React (Vite) SPA skeleton. Ironflyer
// covers Next.js by default, but a large slice of inbound requests
// genuinely want a client-rendered single-page app: dashboards behind
// a CDN, embedded widgets, customer-facing apps that ship to
// Cloudflare Pages / Vercel / Netlify with no server. This pack
// delivers that out of the box.
//
// Detection deliberately avoids stealing Next.js's territory: it only
// triggers when the spec explicitly asks for "spa" / "single-page" /
// "client-side" rendering, or pairs "vite" / "react" with that
// language. Generic "react" alone is NOT enough — Next.js will still
// win the default web-app case.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type ViteReactScaffolder struct{}

func (ViteReactScaffolder) Name() string { return "vite-react" }

func (ViteReactScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	combined := stack + " " + desc

	// Next.js wins when the spec explicitly asks for SSR / server-rendered.
	if strings.Contains(stack, "next") || strings.Contains(stack, "next.js") ||
		strings.Contains(desc, "server-rendered") || strings.Contains(desc, "ssr") {
		return false
	}

	hasSPALanguage := strings.Contains(combined, "spa") ||
		strings.Contains(combined, "single page") ||
		strings.Contains(combined, "single-page") ||
		strings.Contains(combined, "client-side") ||
		strings.Contains(combined, "client side")

	if strings.Contains(stack, "vite") {
		return true
	}
	if strings.Contains(stack, "react") && hasSPALanguage {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "spa") ||
			strings.Contains(body, "single page") ||
			strings.Contains(body, "single-page") {
			return true
		}
	}
	return false
}

func (ViteReactScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	packageJSON := "" +
		"{\n" +
		"  \"name\": \"ironflyer-vite-react\",\n" +
		"  \"private\": true,\n" +
		"  \"version\": \"0.1.0\",\n" +
		"  \"type\": \"module\",\n" +
		"  \"scripts\": {\n" +
		"    \"dev\": \"vite\",\n" +
		"    \"build\": \"tsc -b && vite build\",\n" +
		"    \"preview\": \"vite preview --port 5173\",\n" +
		"    \"test\": \"vitest run\"\n" +
		"  },\n" +
		"  \"dependencies\": {\n" +
		"    \"react\": \"^19.0.0\",\n" +
		"    \"react-dom\": \"^19.0.0\",\n" +
		"    \"react-router-dom\": \"^7.1.1\"\n" +
		"  },\n" +
		"  \"devDependencies\": {\n" +
		"    \"@tailwindcss/vite\": \"^4.0.0\",\n" +
		"    \"@types/react\": \"^19.0.0\",\n" +
		"    \"@types/react-dom\": \"^19.0.0\",\n" +
		"    \"@vitejs/plugin-react\": \"^4.3.4\",\n" +
		"    \"tailwindcss\": \"^4.0.0\",\n" +
		"    \"typescript\": \"^5.7.2\",\n" +
		"    \"vite\": \"^6.0.5\",\n" +
		"    \"vitest\": \"^2.1.8\"\n" +
		"  }\n" +
		"}\n"

	viteConfig := "" +
		"// Vite config — React plugin + Tailwind 4 plugin, plus an @-alias\n" +
		"// to keep deep imports tidy. The dev server listens on :5173 and\n" +
		"// proxies /api requests to the backend declared in\n" +
		"// VITE_API_BASE_URL when running locally.\n" +
		"import { defineConfig } from \"vite\";\n" +
		"import react from \"@vitejs/plugin-react\";\n" +
		"import tailwindcss from \"@tailwindcss/vite\";\n" +
		"import path from \"node:path\";\n" +
		"\n" +
		"export default defineConfig({\n" +
		"  plugins: [react(), tailwindcss()],\n" +
		"  resolve: {\n" +
		"    alias: {\n" +
		"      \"@\": path.resolve(__dirname, \"./src\"),\n" +
		"    },\n" +
		"  },\n" +
		"  server: {\n" +
		"    port: 5173,\n" +
		"    host: true,\n" +
		"  },\n" +
		"});\n"

	indexHTML := "" +
		"<!doctype html>\n" +
		"<html lang=\"en\">\n" +
		"  <head>\n" +
		"    <meta charset=\"utf-8\" />\n" +
		"    <meta name=\"viewport\" content=\"width=device-width,initial-scale=1\" />\n" +
		"    <title>Ironflyer React App</title>\n" +
		"  </head>\n" +
		"  <body>\n" +
		"    <div id=\"root\"></div>\n" +
		"    <script type=\"module\" src=\"/src/main.tsx\"></script>\n" +
		"  </body>\n" +
		"</html>\n"

	mainTSX := "" +
		"// Application bootstrap. StrictMode catches double-effects in\n" +
		"// development; BrowserRouter owns history so every route lives\n" +
		"// inside <App />.\n" +
		"import React from \"react\";\n" +
		"import ReactDOM from \"react-dom/client\";\n" +
		"import { BrowserRouter } from \"react-router-dom\";\n" +
		"import App from \"./App\";\n" +
		"import \"./tailwind.css\";\n" +
		"import \"./styles/tokens.css\";\n" +
		"\n" +
		"const rootEl = document.getElementById(\"root\");\n" +
		"if (!rootEl) throw new Error(\"#root not found in index.html\");\n" +
		"\n" +
		"ReactDOM.createRoot(rootEl).render(\n" +
		"  <React.StrictMode>\n" +
		"    <BrowserRouter>\n" +
		"      <App />\n" +
		"    </BrowserRouter>\n" +
		"  </React.StrictMode>,\n" +
		");\n"

	appTSX := "" +
		"// Top-level routing. Keep public routes (\"/\", \"/login\") flat;\n" +
		"// the authenticated app surface lives under /app/* and can\n" +
		"// expand with nested routes without touching this file.\n" +
		"import { Routes, Route, Navigate } from \"react-router-dom\";\n" +
		"import Home from \"./routes/Home\";\n" +
		"import Login from \"./routes/Login\";\n" +
		"import AppShell from \"./routes/App\";\n" +
		"\n" +
		"export default function App() {\n" +
		"  return (\n" +
		"    <Routes>\n" +
		"      <Route path=\"/\" element={<Home />} />\n" +
		"      <Route path=\"/login\" element={<Login />} />\n" +
		"      <Route path=\"/app/*\" element={<AppShell />} />\n" +
		"      <Route path=\"*\" element={<Navigate to=\"/\" replace />} />\n" +
		"    </Routes>\n" +
		"  );\n" +
		"}\n"

	homeTSX := "" +
		"// Public landing page. Replace this with marketing copy + a CTA\n" +
		"// once the brand voice lands.\n" +
		"import { Link } from \"react-router-dom\";\n" +
		"\n" +
		"export default function Home() {\n" +
		"  return (\n" +
		"    <main className=\"min-h-screen grid place-items-center bg-neutral-950 text-neutral-100\">\n" +
		"      <div className=\"text-center space-y-6\">\n" +
		"        <h1 className=\"text-4xl font-semibold tracking-tight\">Ironflyer React App</h1>\n" +
		"        <p className=\"text-neutral-400 max-w-md\">\n" +
		"          Pure React + Vite. Edit <code>src/routes/Home.tsx</code> to make it yours.\n" +
		"        </p>\n" +
		"        <div className=\"flex gap-3 justify-center\">\n" +
		"          <Link to=\"/login\" className=\"px-4 py-2 rounded-md bg-lime-400 text-neutral-950 font-medium\">Login</Link>\n" +
		"          <Link to=\"/app\" className=\"px-4 py-2 rounded-md border border-neutral-700\">Open App</Link>\n" +
		"        </div>\n" +
		"      </div>\n" +
		"    </main>\n" +
		"  );\n" +
		"}\n"

	loginTSX := "" +
		"// Placeholder login form. Wire to the auth scaffold's API once it\n" +
		"// lands — the fetch wrapper in src/lib/api.ts is the canonical\n" +
		"// way to talk to the backend.\n" +
		"import { useState } from \"react\";\n" +
		"import { useNavigate } from \"react-router-dom\";\n" +
		"import { api } from \"../lib/api\";\n" +
		"\n" +
		"export default function Login() {\n" +
		"  const [email, setEmail] = useState(\"\");\n" +
		"  const [password, setPassword] = useState(\"\");\n" +
		"  const [error, setError] = useState<string | null>(null);\n" +
		"  const nav = useNavigate();\n" +
		"\n" +
		"  async function onSubmit(e: React.FormEvent) {\n" +
		"    e.preventDefault();\n" +
		"    setError(null);\n" +
		"    try {\n" +
		"      await api(\"/auth/login\", { method: \"POST\", body: JSON.stringify({ email, password }) });\n" +
		"      nav(\"/app\");\n" +
		"    } catch (err: unknown) {\n" +
		"      setError(err instanceof Error ? err.message : \"login failed\");\n" +
		"    }\n" +
		"  }\n" +
		"\n" +
		"  return (\n" +
		"    <main className=\"min-h-screen grid place-items-center bg-neutral-950 text-neutral-100\">\n" +
		"      <form onSubmit={onSubmit} className=\"w-full max-w-sm space-y-4 p-6 rounded-lg border border-neutral-800\">\n" +
		"        <h1 className=\"text-2xl font-semibold\">Login</h1>\n" +
		"        <input className=\"w-full px-3 py-2 rounded bg-neutral-900 border border-neutral-800\"\n" +
		"          type=\"email\" placeholder=\"email\" value={email} onChange={(e) => setEmail(e.target.value)} />\n" +
		"        <input className=\"w-full px-3 py-2 rounded bg-neutral-900 border border-neutral-800\"\n" +
		"          type=\"password\" placeholder=\"password\" value={password} onChange={(e) => setPassword(e.target.value)} />\n" +
		"        {error && <p className=\"text-red-400 text-sm\">{error}</p>}\n" +
		"        <button className=\"w-full py-2 rounded bg-lime-400 text-neutral-950 font-medium\" type=\"submit\">Sign in</button>\n" +
		"      </form>\n" +
		"    </main>\n" +
		"  );\n" +
		"}\n"

	appShellTSX := "" +
		"// Authenticated app shell. Nested routes live under /app/* — add\n" +
		"// them here as the surface grows.\n" +
		"import { Routes, Route, Link } from \"react-router-dom\";\n" +
		"\n" +
		"function Dashboard() {\n" +
		"  return <section className=\"p-8\"><h2 className=\"text-xl font-semibold\">Dashboard</h2></section>;\n" +
		"}\n" +
		"\n" +
		"export default function AppShell() {\n" +
		"  return (\n" +
		"    <div className=\"min-h-screen bg-neutral-950 text-neutral-100\">\n" +
		"      <header className=\"border-b border-neutral-800 px-6 py-3 flex items-center justify-between\">\n" +
		"        <Link to=\"/\" className=\"font-semibold\">Ironflyer</Link>\n" +
		"        <nav className=\"text-sm text-neutral-400\">\n" +
		"          <Link to=\"/app\" className=\"hover:text-neutral-100\">Dashboard</Link>\n" +
		"        </nav>\n" +
		"      </header>\n" +
		"      <Routes>\n" +
		"        <Route index element={<Dashboard />} />\n" +
		"      </Routes>\n" +
		"    </div>\n" +
		"  );\n" +
		"}\n"

	libAPI := "" +
		"// Tiny fetch wrapper. Centralizes base URL + JSON handling so\n" +
		"// every call site stays one line. Throws on non-2xx so callers\n" +
		"// can rely on try/catch instead of branching on response.ok.\n" +
		"const BASE = import.meta.env.VITE_API_BASE_URL ?? \"\";\n" +
		"\n" +
		"export async function api<T = unknown>(path: string, init: RequestInit = {}): Promise<T> {\n" +
		"  const headers = new Headers(init.headers);\n" +
		"  if (!headers.has(\"Content-Type\") && init.body) {\n" +
		"    headers.set(\"Content-Type\", \"application/json\");\n" +
		"  }\n" +
		"  const res = await fetch(BASE + path, { credentials: \"include\", ...init, headers });\n" +
		"  if (!res.ok) {\n" +
		"    const text = await res.text().catch(() => res.statusText);\n" +
		"    throw new Error(\"api \" + res.status + \" \" + text);\n" +
		"  }\n" +
		"  const ct = res.headers.get(\"content-type\") ?? \"\";\n" +
		"  if (ct.includes(\"application/json\")) return (await res.json()) as T;\n" +
		"  return (await res.text()) as unknown as T;\n" +
		"}\n"

	tsconfig := "" +
		"{\n" +
		"  \"compilerOptions\": {\n" +
		"    \"target\": \"ES2022\",\n" +
		"    \"useDefineForClassFields\": true,\n" +
		"    \"lib\": [\"ES2022\", \"DOM\", \"DOM.Iterable\"],\n" +
		"    \"module\": \"ESNext\",\n" +
		"    \"skipLibCheck\": true,\n" +
		"    \"moduleResolution\": \"bundler\",\n" +
		"    \"allowImportingTsExtensions\": false,\n" +
		"    \"resolveJsonModule\": true,\n" +
		"    \"isolatedModules\": true,\n" +
		"    \"noEmit\": true,\n" +
		"    \"jsx\": \"react-jsx\",\n" +
		"    \"strict\": true,\n" +
		"    \"noUnusedLocals\": true,\n" +
		"    \"noUnusedParameters\": true,\n" +
		"    \"noFallthroughCasesInSwitch\": true,\n" +
		"    \"baseUrl\": \".\",\n" +
		"    \"paths\": { \"@/*\": [\"src/*\"] }\n" +
		"  },\n" +
		"  \"include\": [\"src\"],\n" +
		"  \"references\": [{ \"path\": \"./tsconfig.node.json\" }]\n" +
		"}\n"

	tsconfigNode := "" +
		"{\n" +
		"  \"compilerOptions\": {\n" +
		"    \"composite\": true,\n" +
		"    \"skipLibCheck\": true,\n" +
		"    \"module\": \"ESNext\",\n" +
		"    \"moduleResolution\": \"bundler\",\n" +
		"    \"allowSyntheticDefaultImports\": true,\n" +
		"    \"strict\": true\n" +
		"  },\n" +
		"  \"include\": [\"vite.config.ts\"]\n" +
		"}\n"

	tailwindCSS := "" +
		"/* Tailwind 4 entrypoint. The plugin in vite.config.ts handles\n" +
		"   discovery — no tailwind.config.js required for the default\n" +
		"   utility set. Add @theme blocks here to customize. */\n" +
		"@import \"tailwindcss\";\n"

	tokensCSS := "" +
		"/* Design tokens — mirrors packages/design-tokens. Importing the\n" +
		"   package directly is fine too once the workspace is wired up;\n" +
		"   this file keeps the SPA buildable as a standalone artifact. */\n" +
		":root {\n" +
		"  --color-bg: #0d0e0f;\n" +
		"  --color-surface: #161718;\n" +
		"  --color-border: #2a2b2d;\n" +
		"  --color-text: #f5f5f4;\n" +
		"  --color-text-muted: #a3a3a3;\n" +
		"  --color-accent-lime: #c7ff00;\n" +
		"  --radius-card: 8px;\n" +
		"  --font-mono: ui-monospace, SFMono-Regular, Menlo, monospace;\n" +
		"}\n"

	gitignore := "" +
		"node_modules\n" +
		"dist\n" +
		".env\n" +
		".env.local\n" +
		".env.*.local\n" +
		"*.log\n" +
		".DS_Store\n" +
		".vite\n"

	envExample := "" +
		"# Base URL the SPA hits for API calls. Leave empty to use a same-origin\n" +
		"# /api path proxied by Cloudflare / Vercel / Netlify edge functions.\n" +
		"VITE_API_BASE_URL=\n"

	files := map[string]string{
		"package.json":          packageJSON,
		"vite.config.ts":        viteConfig,
		"index.html":            indexHTML,
		"src/main.tsx":          mainTSX,
		"src/App.tsx":           appTSX,
		"src/routes/Home.tsx":   homeTSX,
		"src/routes/Login.tsx":  loginTSX,
		"src/routes/App.tsx":    appShellTSX,
		"src/lib/api.ts":        libAPI,
		"tsconfig.json":         tsconfig,
		"tsconfig.node.json":    tsconfigNode,
		"src/tailwind.css":      tailwindCSS,
		"src/styles/tokens.css": tokensCSS,
		".gitignore":            gitignore,
		".env.example":          envExample,
	}

	contract := "Vite + React SPA scaffold: React 19 + react-router-dom 7 + Tailwind 4.\n" +
		"\n" +
		"Already provisioned:\n" +
		"- package.json            vite 6, react 19, react-router-dom 7, tailwindcss 4\n" +
		"- vite.config.ts          @vitejs/plugin-react + @tailwindcss/vite + @-alias\n" +
		"- index.html              root mount + module script\n" +
		"- src/main.tsx            StrictMode + BrowserRouter bootstrap\n" +
		"- src/App.tsx             top-level routes (/, /login, /app/*)\n" +
		"- src/routes/Home.tsx     public landing\n" +
		"- src/routes/Login.tsx    placeholder login form\n" +
		"- src/routes/App.tsx      authenticated app shell\n" +
		"- src/lib/api.ts          fetch wrapper reading VITE_API_BASE_URL\n" +
		"- src/tailwind.css        Tailwind 4 entrypoint\n" +
		"- src/styles/tokens.css   design-token CSS variables\n" +
		"- tsconfig.json + tsconfig.node.json\n" +
		"- .gitignore + .env.example\n" +
		"\n" +
		"Run locally with `npm i && npm run dev` — Vite serves the app at\n" +
		"http://localhost:5173 with hot-module-reload. Build with\n" +
		"`npm run build` (produces a static bundle in dist/), preview with\n" +
		"`npm run preview`. Deploy the dist/ folder to any static host\n" +
		"(Cloudflare Pages, Vercel, Netlify, S3 + CloudFront, GitHub Pages).\n" +
		"\n" +
		"This scaffold is pure client-side. Pair it with the Hono or\n" +
		"Go-HTTP scaffolders for the API layer, or call any existing\n" +
		"backend through the fetch wrapper in src/lib/api.ts. If you need\n" +
		"server-rendering or per-route data loaders, use the Next.js\n" +
		"scaffold instead — Vite SPA is the wrong shape for SSR.\n"

	return DomainScaffold{Files: files, Contract: contract}, nil
}
