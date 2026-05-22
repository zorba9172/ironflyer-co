# Open chat for a project

`Ironflyer: Open Chat` (default ⌘⌥I / Ctrl+Alt+I) opens a streaming chat panel. The composer at the bottom has:

- **Agent role** — `planner` / `architect` / `coder` / `reviewer` / `tester` / `security`. Each is a separate system prompt + capability set.
- **Effort dial** — `Lite` biases the router toward cheap+fast models, `Power` toward reasoning models with extended thinking. `Economy` leaves the agent's default capabilities untouched.

While the panel is open, the extension also subscribes to the project's lifecycle stream — gate status changes and patch proposals refresh the Patches / Finisher Gates trees automatically, and lifecycle events appear inline in the chat log so you can watch a Finisher pass without staring at the trees.
