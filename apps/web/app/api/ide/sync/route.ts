import { createHash } from "node:crypto";
import { mkdir, readdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

const MAX_FILES = 1000;
const MAX_FILE_BYTES = 2 * 1024 * 1024;
const MAX_TOTAL_BYTES = 25 * 1024 * 1024;

interface SyncFile {
  path?: unknown;
  content?: unknown;
}

interface SyncBody {
  projectID?: unknown;
  files?: unknown;
}

interface SyncManifest {
  projectID: string;
  syncedAt: string;
  files: Record<string, { hash: string }>;
}

export async function POST(req: NextRequest) {
  if (!req.cookies.get("ironflyer_token")?.value) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  let body: SyncBody;
  try {
    body = (await req.json()) as SyncBody;
  } catch {
    return NextResponse.json({ error: "invalid json" }, { status: 400 });
  }

  const projectID =
    typeof body.projectID === "string" ? body.projectID.trim() : "";
  if (!projectID) {
    return NextResponse.json({ error: "missing projectID" }, { status: 400 });
  }
  if (!Array.isArray(body.files)) {
    return NextResponse.json({ error: "files must be an array" }, { status: 400 });
  }
  if (body.files.length > MAX_FILES) {
    return NextResponse.json({ error: "too many files" }, { status: 413 });
  }

  const root = syncRoot();
  const projectRoot = path.join(root, projectFolderName(projectID));
  assertInside(root, projectRoot);

  await mkdir(projectRoot, { recursive: true });
  const oldManifest = await readManifest(projectRoot, projectID);
  const nextManifest: SyncManifest = {
    projectID,
    syncedAt: new Date().toISOString(),
    files: { ...oldManifest.files },
  };

  let written = 0;
  let skipped = 0;
  let preserved = 0;
  let removed = 0;
  let totalBytes = 0;
  const skippedPaths: string[] = [];
  const preservedPaths: string[] = [];
  const incomingPaths = new Set<string>();

  for (const entry of body.files as SyncFile[]) {
    const rel = typeof entry.path === "string" ? cleanRelativePath(entry.path) : "";
    const content = typeof entry.content === "string" ? entry.content : null;
    if (!rel || content == null) {
      skipped += 1;
      if (rel) skippedPaths.push(rel);
      continue;
    }

    const bytes = Buffer.byteLength(content, "utf8");
    if (bytes > MAX_FILE_BYTES || totalBytes + bytes > MAX_TOTAL_BYTES) {
      skipped += 1;
      skippedPaths.push(rel);
      continue;
    }
    totalBytes += bytes;
    incomingPaths.add(rel);

    const target = path.join(projectRoot, rel);
    assertInside(projectRoot, target);
    const incomingHash = hashText(content);
    const existing = await readTextIfExists(target);
    const previousHash = oldManifest.files[rel]?.hash;
    if (existing != null && previousHash) {
      const existingHash = hashText(existing);
      if (existingHash !== previousHash && existingHash !== incomingHash) {
        preserved += 1;
        preservedPaths.push(rel);
        continue;
      }
    }

    await mkdir(path.dirname(target), { recursive: true });
    await writeFile(target, content, "utf8");
    nextManifest.files[rel] = { hash: incomingHash };
    written += 1;
  }

  for (const rel of Object.keys(oldManifest.files)) {
    if (incomingPaths.has(rel)) continue;
    const target = path.join(projectRoot, rel);
    assertInside(projectRoot, target);
    const existing = await readTextIfExists(target);
    if (existing == null) {
      delete nextManifest.files[rel];
      continue;
    }
    if (hashText(existing) === oldManifest.files[rel]?.hash) {
      await rm(target, { force: true });
      delete nextManifest.files[rel];
      removed += 1;
    } else {
      preserved += 1;
      preservedPaths.push(rel);
    }
  }

  await writeFile(
    path.join(projectRoot, ".ironflyer-sync.json"),
    JSON.stringify(
      {
        ...nextManifest,
        written,
        skipped,
        preserved,
        removed,
        skippedPaths,
        preservedPaths,
      },
      null,
      2,
    ),
    "utf8",
  );

  return NextResponse.json({
    folder: `/home/workspace/projects/${projectFolderName(projectID)}`,
    written,
    skipped,
    preserved,
    removed,
  });
}

export async function GET(req: NextRequest) {
  if (!req.cookies.get("ironflyer_token")?.value) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const projectID = req.nextUrl.searchParams.get("projectID")?.trim() ?? "";
  if (!projectID) {
    return NextResponse.json({ error: "missing projectID" }, { status: 400 });
  }

  const root = syncRoot();
  const projectRoot = path.join(root, projectFolderName(projectID));
  assertInside(root, projectRoot);

  const files: Array<{
    path: string;
    content: string;
    size: number;
    updatedAt: string;
  }> = [];

  await walkProject(projectRoot, projectRoot, files);
  return NextResponse.json({
    folder: `/home/workspace/projects/${projectFolderName(projectID)}`,
    files,
  });
}

function syncRoot(): string {
  return path.resolve(
    process.env.IRONFLYER_IDE_SYNC_ROOT ??
      path.join(process.cwd(), "../../.ironflyer/ide-sync/projects"),
  );
}

function projectFolderName(projectID: string): string {
  return projectID.replace(/[^a-zA-Z0-9._-]+/g, "-").slice(0, 80) || "project";
}

function cleanRelativePath(raw: string): string {
  const normalized = raw.replaceAll("\\", "/").replace(/^\/+/, "");
  const cleaned = path.posix.normalize(normalized);
  if (!cleaned || cleaned === "." || cleaned.startsWith("../")) return "";
  return cleaned;
}

function assertInside(root: string, target: string): void {
  const rel = path.relative(path.resolve(root), path.resolve(target));
  if (rel.startsWith("..") || path.isAbsolute(rel)) {
    throw new Error("path escapes sync root");
  }
}

async function readManifest(root: string, projectID: string): Promise<SyncManifest> {
  const fallback: SyncManifest = {
    projectID,
    syncedAt: "",
    files: {},
  };
  try {
    const raw = await readFile(path.join(root, ".ironflyer-sync.json"), "utf8");
    const parsed = JSON.parse(raw) as Partial<SyncManifest>;
    if (!parsed || typeof parsed !== "object" || !parsed.files) return fallback;
    return {
      projectID,
      syncedAt: typeof parsed.syncedAt === "string" ? parsed.syncedAt : "",
      files: parsed.files,
    };
  } catch {
    return fallback;
  }
}

async function readTextIfExists(file: string): Promise<string | null> {
  try {
    return await readFile(file, "utf8");
  } catch {
    return null;
  }
}

function hashText(content: string): string {
  return createHash("sha256").update(content).digest("hex");
}

async function walkProject(
  root: string,
  dir: string,
  out: Array<{ path: string; content: string; size: number; updatedAt: string }>,
): Promise<void> {
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch {
    return;
  }

  for (const entry of entries) {
    if (out.length >= MAX_FILES) return;
    if (entry.name === ".git" || entry.name === "node_modules") continue;
    const abs = path.join(dir, entry.name);
    assertInside(root, abs);
    if (entry.isDirectory()) {
      await walkProject(root, abs, out);
      continue;
    }
    if (!entry.isFile() || entry.name === ".ironflyer-sync.json") continue;
    const rel = path.relative(root, abs).replaceAll(path.sep, "/");
    const info = await stat(abs);
    if (info.size > MAX_FILE_BYTES) continue;
    const buf = await readFile(abs);
    if (buf.includes(0)) continue;
    out.push({
      path: rel,
      content: buf.toString("utf8"),
      size: info.size,
      updatedAt: info.mtime.toISOString(),
    });
  }
}
