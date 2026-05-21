// URI helpers for the ironflyer:// scheme used by the diff content provider.
//
// Two roles:
//   ironflyer://current/<projectId>/<path>   — left side: file as it is today
//   ironflyer://proposed/<patchId>/<path>    — right side: file as the patch
//                                              would leave it
//
// The path is percent-encoded so nested slashes are preserved exactly.
// We deliberately keep these as plain string helpers (no VSCode imports)
// so the parsing logic is unit-testable.

export type PatchSide = 'current' | 'proposed';

export interface ParsedPatchUri {
  side: PatchSide;
  id: string;
  path: string;
}

export function buildPatchUri(side: PatchSide, id: string, path: string): string {
  const encodedId = encodeURIComponent(id);
  const encodedPath = path.split('/').map(encodeURIComponent).join('/');
  return `ironflyer://${side}/${encodedId}/${encodedPath}`;
}

export function parsePatchUri(input: string): ParsedPatchUri | undefined {
  const m = /^ironflyer:\/\/(current|proposed)\/([^/]+)\/(.+)$/.exec(input);
  if (!m) return undefined;
  const side = m[1] as PatchSide;
  const id = decodeURIComponent(m[2]);
  const path = m[3].split('/').map(decodeURIComponent).join('/');
  return { side, id, path };
}

export function patchTabTitle(patchId: string, filePath: string): string {
  return `Patch ${shortId(patchId)} · ${filePath}`;
}

export function shortId(id: string): string {
  return id.length > 10 ? id.slice(0, 8) + '…' : id;
}
