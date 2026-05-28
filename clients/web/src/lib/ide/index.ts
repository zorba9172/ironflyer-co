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
// The Studio sync route mirrors projectFiles into
// /home/workspace/projects/<projectID> before the iframe loads. In
// production this same contract should point at a runtime-owned signed
// workspace path instead of the local dev bind mount.

export const OPENVSCODE_DEFAULT_PORT = 3030;
export const OPENVSCODE_DEFAULT_URL = `http://localhost:${OPENVSCODE_DEFAULT_PORT}/ide`;
export const OPENVSCODE_WORKSPACE_ROOT = "/home/workspace/projects";

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
  params.set("folder", getOpenvscodeFolder(projectID));
  if (projectID && projectID.trim()) params.set("projectID", projectID.trim());
  return `${base}/?${params.toString()}`;
}

// getOpenvscodeFileUrl — opens the same workspace folder and asks VS Code
// to focus a specific file. OpenVSCode/code-server both understand
// `goto=/abs/path[:line[:column]]`.
export function getOpenvscodeFileUrl(
  projectID: string,
  filePath: string,
  line = 1,
  column = 1,
): string {
  const base = resolveOpenvscodeBase();
  const folder = getOpenvscodeFolder(projectID);
  const rel = cleanWorkspacePath(filePath);
  const params = new URLSearchParams();
  params.set("folder", folder);
  params.set("goto", `${folder}/${rel}:${Math.max(1, line)}:${Math.max(1, column)}`);
  if (projectID.trim()) params.set("projectID", projectID.trim());
  return `${base}/?${params.toString()}`;
}

export function getOpenvscodeFolder(projectID?: string): string {
  const folder = projectID?.trim() ? projectFolderName(projectID) : "project";
  return `${OPENVSCODE_WORKSPACE_ROOT}/${folder}`;
}

export function projectFolderName(projectID: string): string {
  return projectID.replace(/[^a-zA-Z0-9._-]+/g, "-").slice(0, 80) || "project";
}

function cleanWorkspacePath(raw: string): string {
  const parts = raw
    .replaceAll("\\", "/")
    .split("/")
    .filter((part) => part && part !== "." && part !== "..");
  return parts.join("/") || "README.md";
}
