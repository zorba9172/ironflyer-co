# Studio Table Library Contract

Tables in Studio use one shared shell: `StudioTableShell`, normally through `StudioDataGrid` or `StudioDataTable`.

## Required Structure

- Every management table must declare `title`, `subtitle`, `tabs`, `activeTab`, `onTabChange`, and `footer`.
- Search belongs inside the table shell with `searchValue` and `onSearchChange`.
- Custom actions belong in `actions`. Default `Filter` / `Export` buttons are opt-in with `showDefaultActions`.
- Repeated lists that are not strict grids can still use `StudioTableShell` with custom children.

## Tabs

- Use internal tabs for state, type, ownership, severity, or table name.
- Tab counts must reflect the full unsearched dataset.
- Use tones only for meaning: `success`, `warning`, `error`, `info`.
- Keep labels short enough to scan in a horizontal strip.

## Visual Rules

- One frame only: the shell owns border, radius, and footer.
- Inner AG Grid / MUI DataGrid borders are removed when table chrome is present.
- Tables are light-first: white paper, warm gray dividers, orange active cues.
- Avoid neon, glow, purple-blue gradients, nested cards, and decorative chart chrome.
- Default Studio density is compact for repeated work.

## States

- Empty rows need a clear `emptyLabel`.
- Large logs and workflow lists can stay virtualized, but their controls still live in `StudioTableShell`.
- Footer text should explain data source, safety, or operational meaning, not repeat the title.

## Responsive

- Search must collapse to full width on mobile.
- Actions wrap after search.
- Tabs must scroll horizontally without resizing the table body.
