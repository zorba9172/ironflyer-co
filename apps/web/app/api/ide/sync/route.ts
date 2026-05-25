import { mkdir, rm, writeFile } from "node:fs/promises";
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

  await rm(projectRoot, { force: true, recursive: true });
  await mkdir(projectRoot, { recursive: true });

  let written = 0;
  let skipped = 0;
  let totalBytes = 0;
  const skippedPaths: string[] = [];

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

    const target = path.join(projectRoot, rel);
    assertInside(projectRoot, target);
    await mkdir(path.dirname(target), { recursive: true });
    await writeFile(target, content, "utf8");
    written += 1;
  }

  await writeFile(
    path.join(projectRoot, ".ironflyer-sync.json"),
    JSON.stringify(
      {
        projectID,
        syncedAt: new Date().toISOString(),
        written,
        skipped,
        skippedPaths,
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
