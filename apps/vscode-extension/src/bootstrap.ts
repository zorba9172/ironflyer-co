// Bootstrap — picks up a single-use auth handoff dropped by the
// Studio's IDE sync route into the workspace folder.
//
// Flow when the user opens an Ironflyer project in the cloud IDE
// embedded in the Studio:
//   1. The Studio's POST /api/ide/sync is cookie-authenticated by the
//      Next.js session. After mirroring projectFiles to disk it also
//      writes `.ironflyer/auth.json` with the user's current JWT, the
//      pinned projectID, and an expiresAt 60 seconds out.
//   2. openvscode-server boots, this extension activates, and this
//      function scans every workspace folder for the handoff file.
//   3. If found AND not expired: store the JWT in SecretStorage, pin
//      the project on the ActiveProject store, and DELETE the file.
//   4. Otherwise we fall back to the normal sign-in command flow.
//
// Why a single-use file rather than a URL/query param: query params
// land in browser history + access logs, postMessage doesn't reach
// the extension host, and cookies don't cross the iframe boundary
// when the openvscode container is on a different port. The file
// lives only inside the bind mount Docker shares with this
// container; it's deleted as soon as it's read.

import { promises as fs } from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';
import { ActiveProject } from './activeProject';
import { Api } from './api';
import { Auth } from './auth';
import { log } from './logger';

interface AuthHandoff {
  token: string;
  projectID?: string;
  expiresAt: number;
}

const HANDOFF_DIR = '.ironflyer';
const HANDOFF_FILE = 'auth.json';

export async function consumeStudioHandoff(
  auth: Auth,
  api: Api,
  activeProject: ActiveProject,
): Promise<void> {
  const folders = vscode.workspace.workspaceFolders ?? [];
  for (const folder of folders) {
    if (folder.uri.scheme !== 'file') continue;
    const handoffPath = path.join(folder.uri.fsPath, HANDOFF_DIR, HANDOFF_FILE);
    try {
      const raw = await fs.readFile(handoffPath, 'utf8');
      const parsed = JSON.parse(raw) as AuthHandoff;
      // Always delete the file first — even if the payload is invalid
      // or expired we don't want a stale token sitting on disk longer
      // than necessary. unlink failures are non-fatal; the worst case
      // is the file lives until the next sync overwrites it.
      await fs.unlink(handoffPath).catch(() => undefined);
      if (!parsed || typeof parsed.token !== 'string' || !parsed.token.trim()) {
        log.warn(`auth handoff at ${handoffPath} missing token`);
        continue;
      }
      if (typeof parsed.expiresAt !== 'number' || parsed.expiresAt < Date.now()) {
        log.warn(`auth handoff at ${handoffPath} expired or missing expiresAt`);
        continue;
      }
      await auth.setToken(parsed.token.trim());
      if (typeof parsed.projectID === 'string' && parsed.projectID.trim()) {
        try {
          const p = await api.getProject(parsed.projectID.trim());
          await activeProject.set({ id: p.id, name: p.name });
        } catch (err) {
          log.warn(
            `auth handoff pinned projectID ${parsed.projectID} but getProject failed`,
            err,
          );
        }
      }
      log.info(`accepted Studio auth handoff from ${handoffPath}`);
      return;
    } catch (err) {
      const code = (err as NodeJS.ErrnoException)?.code;
      if (code === 'ENOENT') continue;
      log.warn(`auth handoff read failed for ${handoffPath}`, err);
    }
  }
}
