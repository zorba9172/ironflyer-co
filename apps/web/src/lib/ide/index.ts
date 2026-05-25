// ide — small helper for opening the openvscode-server iframe.
//
// openvscode-server is started with --server-base-path /ide so every
// asset and WS URL is namespaced under /ide. Two ways to embed:
//
//   1. **Same-origin proxy** (preferred for production): a real
//      reverse proxy (nginx / Caddy / cloudflared) at the web origin
//      forwards /ide/* to the per-user openvscode container, with
//      WebSocket upgrade support. Cookies, clipboard, popups, and
//      X-Frame-Options all behave naturally on the same origin.
//   2. **Direct cross-origin** (default in dev): the iframe points at
//      http://localhost:3030/ide. Next.js's dev rewrite layer does not
//      proxy WS upgrades reliably, and VS Code in the browser needs WS
//      for its extension host. Cross-origin is fine — the only
//      friction is a clipboard permission prompt on first use.
//
// NEXT_PUBLIC_OPENVSCODE_URL is the single switch. Set to "/ide" in
// production behind a real reverse proxy; leave at
// http://localhost:3030/ide in dev.
//
// Today the workspace folder is always /home/workspace. Per-project
// sandboxes are on the roadmap — the helper accepts a projectID and
// forwards it as a query parameter for forward compatibility.

export const OPENVSCODE_DEFAULT_PORT = 3030;
export const OPENVSCODE_DEFAULT_URL = `http://localhost:${OPENVSCODE_DEFAULT_PORT}/ide`;
export const OPENVSCODE_WORKSPACE_FOLDER = "/home/workspace";

// resolveOpenvscodeBase — returns the configured base URL with no
// trailing slash. Defaults to the direct dev URL so the iframe works
// out of the box without a reverse proxy in front.
export function resolveOpenvscodeBase(): string {
  const raw =
    (typeof process !== "undefined" &&
      process.env?.NEXT_PUBLIC_OPENVSCODE_URL) ||
    OPENVSCODE_DEFAULT_URL;
  return raw.replace(/\/+$/, "");
}

// getOpenvscodeUrl — builds the iframe src.
export function getOpenvscodeUrl(projectID?: string): string {
  const base = resolveOpenvscodeBase();
  const params = new URLSearchParams();
  params.set("folder", OPENVSCODE_WORKSPACE_FOLDER);
  if (projectID && projectID.trim()) {
    // NOTE: openvscode does not yet read this — the parameter is wired
    // through so the Go side can opt in without a client change.
    params.set("projectID", projectID.trim());
  }
  return `${base}/?${params.toString()}`;
}
