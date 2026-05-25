/**
 * Quickstart: sign in, list projects, propose a patch, exit.
 *
 *   npx tsx examples/quickstart.ts
 *
 * Env:
 *   IRONFLYER_ENDPOINT   — e.g. https://api.ironflyer.dev (default: http://localhost:8080)
 *   IRONFLYER_EMAIL      — account email
 *   IRONFLYER_PASSWORD   — account password
 */

import { Ironflyer, IronflyerError } from '../src/index.js';

async function main() {
  const endpoint = process.env.IRONFLYER_ENDPOINT ?? 'http://localhost:8080';
  const email = process.env.IRONFLYER_EMAIL;
  const password = process.env.IRONFLYER_PASSWORD;

  if (!email || !password) {
    console.error('Set IRONFLYER_EMAIL and IRONFLYER_PASSWORD before running.');
    process.exit(1);
  }

  const ifr = new Ironflyer({ endpoint });

  try {
    const session = await ifr.signIn({ email, password });
    console.log('signed in as', session.user.email);

    const projects = await ifr.projects();
    console.log(`found ${projects.length} project(s)`);
    if (projects.length === 0) {
      console.log('create a project from the web UI first, then re-run.');
      return;
    }

    const target = projects[0];
    console.log('targeting project', target.id, target.name);

    const patch = await ifr.proposePatch({
      projectId: target.id,
      title: 'SDK quickstart patch',
      summary: 'Demonstrates @ironflyer/sdk',
      author: 'sdk-quickstart',
      changes: [
        {
          op: 'CREATE',
          path: 'docs/sdk-hello.md',
          content: '# Hello from @ironflyer/sdk\n',
        },
      ],
    });
    console.log('proposed patch', patch.id, 'status', patch.status);
  } catch (err) {
    if (err instanceof IronflyerError) {
      console.error('Ironflyer error:', err.message);
      console.error('cause:', err.cause);
    } else {
      console.error(err);
    }
    process.exitCode = 1;
  } finally {
    ifr.dispose();
  }
}

void main();
