// ide — small helper for opening the openvscode-server iframe.
//
// openvscode-server runs as the `openvscode` service in
// infra/compose/docker-compose.dev.yml under the `--profile ide`
// profile, exposed at http://localhost:3030 by default. The browser-
// facing URL is configured through `NEXT_PUBLIC_OPENVSCODE_URL` so the
// production deployment (which fronts openvscode behind a TLS
// hostname) can point the Studio at the right origin without touching
// component code.
//
// Today the openvscode workspace folder is always `/home/workspace`
// inside the container. Per-project scoping (one workspace per
// Ironflyer project) is on the roadmap — the helper accepts a
// projectID argument so the call sites already pass it; the value is
// appended as a `projectID` query parameter for the openvscode side to
// pick up when that work lands.

export const OPENVSCODE_DEFAULT_PORT = 3030;
export const OPENVSCODE_DEFAULT_URL = `http://localhost:${OPENVSCODE_DEFAULT_PORT}`;
export const OPENVSCODE_WORKSPACE_FOLDER = "/home/workspace";

// resolveOpenvscodeBase — returns the configured base URL with no
// trailing slash. Falls back to the dev default so the iframe still
// renders against a local openvscode container when no env override
// is present.
export function resolveOpenvscodeBase(): string {
  const raw =
    (typeof process !== "undefined" &&
      process.env?.NEXT_PUBLIC_OPENVSCODE_URL) ||
    OPENVSCODE_DEFAULT_URL;
  return raw.replace(/\/+$/, "");
}

// getOpenvscodeUrl — builds the iframe src.
//
// Today we always open `/home/workspace`. Once openvscode-server is
// taught about per-project sandboxes the `projectID` will route the
// iframe at `/home/workspace/<projectID>` (or similar) — for now the
// value is forwarded as a query parameter for forward compatibility.
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
