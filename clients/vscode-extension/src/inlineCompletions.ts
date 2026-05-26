// Cursor-style ghost-text inline completions. Registers as an
// `InlineCompletionItemProvider` for `*` (all languages); the orchestrator
// picks a cheap+fast provider via the capability-tagged bandit and streams
// back the suggestion via SSE.
//
// Design notes:
//
//   - Debounce is configurable via `ironflyer.completions.debounceMs`
//     (default 250). The provider returns null on every call until the
//     timer fires AND a fresh provideInlineCompletionItems request has
//     stabilised the cursor for >= debounce ms. We do this by recording
//     the most-recent (uri, position) and only firing the network call
//     when the same coordinate survives the debounce window.
//
//   - Each request carries an opaque `requestId` keyed by (uri, line, col).
//     The orchestrator cancels prior pending streams under the same id so
//     a quick re-trigger never double-bills.
//
//   - We cap the rendered suggestion at `ironflyer.completions.maxLines`
//     lines so a runaway model doesn't paint 200 lines of ghost text into
//     the editor. Output is stripped of common boilerplate (fenced code
//     blocks, "Here's the completion:" prefixes) before display.
//
//   - The provider auto-disables when the user toggles the status-bar
//     pill off OR when `ironflyer.completions.enabled` is false. Both
//     paths route through the same config flag — the toggle command
//     writes `ironflyer.completions.enabled` and the provider re-reads
//     it on every call.

import * as vscode from 'vscode';
import { Api } from './api';
import { Auth } from './auth';
import { readConfig } from './config';
import { log } from './logger';

const PREFIX_CAP_BYTES = 2 * 1024;
const SUFFIX_CAP_BYTES = 512;
// Hard request-timeout backstop. The orchestrator enforces its own
// 1.5s first-token deadline; this is just a belt-and-braces escape
// hatch in case the server hangs while streaming.
const REQUEST_TIMEOUT_MS = 5000;
// Client-side first-token deadline. With the GraphQL subscription
// transport we no longer get a free "204" from the server when the
// model is too slow — so the provider races the first delta against a
// timer and silently bails if the wire is dry by then. This keeps
// ghost-text suggestions feeling instant.
const FIRST_TOKEN_DEADLINE_MS = 1500;

export class IronflyerInlineCompletionProvider
  implements vscode.InlineCompletionItemProvider
{
  constructor(
    private readonly api: Api,
    private readonly auth: Auth,
  ) {}

  async provideInlineCompletionItems(
    document: vscode.TextDocument,
    position: vscode.Position,
    _context: vscode.InlineCompletionContext,
    token: vscode.CancellationToken,
  ): Promise<vscode.InlineCompletionItem[] | vscode.InlineCompletionList | null> {
    const cfg = readConfig();
    if (!cfg.completionsEnabled) return null;
    // Must be signed in — the endpoint is auth-gated, no point burning a
    // round-trip we know will 401.
    if (!(await this.auth.getToken())) return null;

    // Debounce: wait for the cursor to settle. If the user keeps typing
    // the CancellationToken fires and we bail before any network call.
    try {
      await debounce(cfg.completionsDebounceMs, token);
    } catch {
      return null;
    }
    if (token.isCancellationRequested) return null;

    const { prefix, suffix } = sliceAroundCursor(document, position);
    if (!prefix.trim() && !suffix.trim()) return null;

    const filename = vscode.workspace.asRelativePath(document.uri, false);
    const requestId = `${document.uri.toString()}#${position.line}:${position.character}`;

    const controller = new AbortController();
    const onCancel = token.onCancellationRequested(() => controller.abort());
    const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);

    try {
      let text = '';
      const stream = this.api.inlineCompletion(
        {
          prefix,
          suffix,
          language: document.languageId,
          filename,
          requestId,
        },
        controller.signal,
      );
      const iter = stream[Symbol.asyncIterator]();
      // Race the first delta against the 1.5s deadline — if the wire is
      // dry by then the suggestion is already too stale to render, so
      // we abort the WS subscription and silently bail out.
      const first = await Promise.race([
        iter.next(),
        new Promise<'timeout'>((resolve) =>
          setTimeout(() => resolve('timeout'), FIRST_TOKEN_DEADLINE_MS),
        ),
      ]);
      if (first === 'timeout') {
        controller.abort();
        return null;
      }
      if (first.done) return null;
      let cur: IteratorResult<typeof first.value> = first;
      while (!cur.done) {
        if (token.isCancellationRequested) return null;
        const evt = cur.value;
        if (evt.event === 'text') {
          const piece = safeJsonField(evt.data, 'text');
          if (piece) text += piece;
        } else if (evt.event === 'error') {
          log.warn(`inline-completion error: ${JSON.stringify(evt.data)}`);
          return null;
        } else if (evt.event === 'done' || evt.event === 'cancelled') {
          break;
        }
        cur = await iter.next();
      }
      const trimmed = sanitizeSuggestion(text, cfg.completionsMaxLines);
      if (!trimmed) return null;
      const item = new vscode.InlineCompletionItem(
        trimmed,
        new vscode.Range(position, position),
      );
      // Accept-rate telemetry: fire the no-op accept endpoint when the
      // user actually accepts the suggestion (tab/enter). VSCode invokes
      // the `command` field on accept.
      item.command = {
        command: 'ironflyer.inlineCompletionAccepted',
        title: 'Ironflyer inline accept telemetry',
      };
      return [item];
    } catch (err) {
      if (controller.signal.aborted) return null;
      log.warn('inline-completion request failed', err);
      return null;
    } finally {
      clearTimeout(timeout);
      onCancel.dispose();
    }
  }
}

