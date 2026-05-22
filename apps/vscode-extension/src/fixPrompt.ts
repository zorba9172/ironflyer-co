// Pure prompt formatter for "Ask Ironflyer to fix" — takes a diagnostic
// payload (file path, language, line range, message, severity, code
// snippet) and returns the markdown body we send to the orchestrator
// chat endpoint. Kept here so the formatter is exercised by unit tests
// without spinning up VSCode.

export type DiagnosticSeverity = 'error' | 'warning' | 'info' | 'hint';

export interface FixPromptInput {
  filePath: string;     // Workspace-relative if available, else absolute.
  language?: string;    // languageId (typescript, go, ...). Used as the fence tag.
  startLine: number;    // 1-based inclusive.
  endLine: number;      // 1-based inclusive.
  message: string;
  severity: DiagnosticSeverity;
  source?: string;      // e.g. "eslint", "tsc".
  code?: string;        // diagnostic code, if any.
  snippet?: string;     // The code under the diagnostic range.
}

export function buildFixPrompt(input: FixPromptInput): string {
  const lineRange = input.startLine === input.endLine
    ? `line ${input.startLine}`
    : `lines ${input.startLine}-${input.endLine}`;
  const headerBits: string[] = [`**${input.severity.toUpperCase()}** at ${input.filePath} (${lineRange})`];
  if (input.source) headerBits.push(`source: ${input.source}`);
  if (input.code) headerBits.push(`code: ${input.code}`);
  const lines: string[] = [
    `# Fix this`,
    '',
    headerBits.join(' · '),
    '',
    `> ${input.message.replace(/\n/g, '\n> ')}`,
  ];
  if (input.snippet && input.snippet.trim()) {
    const fence = input.language ?? '';
    lines.push('', '```' + fence, input.snippet.trimEnd(), '```');
  }
  lines.push(
    '',
    'Propose a minimal patch that resolves the issue. Explain the root cause in one sentence, then list the change(s).',
  );
  return lines.join('\n');
}
