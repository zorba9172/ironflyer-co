# Pin a project

Each VSCode window can pin to exactly one Ironflyer project. The pin powers:

- **Ask Ironflyer to fix** — the code action knows which project to route the prompt to.
- **Future quick actions** — Run Finisher / Show Budget surface scoped to the pinned project.

`Ironflyer: Set Active Project` shows a quick-pick of every project you own. The choice persists in `workspaceState`, so it survives reloads but does not bleed across windows.

If you only have one project, the pin happens automatically the first time it's needed.
