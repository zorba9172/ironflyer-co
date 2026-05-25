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
import { useCallback } from "react";
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
  // Optional save handler — when supplied, Ctrl/Cmd+S triggers it.
  onSave?: (next: string) => void;
  onChange?: (next: string) => void;
  // Override the editor's container height. Defaults to "100%" so the
  // wrapper inherits parent flex sizing.
  height?: string | number;
}

export function MonacoFileView({
  value,
  language,
  path,
  readOnly = true,
  wordWrap = false,
  onSave,
  onChange,
  height = "100%",
}: MonacoFileViewProps) {
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
    },
    [onSave],
  );

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.inset,
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
          minimap: { enabled: false },
          smoothScrolling: true,
          scrollBeyondLastLine: false,
          renderLineHighlight: "line",
          wordWrap: wordWrap ? "on" : "off",
          tabSize: 2,
          insertSpaces: true,
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
