// /docs/cli — placeholder while Agent P ships the CLI.

import type { Metadata } from 'next';
import { DocPage } from '../../../components/docs/DocPage';
import { CodeBlock } from '../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'CLI',
  description: 'The Ironflyer command-line — shipping next. Drives finisher runs from your terminal.',
  openGraph: { title: 'CLI · Ironflyer', description: 'Coming soon — the terminal cockpit for the finisher loop.', images: ['/opengraph-image'] },
};

const toc = [
  { id: 'status', label: 'Status' },
  { id: 'preview', label: 'Preview of the surface' },
  { id: 'follow', label: 'Follow along' },
];

export default function CLIPage() {
  return (
    <DocPage
      eyebrow="Clients"
      title="Ironflyer CLI"
      description="Run the finisher, watch gates, and ship deploys without leaving the terminal."
      toc={toc}
    >
      <h2 id="status">Status</h2>
      <p>
        <strong>Coming soon.</strong> The Ironflyer CLI is shipping next. Track progress in the
        <a href="https://github.com/zorba9172/ironflyer"> GitHub repo</a>; the first release tag will
        be announced on the <a href="/changelog">changelog</a> and the <a href="/blog">blog</a>.
      </p>

      <h2 id="preview">Preview of the surface</h2>
      <p>
        We are designing the CLI to mirror the SDK surface as closely as we can, so you can swap
        between scripts and terminal use without re-learning anything. Planned commands:
      </p>
      <CodeBlock language="bash">{`ironflyer login                    # opens browser, stashes JWT in OS keychain
ironflyer projects                 # lists projects
ironflyer projects create "name"   # creates a new project
ironflyer run                      # runs the finisher on the pinned project
ironflyer logs --follow            # streams gate events
ironflyer deploy fly               # one-click deploy
ironflyer budget                   # prints your spend + plan cap`}</CodeBlock>

      <h2 id="follow">Follow along</h2>
      <p>
        We will update this page with the install instructions and a full reference the moment the CLI
        cuts a 0.1 release. Want early access? <a href="mailto:hello@ironflyer.dev">Email us</a> and we
        will add you to the private beta.
      </p>
    </DocPage>
  );
}
