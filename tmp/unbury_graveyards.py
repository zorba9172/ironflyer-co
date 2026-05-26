"""Unbury gqlgen-graveyarded helpers.

gqlgen wraps every "unknown" helper in a `/* ... */` comment with this
exact preamble:

    // The code below was going to be deleted when updating resolvers. ...
    /*
        <helper code>
    */

For each *.resolver.go in the resolver dir, this script:
  1. finds the comment-marker line
  2. extracts the code between the matching /* and */
  3. writes the extracted code as a sibling file `<basename>_graveyard.go`
  4. strips the marker + /* ... */ block from the original

The extracted file gets `package resolver` + an import block synthesized
from the imports already in the original file (we copy all imports; Go
will complain about unused ones, which we handle by appending `var _ =
import.Name` lines if necessary — but for the kinds of helpers gqlgen
graveyards, the imports usually map 1-to-1).
"""

import re
import sys
from pathlib import Path

ROOT = Path("/Users/moshemoran/workspace/ironflyer-copilot/ironflyer/apps/orchestrator/internal/graph/resolver")
MARKER = "// The code below was going to be deleted when updating resolvers."

extracted = {}

for path in sorted(ROOT.glob("*.resolver.go")):
    text = path.read_text()
    if MARKER not in text:
        continue
    idx = text.index(MARKER)
    # The marker line is one of 4 // lines; the actual `/*` is just after.
    open_idx = text.find("/*", idx)
    if open_idx < 0:
        continue
    # The closing */ is the LAST */ in the file (gqlgen puts a single
    # /* ... */ block at the bottom).
    close_idx = text.rfind("*/")
    if close_idx < open_idx:
        continue
    block = text[open_idx + 2:close_idx]
    # Cut from the // marker line start to end of file (drop the
    # graveyard entirely from the resolver file).
    # The // marker is preceded by 4 // lines and a /* on its own line.
    # Walk back to the first of the // lines.
    head_start = idx
    # Walk back 3 more //s if present.
    for _ in range(6):
        prev_nl = text.rfind("\n", 0, head_start - 1)
        if prev_nl < 0:
            break
        prev_line = text[prev_nl + 1:head_start].rstrip()
        if prev_line.startswith("//"):
            head_start = prev_nl + 1
        else:
            break
    # Drop everything from head_start to end.
    new_text = text[:head_start].rstrip() + "\n"
    path.write_text(new_text)
    extracted[path.name] = block

# Build a single graveyard recovery file. Each helper from the resolver
# files lives in a single new `_revived.go` per original file.
for src_name, block in extracted.items():
    # Dedent: gqlgen indents the whole block by one tab. Strip one
    # leading tab from each line if present.
    lines = []
    for line in block.split("\n"):
        if line.startswith("\t"):
            line = line[1:]
        lines.append(line)
    body = "\n".join(lines).strip()
    out_path = ROOT / src_name.replace(".resolver.go", "_revived.go")
    # Synthesize a header. We don't try to compute imports — Go will
    # tell us what's missing; for the gqlgen graveyards the typical
    # imports are already in the matching helpers / resolver file.
    out_path.write_text(
        "// Auto-revived helpers — originally graveyarded by gqlgen during a\n"
        "// resolver regeneration. Restored here as a sibling file so the\n"
        "// package compiles. Move each function into the topic-specific\n"
        "// *_helpers.go when convenient.\n\n"
        "package resolver\n\n" + body + "\n"
    )
    print(f"revived {src_name} -> {out_path.name} ({len(body)} bytes)")

print(f"\nrevived {len(extracted)} files")
