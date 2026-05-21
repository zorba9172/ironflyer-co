import { describe, expect, it } from 'vitest';
import { buildPatchUri, parsePatchUri, patchTabTitle, shortId } from './patchUri';

describe('buildPatchUri / parsePatchUri', () => {
  it('round-trips a simple path', () => {
    const u = buildPatchUri('current', 'proj-1', 'src/index.ts');
    expect(u).toBe('ironflyer://current/proj-1/src/index.ts');
    expect(parsePatchUri(u)).toEqual({ side: 'current', id: 'proj-1', path: 'src/index.ts' });
  });

  it('preserves nested slashes through encoding', () => {
    const path = 'apps/web/app/[id]/page.tsx';
    const u = buildPatchUri('proposed', 'patch-abc', path);
    expect(parsePatchUri(u)?.path).toBe(path);
  });

  it('handles paths with spaces and special chars', () => {
    const path = 'docs/My Notes (draft)/README.md';
    const u = buildPatchUri('current', 'p', path);
    expect(parsePatchUri(u)?.path).toBe(path);
  });

  it('returns undefined for unrelated URIs', () => {
    expect(parsePatchUri('file:///etc/passwd')).toBeUndefined();
    expect(parsePatchUri('ironflyer://other/x/y')).toBeUndefined();
    expect(parsePatchUri('ironflyer://current/onlyId')).toBeUndefined();
  });

  it('extracts the side correctly', () => {
    expect(parsePatchUri('ironflyer://proposed/p/x.ts')?.side).toBe('proposed');
    expect(parsePatchUri('ironflyer://current/p/x.ts')?.side).toBe('current');
  });
});

describe('shortId / patchTabTitle', () => {
  it('truncates long ids with an ellipsis', () => {
    expect(shortId('patch_abcdef0123')).toBe('patch_ab…');
    expect(shortId('short')).toBe('short');
  });

  it('formats a tab title that fits VSCode tab widths', () => {
    expect(patchTabTitle('patch_abcdef0123', 'src/x.ts')).toBe('Patch patch_ab… · src/x.ts');
  });
});
