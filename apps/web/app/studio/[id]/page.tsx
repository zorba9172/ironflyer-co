// /studio/[id] → /p/[id] alias. The canonical studio route lives at
// /p/[projectID]; this redirect keeps legacy links (Dashboard,
// Templates, the empty-state recents list) working until they're all
// migrated to the canonical path.

import { redirect } from "next/navigation";

export default async function Page({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { id } = await params;
  const sp = (await searchParams) ?? {};
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(sp)) {
    if (typeof v === "string") qs.set(k, v);
    else if (Array.isArray(v)) for (const item of v) qs.append(k, item);
  }
  const tail = qs.toString();
  redirect(`/p/${encodeURIComponent(id)}${tail ? `?${tail}` : ""}`);
}
