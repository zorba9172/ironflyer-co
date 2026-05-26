export interface IntelligenceFile {
  path: string;
  content?: string | null;
  size?: number | null;
  language?: string | null;
}

export interface TechSlice {
  key: string;
  label: string;
  icon: string;
  percent: number;
  bytes: number;
  files: number;
  color: string;
}

export interface ComponentSignal {
  key: string;
  label: string;
  icon: string;
  tone: "primary" | "muted" | "security" | "data" | "deploy";
}

export interface ProjectIntelligence {
  primaryStack: string;
  totalFiles: number;
  codeBytes: number;
  languages: TechSlice[];
  components: ComponentSignal[];
  insight: string;
}

type LanguageMeta = {
  label: string;
  icon: string;
  color: string;
};

const LANGUAGE_META: Record<string, LanguageMeta> = {
  typescript: { label: "TypeScript", icon: "TS", color: "#5EA7FF" },
  javascript: { label: "JavaScript", icon: "JS", color: "#F7D95C" },
  react: { label: "React", icon: "Rx", color: "#67E8F9" },
  go: { label: "Go", icon: "Go", color: "#5EEAD4" },
  python: { label: "Python", icon: "Py", color: "#FACC15" },
  css: { label: "CSS", icon: "CSS", color: "#A78BFA" },
  html: { label: "HTML", icon: "HT", color: "#FB7185" },
  json: { label: "JSON", icon: "{}", color: "#C4B5FD" },
  yaml: { label: "YAML", icon: "Y", color: "#FDBA74" },
  docker: { label: "Docker", icon: "Dk", color: "#38BDF8" },
  sql: { label: "SQL", icon: "DB", color: "#86EFAC" },
  shell: { label: "Shell", icon: "$", color: "#D9F99D" },
  markdown: { label: "Docs", icon: "Md", color: "#CBD5E1" },
  other: { label: "Other", icon: "*", color: "#94A3B8" },
};

const CODELIKE = new Set([
  "typescript",
  "javascript",
  "react",
  "go",
  "python",
  "css",
  "html",
  "json",
  "yaml",
  "docker",
  "sql",
  "shell",
]);

export function buildProjectIntelligence(files: IntelligenceFile[]): ProjectIntelligence {
  const stats = new Map<string, { bytes: number; files: number }>();
  const components = new Map<string, ComponentSignal>();
  let codeBytes = 0;

  for (const file of files) {
    const path = cleanPath(file.path);
    if (!path || shouldIgnorePath(path)) continue;

    const lang = detectLanguage(path, file.language);
    const bytes = Math.max(0, file.size ?? file.content?.length ?? 0);
    const prev = stats.get(lang) ?? { bytes: 0, files: 0 };
    prev.bytes += bytes;
    prev.files += 1;
    stats.set(lang, prev);
    if (CODELIKE.has(lang)) codeBytes += bytes;

    detectPathComponents(path, file.content ?? "", components);
  }

  const denominator = Math.max(1, Array.from(stats.values()).reduce((sum, s) => sum + s.bytes, 0));
  const languages = Array.from(stats.entries())
    .map(([key, s]) => {
      const meta = LANGUAGE_META[key] ?? LANGUAGE_META.other;
      return {
        key,
        label: meta.label,
        icon: meta.icon,
        percent: Math.round((s.bytes / denominator) * 100),
        bytes: s.bytes,
        files: s.files,
        color: meta.color,
      };
    })
    .filter((s) => s.bytes > 0 || s.files > 0)
    .sort((a, b) => b.bytes - a.bytes)
    .slice(0, 5);

  const componentList = Array.from(components.values()).slice(0, 8);
  const primaryStack = inferPrimaryStack(languages, componentList);
  return {
    primaryStack,
    totalFiles: files.filter((f) => !shouldIgnorePath(cleanPath(f.path))).length,
    codeBytes,
    languages,
    components: componentList,
    insight: buildInsight(primaryStack, languages, componentList),
  };
}

