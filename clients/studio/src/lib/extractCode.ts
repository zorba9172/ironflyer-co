// Pull file blocks out of streamed markdown. The chat is prompted to tag each
// fence with the file path, e.g.  ```tsx src/App.tsx  → { path, content }.
// Only complete fences with a path-like info token become files (bash/steps
// blocks are ignored).
const FENCE = /```([^\n`]*)\n([\s\S]*?)```/g;

export function extractCodeFiles(markdown: string): { path: string; content: string }[] {
  const out: { path: string; content: string }[] = [];
  FENCE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = FENCE.exec(markdown))) {
    const info = (m[1] ?? '').trim();
    const content = (m[2] ?? '').replace(/\n$/, '');
    if (!info) continue;
    const tokens = info.split(/\s+/);
    // path is the last token when there's a "lang path" pair, else the single token
    const candidate = tokens.length > 1 ? tokens[tokens.length - 1] : tokens[0];
    if (candidate && /[\\/]/.test(candidate) === false && /\.\w{1,8}$/.test(candidate) === false) continue;
    if (candidate && /\.\w{1,8}$/.test(candidate)) out.push({ path: candidate, content });
  }
  return out;
}
