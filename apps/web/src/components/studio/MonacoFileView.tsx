"use client";

// MonacoFileView — read-only Monaco editor wrapper themed to the
// IronFlyer dark palette. Used by CodePane and FileViewer to give the
// studio a real "VS Code cloud builder" feel (per the locked design
// reference manifest at `design-reference/2026-05-25-private-ironflyer/`).
//
// Why Monaco (vs. a plain <pre>): syntax highlighting, soft tabs,
// keyboard navigation, find/replace, multi-cursor, copy-with-format —
// the things a real engineer expects when reading generated code.
// Read-only by default because the project keeps "patches are
// mandatory" — the AI never writes files directly, and human edits
// will route through `proposePatch` when that flow is wired.
//
// The component is intentionally lazy-friendly: import it inside a
// `next/dynamic({ ssr: false })` block so the ~1MB monaco-editor
// bundle only ships when the user opens the Code/Files tab.

import { Box } from "@mui/material";
import Editor, { type OnMount } from "@monaco-editor/react";
import { useCallback, useMemo, useRef, useState } from "react";
import { tokens } from "../../theme";

// IRONFLYER_THEME — maps our token palette into Monaco's rules array.
// Only need to register once per page load; the wrapper guards with
// the `themeReady` flag in onMount.
const THEME_NAME = "ironflyer-dark";

let themeRegistered = false;

export interface MonacoFileViewProps {
  value: string;
  language?: string;
  // Used as the model URI so Monaco's per-language tokenizer + symbol
  // resolution keys off the file path the same way VS Code does.
  path?: string;
  readOnly?: boolean;
  // Wrap long lines like VS Code's "View → Word Wrap".
  wordWrap?: boolean;
  // Optional minimap toggle. Kept off by default for dense Studio panes.
  minimap?: boolean;
  // Show a compact VS Code-style status strip under the editor.
  showStatusBar?: boolean;
  // Optional save handler — when supplied, Ctrl/Cmd+S triggers it.
  onSave?: (next: string) => void;
  onChange?: (next: string) => void;
  // Override the editor's container height. Defaults to "100%" so the
  // wrapper inherits parent flex sizing.
  height?: string | number;
}

interface MonacoStatus {
  line: number;
  column: number;
  selectedChars: number;
  selectedLines: number;
  lines: number;
  chars: number;
  loadMs: number | null;
}