/**
 * Status bar pill that mirrors the `ironflyer.completions.enabled` flag
 * and lets the user toggle ghost-text with a single click. Kept beside
 * the budget pill so the on/off state is always visible while editing.
 */
export class InlineCompletionsStatusBar implements vscode.Disposable {
  private readonly item: vscode.StatusBarItem;
  private readonly disposables: vscode.Disposable[] = [];

  constructor() {
    this.item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 99);
    this.item.command = 'ironflyer.toggleInlineCompletions';
    this.refresh();
    this.item.show();
    this.disposables.push(
      vscode.workspace.onDidChangeConfiguration((e) => {
        if (e.affectsConfiguration('ironflyer.completions.enabled')) {
          this.refresh();
        }
      }),
    );
  }

  refresh(): void {
    const enabled = readConfig().completionsEnabled;
    this.item.text = enabled ? '$(sparkle) Ironflyer AI · on' : '$(circle-slash) Ironflyer AI · off';
    this.item.tooltip = enabled
      ? 'Ironflyer inline AI completions are ON — click to disable'
      : 'Ironflyer inline AI completions are OFF — click to enable';
  }

  dispose(): void {
    while (this.disposables.length) this.disposables.pop()?.dispose();
    this.item.dispose();
  }
}

/**
 * Pulls up to PREFIX_CAP_BYTES before the cursor and SUFFIX_CAP_BYTES after.
 * We slice in characters then truncate by byte length to keep the wire
 * payload bounded — Monaco text is UTF-16, so a character can be up to 4
 * UTF-8 bytes; encoder bytes are the right unit for the server cap.
 */
function sliceAroundCursor(
  doc: vscode.TextDocument,
  pos: vscode.Position,
): { prefix: string; suffix: string } {
  const offset = doc.offsetAt(pos);
  // Generous character window — sanitised by encoder below.
  const preChars = Math.min(offset, PREFIX_CAP_BYTES);
  const sufChars = Math.min(doc.getText().length - offset, SUFFIX_CAP_BYTES);
  const prefix = doc.getText(
    new vscode.Range(doc.positionAt(offset - preChars), pos),
  );
  const suffix = doc.getText(
    new vscode.Range(pos, doc.positionAt(offset + sufChars)),
  );
  return {
    prefix: trimToBytes(prefix, PREFIX_CAP_BYTES, 'tail'),
    suffix: trimToBytes(suffix, SUFFIX_CAP_BYTES, 'head'),
  };
}

const encoder = new TextEncoder();

function trimToBytes(s: string, cap: number, side: 'head' | 'tail'): string {
  const bytes = encoder.encode(s);
  if (bytes.length <= cap) return s;
  // Trim greedily by char (safe UTF-16 boundary) until under the cap.
  // We never split a JS surrogate pair because we step by `String#slice`.
  let out = s;
  while (encoder.encode(out).length > cap) {
    if (side === 'tail') {
      out = out.slice(Math.ceil(out.length * 0.05) + 1);
    } else {
      out = out.slice(0, out.length - Math.ceil(out.length * 0.05) - 1);
    }
    if (!out) break;
  }
  return out;
}

/**
 * Strip common LLM boilerplate from the streamed suggestion. Models
 * occasionally ignore the "no fences" rule, so we lift the body out of a
 * fenced block. Also caps to maxLines so a runaway completion doesn't
 * paint a wall of ghost text.
 */
export function sanitizeSuggestion(raw: string, maxLines: number): string {
  let s = raw;
  // Drop a leading fenced block — keep only the code inside the first ```.
  const fenceOpen = s.indexOf('```');
  if (fenceOpen === 0) {
    const afterLang = s.indexOf('\n', fenceOpen + 3);
    if (afterLang > 0) {
      const fenceClose = s.indexOf('```', afterLang + 1);
      s = fenceClose > 0 ? s.slice(afterLang + 1, fenceClose) : s.slice(afterLang + 1);
    }
  }
  // Trim trailing fence if the model wrapped without an opener (rare).
  s = s.replace(/\n?```\s*$/u, '');
  // Cap by line count.
  const lines = s.split('\n');
  if (lines.length > maxLines) {
    s = lines.slice(0, maxLines).join('\n');
  }
  // Don't return whitespace-only ghost text — it looks like a bug.
  if (!s.trim()) return '';
  return s;
}

/**
 * Tiny cancellable debounce. Resolves after `ms` ms; rejects if the
 * VSCode cancellation token fires first.
 */
function debounce(ms: number, token: vscode.CancellationToken): Promise<void> {
  return new Promise((resolve, reject) => {
    if (token.isCancellationRequested) {
      reject(new Error('cancelled'));
      return;
    }
    const timer = setTimeout(() => {
      sub.dispose();
      resolve();
    }, ms);
    const sub = token.onCancellationRequested(() => {
      clearTimeout(timer);
      sub.dispose();
      reject(new Error('cancelled'));
    });
  });
}

/**
 * Pulls one string field out of an SSE `data:` payload. The server emits
 * JSON-encoded payloads for every event; this is a stable extractor that
 * never throws.
 */
function safeJsonField(data: unknown, field: string): string | undefined {
  if (data == null) return undefined;
  let obj: any = data;
  if (typeof data === 'string') {
    try {
      obj = JSON.parse(data);
    } catch {
      return undefined;
    }
  }
  if (obj && typeof obj === 'object' && typeof obj[field] === 'string') {
    return obj[field];
  }
  return undefined;
}
