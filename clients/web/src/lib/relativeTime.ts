// Tiny relative time formatter. "just now" / "5m ago" / "2h ago" /
// "3d ago" / falls back to a long date past two weeks. Future dates
// invert to "in 5m".

const FMT = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

export function relativeTime(value: string | Date | null | undefined): string {
  if (!value) return "—";
  const d = typeof value === "string" ? new Date(value) : value;
  if (Number.isNaN(d.getTime())) return "—";

  const diffMs = d.getTime() - Date.now();
  const abs = Math.abs(diffMs);
  const sec = Math.round(diffMs / 1000);

  if (abs < 45 * 1000) return "just now";
  if (abs < 60 * 60 * 1000) return FMT.format(Math.round(sec / 60), "minute");
  if (abs < 24 * 60 * 60 * 1000) return FMT.format(Math.round(sec / 3600), "hour");
  if (abs < 14 * 24 * 60 * 60 * 1000) return FMT.format(Math.round(sec / 86400), "day");

  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}
