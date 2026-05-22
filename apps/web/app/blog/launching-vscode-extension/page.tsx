// /blog/launching-vscode-extension — story behind the 0.3 extension release.

import type { Metadata } from 'next';
import { BlogPost } from '../../../components/docs/BlogPost';

export const metadata: Metadata = {
  title: 'Shipping the VSCode extension we wanted to use — Ironflyer',
  description: 'Behind the 0.3 release: live preview in a webview, a patches tree with a real diff editor, and a quick action for every diagnostic.',
  openGraph: {
    title: 'Shipping the VSCode extension we wanted to use',
    description: 'The story behind the 0.3 release.',
    images: ['/opengraph-image'],
  },
};

export default function LaunchingVSCodeExtensionPost() {
  return (
    <BlogPost
      title="Shipping the VSCode extension we wanted to use."
      subtitle="Behind the 0.3 release: live preview in a webview, a patches tree with a real diff editor, and a quick action for every diagnostic."
      tag="Tooling"
      date="2026-03-05"
      gradient="linear-gradient(135deg, #78dbff 0%, #671dfc 100%)"
    >
      <p>
        We use Ironflyer to build Ironflyer. The dashboard is great when we are doing planning work
        or kicking off a long finisher run, but most of the time we are heads-down in VSCode and the
        last thing we want is to alt-tab to a browser. So the VSCode extension is not a marketing
        afterthought — it is the surface we use the most. Version 0.3 is the first release we genuinely
        prefer to the browser.
      </p>

      <h2>The constraint we set</h2>
      <p>
        The extension is a <strong>thin client</strong>. Every AI call, every patch, every gate, every
        provider token sits on the orchestrator. The extension stores exactly one piece of state — the
        JWT in <code>SecretStorage</code>. We picked that boundary because the alternative is a slow
        slide into a fat client where the same business logic ships in three places and they drift.
      </p>
      <p>
        The extension talks to the orchestrator with the same SDK the web app uses, the same SSE feed,
        the same patch endpoints. When we add a new gate to the orchestrator, the extension renders it
        automatically because the gate list comes back over the wire.
      </p>

      <h2>Live preview, inside a webview</h2>
      <p>
        The headline 0.3 feature is the live preview, rendered inside a VSCode webview. The preview
        proxy uses signed <code>?t=…</code> tokens because webviews cannot set auth headers. The token
        is bound to a workspace + port + TTL; it rotates every 24 hours.
      </p>
      <p>
        We added three viewport presets — mobile, tablet, desktop — because the muscle memory of
        switching them on the dashboard turned out to be one of the most-used interactions, and we did
        not want to lose it. The webview keeps the iframe alive across editor focus changes, so
        switching between the preview tab and a file tab does not restart the dev server.
      </p>

      <h2>Patches that open in VSCode’s diff editor</h2>
      <p>
        Patches were the most fun feature to ship. The orchestrator stores each patch as a list of
        <code> FileChange</code>s — op, path, content. The extension turns each change into a virtual
        document URI and hands the pair to VSCode’s built-in <code>vscode.diff</code> command. The
        result is the native side-by-side diff editor — keyboard navigation, inline diff actions,
        everything. Hitting the green checkmark calls
        <code> POST /patches/&#123;id&#125;/apply</code> on the orchestrator; the runtime applier
        mirrors the change into the sandbox; the preview reloads.
      </p>

      <h2>Ask Ironflyer to fix</h2>
      <p>
        VSCode’s <code>languages.registerCodeActionsProvider</code> made one of our favourite
        interactions almost trivial. Every editor diagnostic — TypeScript, ESLint, Go, anything that
        emits diagnostics — gets a quick action titled <em>Ask Ironflyer to fix</em>. Clicking it
        bundles the message, the code, and the surrounding snippet, opens the chat for the pinned
        project, and routes the request to the <code>coder</code> agent.
      </p>
      <p>
        The first time we shipped this internally, three of us realised we had not opened the dashboard
        in two days. That was the signal that the extension was on the right track.
      </p>

      <h2>Status bar cockpit</h2>
      <p>
        VSCode’s status bar is undervalued. We put four things on it: the pinned project name, the
        last gate status (a coloured dot), the budget remaining for the month, and a one-tap Run
        button. Each piece is a click target — pinned project opens the picker, gate status opens the
        finisher gates view, budget opens the budget dashboard, Run kicks off a finisher run. Total
        cost on the status bar real estate: about 220 pixels.
      </p>

      <h2>What is next</h2>
      <p>
        0.4 will bring inline patch suggestions — the orchestrator already has the patch, we just want
        to surface them as inline ghost text in the editor like a normal completion. 0.5 will bring a
        terminal that talks straight to the workspace runtime’s PTY socket so you can attach to a
        sandbox directly from VSCode. The roadmap is on the <a href="/changelog">changelog</a>; the
        repo is on <a href="https://github.com/zorba9172/ironflyer">GitHub</a>.
      </p>
    </BlogPost>
  );
}