export function MonacoFileView({
  value,
  language,
  path,
  readOnly = true,
  wordWrap = false,
  minimap = false,
  showStatusBar = true,
  onSave,
  onChange,
  height = "100%",
}: MonacoFileViewProps) {
  const mountStartedAt = useRef<number>(
    typeof performance !== "undefined" ? performance.now() : Date.now(),
  );
  const [status, setStatus] = useState<MonacoStatus>(() => ({
    line: 1,
    column: 1,
    selectedChars: 0,
    selectedLines: 0,
    lines: value.split(/\r\n|\r|\n/).length,
    chars: value.length,
    loadMs: null,
  }));

  const onMount: OnMount = useCallback(
    (editor, monaco) => {
      if (!themeRegistered) {
        monaco.editor.defineTheme(THEME_NAME, {
          base: "vs-dark",
          inherit: true,
          rules: [
            { token: "comment", foreground: stripHash(tokens.color.text.muted), fontStyle: "italic" },
            { token: "keyword", foreground: stripHash(tokens.color.accent.violet), fontStyle: "bold" },
            { token: "string", foreground: stripHash(tokens.color.accent.coral) },
            { token: "number", foreground: stripHash(tokens.color.accent.yellow) },
            { token: "type", foreground: stripHash(tokens.color.accent.sky) },
            { token: "function", foreground: stripHash(tokens.color.accent.sky) },
            { token: "variable", foreground: stripHash(tokens.color.text.primary) },
            { token: "tag", foreground: stripHash(tokens.color.accent.violet) },
            { token: "attribute.name", foreground: stripHash(tokens.color.accent.sky) },
            { token: "attribute.value", foreground: stripHash(tokens.color.accent.coral) },
          ],
          colors: {
            "editor.background": tokens.color.bg.inset,
            "editor.foreground": tokens.color.text.primary,
            "editorLineNumber.foreground": tokens.color.text.muted,
            "editorLineNumber.activeForeground": tokens.color.accent.violet,
            "editorCursor.foreground": tokens.color.accent.violet,
            "editor.selectionBackground": `${tokens.color.accent.violet}3d`,
            "editor.inactiveSelectionBackground": `${tokens.color.accent.violet}26`,
            "editor.lineHighlightBackground": tokens.color.bg.surface,
            "editor.lineHighlightBorder": tokens.color.bg.surface,
            "editorIndentGuide.background1": `${tokens.color.text.muted}33`,
            "editorIndentGuide.activeBackground1": `${tokens.color.accent.violet}66`,
            "editorBracketMatch.background": `${tokens.color.accent.violet}22`,
            "editorBracketMatch.border": tokens.color.accent.violet,
            "editorGutter.background": tokens.color.bg.inset,
            "editor.findMatchBackground": `${tokens.color.accent.warning}55`,
            "editor.findMatchHighlightBackground": `${tokens.color.accent.warning}2e`,
            "scrollbarSlider.background": `${tokens.color.text.muted}33`,
            "scrollbarSlider.hoverBackground": `${tokens.color.text.muted}55`,
            "scrollbarSlider.activeBackground": tokens.color.accent.violet,
            "minimap.background": tokens.color.bg.inset,
          },
        });
        themeRegistered = true;
      }
      monaco.editor.setTheme(THEME_NAME);

      if (onSave) {
        // Ctrl/Cmd+S → save. Monaco passes the latest model value so
        // the handler doesn't have to re-read it.
        editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
          onSave(editor.getValue());
        });
      }

      editor.addAction({
        id: "ironflyer.format-document",
        label: "IronFlyer: Format Document",
        keybindings: [monaco.KeyMod.Shift | monaco.KeyMod.Alt | monaco.KeyCode.KeyF],
        run: async () => {
          await editor.getAction("editor.action.formatDocument")?.run();
        },
      });
      editor.addAction({
        id: "ironflyer.copy-path",
        label: "IronFlyer: Copy File Path",
        run: async () => {
          if (!path || typeof navigator === "undefined") return;
          await navigator.clipboard.writeText(path).catch(() => undefined);
        },
      });
      editor.addAction({
        id: "ironflyer.toggle-minimap",
        label: "IronFlyer: Toggle Minimap",
        run: () => {
          const current = editor.getRawOptions().minimap?.enabled ?? false;
          editor.updateOptions({ minimap: { enabled: !current } });
        },
      });

      const updateStatus = () => {
        const model = editor.getModel();
        if (!model) return;
        const pos = editor.getPosition();
        const selection = editor.getSelection();
        const selectedText =
          selection && !selection.isEmpty()
            ? model.getValueInRange(selection)
            : "";
        setStatus({
          line: pos?.lineNumber ?? 1,
          column: pos?.column ?? 1,
          selectedChars: selectedText.length,
          selectedLines:
            selection && !selection.isEmpty()
              ? Math.abs(selection.endLineNumber - selection.startLineNumber) + 1
              : 0,
          lines: model.getLineCount(),
          chars: model.getValueLength(),
          loadMs: Math.max(
            0,
            Math.round(
              (typeof performance !== "undefined" ? performance.now() : Date.now()) -
                mountStartedAt.current,
            ),
          ),
        });
      };
      const disposables = [
        editor.onDidChangeCursorPosition(updateStatus),
        editor.onDidChangeCursorSelection(updateStatus),
        editor.onDidChangeModelContent(updateStatus),
      ];
      updateStatus();
      editor.onDidDispose(() => {
        disposables.forEach((d) => d.dispose());
      });
    },
    [onSave, path],
  );

  const statusText = useMemo(() => {
    const bits = [
      `Ln ${status.line}`,
      `Col ${status.column}`,
      `${status.lines} lines`,
      `${status.chars.toLocaleString()} chars`,
    ];
    if (status.selectedChars > 0) {
      bits.splice(
        2,
        0,
        `${status.selectedChars.toLocaleString()} selected`,
        `${status.selectedLines} sel lines`,
      );
    }
    if (status.loadMs != null) bits.push(`${status.loadMs} ms`);
    return bits.join(" · ");
  }, [status]);

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.inset,
        display: "flex",
        flexDirection: "column",
        flex: 1,
        height,
        minHeight: 0,
        width: "100%",
      }}
    >
      <Editor
        value={value}
        language={language}
        path={path}
        theme={THEME_NAME}
        loading={<EditorLoading />}
        onMount={onMount}
        onChange={(next) => onChange?.(next ?? "")}
        options={{
          readOnly,
          domReadOnly: readOnly,
          fontFamily: tokens.font.mono,
          fontSize: 13,
          lineHeight: 1.6,
          minimap: { enabled: minimap },
          smoothScrolling: true,
          scrollBeyondLastLine: false,
          renderLineHighlight: "line",
          wordWrap: wordWrap ? "on" : "off",
          tabSize: 2,
          insertSpaces: true,
          autoClosingBrackets: "always",
          autoClosingQuotes: "always",
          autoIndent: "advanced",
          codeLens: true,
          colorDecorators: true,
          dragAndDrop: true,
          find: { addExtraSpaceOnTop: false, autoFindInSelection: "multiline" },
          folding: true,
          formatOnPaste: !readOnly,
          formatOnType: !readOnly,
          inlayHints: { enabled: "onUnlessPressed" },
          linkedEditing: true,
          links: true,
          matchBrackets: "always",
          occurrencesHighlight: "singleFile",
          parameterHints: { enabled: true },
          quickSuggestions: !readOnly,
          renderFinalNewline: "on",
          renderWhitespace: "selection",
          selectionHighlight: true,
          showFoldingControls: "mouseover",
          snippetSuggestions: "inline",
          stickyScroll: { enabled: true },
          suggest: {
            preview: true,
            showInlineDetails: true,
            showStatusBar: true,
            snippetsPreventQuickSuggestions: false,
          },
          padding: { top: 12, bottom: 16 },
          guides: {
            indentation: true,
            highlightActiveIndentation: true,
            bracketPairs: true,
          },
          bracketPairColorization: { enabled: true },
          scrollbar: {
            verticalScrollbarSize: 10,
            horizontalScrollbarSize: 10,
            alwaysConsumeMouseWheel: false,
          },
          // Read-only quality of life: hide the cursor blink + drop the
          // "you can edit me" affordances when we're in viewer mode.
          ...(readOnly
            ? {
                cursorBlinking: "solid",
                lineDecorationsWidth: 8,
                renderValidationDecorations: "off",
                contextmenu: false,
              }
            : {}),
        }}
      />
      {showStatusBar ? (
        <Box
          sx={{
            alignItems: "center",
            borderTop: `1px solid ${tokens.color.border.subtle}`,
            color: tokens.color.text.muted,
            display: "flex",
            flex: "0 0 auto",
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            gap: 1,
            height: 24,
            minWidth: 0,
            overflow: "hidden",
            px: 1.25,
            whiteSpace: "nowrap",
          }}
        >
          <Box
            component="span"
            sx={{
              minWidth: 0,
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {statusText}
          </Box>
        </Box>
      ) : null}
    </Box>
  );
}

function EditorLoading() {
  return (
    <Box
      sx={{
        alignItems: "center",
        bgcolor: tokens.color.bg.inset,
        color: tokens.color.text.muted,
        display: "flex",
        fontFamily: tokens.font.mono,
        fontSize: 11,
        height: "100%",
        justifyContent: "center",
        letterSpacing: 1,
        textTransform: "uppercase",
        width: "100%",
      }}
    >
      Loading editor…
    </Box>
  );
}

// Monaco's theme rules want hex without the leading `#`. The cockpit
// tokens are stored with `#` so we strip on the boundary.
function stripHash(hex: string): string {
  return hex.startsWith("#") ? hex.slice(1) : hex;
}
