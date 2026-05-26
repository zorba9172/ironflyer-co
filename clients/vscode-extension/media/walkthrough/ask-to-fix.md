# Ask Ironflyer to fix a diagnostic

Any diagnostic in any language gets a quick fix offer. The extension reads `vscode.languages.getDiagnostics()` indirectly via the standard `CodeActionContext`, so anything that produces squigglies — `tsserver`, `eslint`, `golangci-lint`, Pylance, the Rust analyzer, you name it — works.

When you accept the action, the extension:

1. Captures the diagnostic message, source, code, severity, file path, language id, and the snippet under the range.
2. Formats a markdown prompt addressed to the `coder` agent at `economy` effort.
3. Opens (or reveals) the chat panel for your pinned project.
4. Renders the user turn in the chat so you can see exactly what was sent.
5. Streams the agent's response in place.

The prompt asks for **a minimal patch** and a one-sentence root-cause explanation, so the answer slots cleanly into the Patches review flow.