function cleanPath(path: string): string {
  return path.trim().replaceAll("\\", "/").replace(/^\.\//, "");
}

function shouldIgnorePath(path: string): boolean {
  const low = path.toLowerCase();
  return (
    low.includes("/node_modules/") ||
    low.startsWith("node_modules/") ||
    low.includes("/.git/") ||
    low.startsWith(".git/") ||
    low.endsWith("package-lock.json") ||
    low.endsWith("go.sum") ||
    low.endsWith(".png") ||
    low.endsWith(".jpg") ||
    low.endsWith(".jpeg") ||
    low.endsWith(".gif") ||
    low.endsWith(".webp") ||
    low.endsWith(".zip")
  );
}

function detectLanguage(path: string, declared?: string | null): string {
  const declaredLang = declared?.trim().toLowerCase();
  if (declaredLang) {
    if (declaredLang.includes("typescript")) return "typescript";
    if (declaredLang.includes("javascript")) return "javascript";
    if (declaredLang.includes("python")) return "python";
    if (declaredLang === "go") return "go";
  }
  const low = path.toLowerCase();
  if (low.endsWith(".tsx") || low.endsWith(".jsx")) return "react";
  if (low.endsWith(".ts")) return "typescript";
  if (low.endsWith(".js") || low.endsWith(".mjs") || low.endsWith(".cjs")) return "javascript";
  if (low.endsWith(".go")) return "go";
  if (low.endsWith(".py")) return "python";
  if (low.endsWith(".css") || low.endsWith(".scss")) return "css";
  if (low.endsWith(".html")) return "html";
  if (low.endsWith(".json")) return "json";
  if (low.endsWith(".yaml") || low.endsWith(".yml")) return "yaml";
  if (low.endsWith("dockerfile") || low.includes(".dockerfile")) return "docker";
  if (low.endsWith(".sql")) return "sql";
  if (low.endsWith(".sh") || low.endsWith(".bash")) return "shell";
  if (low.endsWith(".md") || low.endsWith(".mdx")) return "markdown";
  return "other";
}

function detectPathComponents(
  path: string,
  content: string,
  out: Map<string, ComponentSignal>,
): void {
  const low = path.toLowerCase();
  if (low.endsWith("next.config.js") || low.endsWith("next.config.mjs") || low.endsWith("next.config.ts")) {
    add(out, "next", "Next.js", "Nx", "primary");
  }
  if (low.endsWith("vite.config.ts") || low.endsWith("vite.config.js")) {
    add(out, "vite", "Vite", "V", "primary");
  }
  if (low.endsWith("dockerfile") || low.includes(".dockerfile")) {
    add(out, "docker", "Docker", "Dk", "deploy");
  }
  if (low.includes("docker-compose") || low.includes("/compose/")) {
    add(out, "compose", "Compose", "Dc", "deploy");
  }
  if (low.includes("pulumi.")) {
    add(out, "pulumi", "Pulumi", "Pu", "deploy");
  }
  if (low.includes("graphql") || content.includes("gql`") || content.includes("GraphQL")) {
    add(out, "graphql", "GraphQL", "Gq", "data");
  }
  if (low.endsWith("go.mod")) {
    add(out, "go", "Go service", "Go", "primary");
    if (content.includes("github.com/go-chi/chi")) add(out, "chi", "Chi router", "χ", "primary");
    if (content.includes("github.com/99designs/gqlgen")) add(out, "gqlgen", "gqlgen", "Gq", "data");
    if (content.includes("github.com/jackc/pgx")) add(out, "postgres", "Postgres", "Pg", "data");
  }
  if (low.endsWith("requirements.txt") || low.endsWith("pyproject.toml")) {
    add(out, "python", "Python", "Py", "primary");
    const text = content.toLowerCase();
    if (text.includes("fastapi")) add(out, "fastapi", "FastAPI", "Fa", "primary");
    if (text.includes("flask")) add(out, "flask", "Flask", "Fl", "primary");
    if (text.includes("django")) add(out, "django", "Django", "Dj", "primary");
  }
  if (low.endsWith("package.json")) {
    addPackageJSONComponents(content, out);
  }
}

function addPackageJSONComponents(content: string, out: Map<string, ComponentSignal>): void {
  try {
    const doc = JSON.parse(content) as {
      dependencies?: Record<string, string>;
      devDependencies?: Record<string, string>;
    };
    const deps = { ...(doc.dependencies ?? {}), ...(doc.devDependencies ?? {}) };
    if (deps.next) add(out, "next", "Next.js", "Nx", "primary");
    if (deps.react) add(out, "react", "React", "Rx", "primary");
    if (deps["@mui/material"]) add(out, "mui", "MUI", "Mui", "primary");
    if (deps["@apollo/client"]) add(out, "apollo", "Apollo", "Ap", "data");
    if (deps.graphql) add(out, "graphql", "GraphQL", "Gq", "data");
    if (deps.zustand) add(out, "zustand", "Zustand", "Z", "muted");
    if (deps["@playwright/test"]) add(out, "playwright", "Playwright", "Pw", "security");
    if (deps.expo) add(out, "expo", "Expo", "Ex", "primary");
    if (deps.vue) add(out, "vue", "Vue", "Vue", "primary");
  } catch {
    // Invalid package.json should be surfaced by lint/build, not this summary.
  }
}

function add(
  out: Map<string, ComponentSignal>,
  key: string,
  label: string,
  icon: string,
  tone: ComponentSignal["tone"],
): void {
  if (out.has(key)) return;
  out.set(key, { key, label, icon, tone });
}

function inferPrimaryStack(languages: TechSlice[], components: ComponentSignal[]): string {
  const keys = new Set(components.map((c) => c.key));
  if (keys.has("next")) return "Next.js app";
  if (keys.has("expo")) return "Expo mobile";
  if (keys.has("fastapi")) return "FastAPI service";
  if (keys.has("flask")) return "Flask API";
  if (keys.has("go") && keys.has("graphql")) return "Go GraphQL service";
  if (keys.has("go")) return "Go service";
  const top = languages[0]?.label;
  return top ? `${top} project` : "Project";
}

function buildInsight(
  primaryStack: string,
  languages: TechSlice[],
  components: ComponentSignal[],
): string {
  const top = languages[0];
  const deploy = components.some((c) => c.tone === "deploy");
  const data = components.some((c) => c.tone === "data");
  if (!top) return "Waiting for source files.";
  const extras = [data ? "data layer" : "", deploy ? "deploy assets" : ""].filter(Boolean);
  return `${primaryStack}; ${top.label} is ${top.percent}%${extras.length ? ` with ${extras.join(" + ")}` : ""}.`;
}
