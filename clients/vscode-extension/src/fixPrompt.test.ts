import { describe, expect, it } from 'vitest';
import { buildFixPrompt } from './fixPrompt';

describe('buildFixPrompt', () => {
  it('emits header, message, and snippet fenced with the languageId', () => {
    const out = buildFixPrompt({
      filePath: 'src/foo.ts',
      language: 'typescript',
      startLine: 12,
      endLine: 12,
      message: 'Type "string" is not assignable to type "number".',
      severity: 'error',
      source: 'ts',
      code: '2322',
      snippet: 'const n: number = "oops";',
    });
    expect(out).toContain('**ERROR** at src/foo.ts (line 12)');
    expect(out).toContain('source: ts');
    expect(out).toContain('code: 2322');
    expect(out).toContain('> Type "string" is not assignable to type "number".');
    expect(out).toContain('```typescript');
    expect(out).toContain('const n: number = "oops";');
  });

  it('uses "lines N-M" when the range spans multiple lines', () => {
    const out = buildFixPrompt({
      filePath: 'a.go',
      startLine: 5,
      endLine: 9,
      message: 'unused import',
      severity: 'warning',
    });
    expect(out).toContain('lines 5-9');
    expect(out).toContain('**WARNING**');
  });

  it('drops the snippet block when no snippet is provided', () => {
    const out = buildFixPrompt({
      filePath: 'x',
      startLine: 1,
      endLine: 1,
      message: 'm',
      severity: 'info',
    });
    expect(out).not.toContain('```');
  });

  it('quotes multi-line messages line-by-line', () => {
    const out = buildFixPrompt({
      filePath: 'x',
      startLine: 1,
      endLine: 1,
      message: 'first line\nsecond line',
      severity: 'error',
    });
    expect(out).toContain('> first line\n> second line');
  });

  it('always ends with the patch directive', () => {
    const out = buildFixPrompt({
      filePath: 'x',
      startLine: 1,
      endLine: 1,
      message: 'm',
      severity: 'hint',
    });
    expect(out.trimEnd().endsWith('list the change(s).')).toBe(true);
  });
});
