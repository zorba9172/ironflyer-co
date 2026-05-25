/**
 * Streaming example: open runProject and print events as they arrive.
 *
 *   npx tsx examples/streaming.ts <projectId>
 *
 * Env:
 *   IRONFLYER_ENDPOINT — defaults to http://localhost:8080
 *   IRONFLYER_TOKEN    — bearer JWT (skip signIn for brevity)
 */

import { Ironflyer } from '../src/index.js';

async function main() {
  const projectId = process.argv[2];
  if (!projectId) {
    console.error('usage: tsx examples/streaming.ts <projectId>');
    process.exit(1);
  }

  const endpoint = process.env.IRONFLYER_ENDPOINT ?? 'http://localhost:8080';
  const token = process.env.IRONFLYER_TOKEN;
  if (!token) {
    console.error('IRONFLYER_TOKEN is required for the streaming example.');
    process.exit(1);
  }

  // On Node 18-21 you'd need to pass webSocketImpl. Node 22+ has it native.
  const ifr = new Ironflyer({ endpoint, token });

  console.log(`streaming runProject for ${projectId}`);
  try {
    for await (const evt of ifr.runProject(projectId)) {
      switch (evt.__typename) {
        case 'RunGateEvent':
          console.log(`[gate] ${evt.gate} → ${evt.status}${evt.gateMessage ? ' — ' + evt.gateMessage : ''}`);
          break;
        case 'RunExecutionEvent':
          console.log(`[exec] ${JSON.stringify(evt.payload)}`);
          break;
        case 'RunDoneEvent':
          console.log(`[done] ok=${evt.ok}`);
          return;
        case 'RunErrorEvent':
          console.error(`[error] ${evt.code}: ${evt.message}`);
          return;
        default:
          console.log('[unknown]', evt);
      }
    }
  } finally {
    ifr.dispose();
  }
}

void main();
