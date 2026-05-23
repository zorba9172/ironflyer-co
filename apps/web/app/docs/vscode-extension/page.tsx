// /docs/vscode-extension — public-facing companion to the extension README.

import type { Metadata } from 'next';
import { DocPage } from '../../../components/docs/DocPage';
import { CodeBlock } from '../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'VSCode Extension',
  description: 'Use Ironflyer without leaving your editor — chat, gates, patches, live preview.',
  openGraph: { title: 'VSCode Extension · Ironflyer', description: 'The thin client that puts the finisher loop inside VSCode.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'what-it-is', label: 'What it is' },
  { id: 'install', label: 'Install' },
  { id: 'sign-in', label: 'Sign in' },
  { id: 'features', label: 'Features' },
  { id: 'commands', label: 'Commands' },
  { id: 'settings', label: 'Settings' },
];

export default function VSCodeExtensionPage() {
  return (
    <DocPage
      eyebrow="Clients"
      title="VSCode Extension"
      description="A thin client for the platform. Auth, AI calls, and patch state live on the server; the extension renders the loop and routes your input."
      toc={toc}
    >
      <h2 id="what-it-is">What it is</h2>
      <p>
        The Ironflyer VSCode extension is the delightful editor surface for the AI Product Finisher.
        Pin a project, hit Run, watch the nine gates light up. Patches arrive as diffs you can review
        side-by-side; the live preview renders inside a webview with mobile / tablet / desktop presets;
        every editor diagnostic gets an <em>Ask Ironflyer to fix</em> quick action.
      </p>

      <h2 id="install">Install</h2>
      <CodeBlock language="bash">{`# From the VS Code marketplace
ext install ironflyer.ironflyer

# Or sideload a release vsix
code --install-extension ironflyer-0.3.0.vsix`}</CodeBlock>
      <p>Cursor, VSCodium, Theia, and any Open VSX-compatible editor are supported via the Open VSX listing.</p>

      <h2 id="sign-in">Sign in</h2>
      <p>
        Run <code>Ironflyer: Sign In</code>. The web app opens with a callback URL pointing back at the
        extension; after auth the orchestrator redirects to
        <code> vscode://ironflyer.ironflyer/auth?token=…</code> and the extension stashes the JWT in
        VSCode <code>SecretStorage</code>. The token never enters <code>settings.json</code>.
      </p>

      <h2 id="features">Features</h2>
      <ul>
        <li><strong>Projects sidebar</strong> — every Ironflyer project you own, one click away.</li>
        <li><strong>Chat panel</strong> — streams the orchestrator with the same role + effort dial as the web app.</li>
        <li><strong>Live preview</strong> — sandboxed iframe rendered inside a webview, with viewport presets.</li>
        <li><strong>Finisher gates view</strong> — status colors, last-updated timestamps, drill-down into issues, re-run a single gate.</li>
        <li><strong>Patches view</strong> — open any change in VSCode’s diff editor; apply through the orchestrator.</li>
        <li><strong>Run output channel + toast actions</strong> — <em>gate_failed</em>, <em>run_complete</em>, <em>patch_proposed</em> surface as notifications.</li>
        <li><strong>Ask Ironflyer to fix</strong> — every diagnostic gets a quick action that bundles message + code + snippet.</li>
        <li><strong>Status bar cockpit</strong> — pinned project, last gate status, budget remaining, one-tap Run.</li>
      </ul>

      <h2 id="commands">Commands</h2>
      <p>Hit ⌘⇧P / Ctrl+⇧P and search for <em>Ironflyer</em>:</p>
      <ul>
        <li><code>Ironflyer: Sign In</code> / <code>Sign Out</code></li>
        <li><code>Ironflyer: Pin Project</code></li>
        <li><code>Ironflyer: Run Finisher</code> / <code>Re-run Gate</code></li>
        <li><code>Ironflyer: Open Live Preview</code></li>
        <li><code>Ironflyer: Open Chat for Pinned Project</code></li>
      </ul>

      <h2 id="settings">Settings</h2>
      <p>The extension keeps its config surface tiny on purpose:</p>
      <ul>
        <li><code>ironflyer.orchestratorUrl</code> — defaults to the public API.</li>
        <li><code>ironflyer.runtimeUrl</code> — defaults to the public runtime.</li>
        <li><code>ironflyer.preview.defaultViewport</code> — <code>mobile</code> | <code>tablet</code> | <code>desktop</code>.</li>
      </ul>
    </DocPage>
  );
}
