# Review and apply patches

The **Patches** view in the Ironflyer activity bar lists every patch the orchestrator has proposed, grouped by project. Each patch expands into its file changes — clicking any change opens VSCode's native side-by-side diff between the current file and the proposed content. The "left" and "right" sides are served through an `ironflyer://` text-document content provider, so they are read-only and round-trip cleanly.

Inline ✔ on a `validated` or `proposed` patch confirms with a modal and posts to `/patches/{id}/apply`. The provider's caches invalidate immediately so the next diff reflects the new project state.

Status icons + colors mirror the lifecycle:

| status      | icon              | color   |
| ----------- | ----------------- | ------- |
| proposed    | `circle-outline`  | default |
| validated   | `pass`            | default |
| applied     | `check`           | green   |
| rejected    | `error`           | red     |
| rolled-back | `history`         | orange  |
