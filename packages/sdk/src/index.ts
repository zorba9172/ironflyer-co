export * from './types.js';
export { IronflyerError, type TokenProvider, type TransportConfig } from './http.js';
export { OrchestratorClient } from './orchestrator.js';
export { RuntimeClient } from './runtime.js';

import { OrchestratorClient } from './orchestrator.js';
import { RuntimeClient } from './runtime.js';
import type { TokenProvider } from './http.js';

export interface IronflyerConfig {
  /** Base URL of the orchestrator HTTP API. */
  orchestratorUrl: string;
  /** Base URL of the workspace runtime HTTP API. */
  runtimeUrl?: string;
  /** Bearer-token getter shared by both clients. */
  getToken?: TokenProvider;
  /** Optional fetch override (Node 18 / Bun / Deno / browser). */
  fetch?: typeof fetch;
  /** Optional default headers merged into every request. */
  headers?: Record<string, string>;
}

/**
 * ironflyer is the convenience factory: one config, one auth source, both
 * clients. Use the clients directly when you need them in different contexts
 * with different tokens.
 *
 *     const ifc = ironflyer({
 *       orchestratorUrl: process.env.IRONFLYER_ORCHESTRATOR_URL!,
 *       runtimeUrl: process.env.IRONFLYER_RUNTIME_URL!,
 *       getToken: () => localStorage.getItem('ironflyer.token'),
 *     });
 *     await ifc.orchestrator.listProjects();
 *     await ifc.runtime.exec(wsId, { shell: 'go test ./...' });
 */
export function ironflyer(cfg: IronflyerConfig) {
  const shared = { getToken: cfg.getToken, fetch: cfg.fetch, headers: cfg.headers };
  return {
    orchestrator: new OrchestratorClient({ baseUrl: cfg.orchestratorUrl, ...shared }),
    runtime: cfg.runtimeUrl
      ? new RuntimeClient({ baseUrl: cfg.runtimeUrl, ...shared })
      : undefined,
  };
}
