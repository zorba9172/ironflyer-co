// Small set of formatters used across the cockpit. Kept dependency-free
// so server components and client components can both call into them.

const USD = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const USD_COMPACT = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  notation: "compact",
  maximumFractionDigits: 1,
});

const NUMBER = new Intl.NumberFormat("en-US");

const PERCENT = new Intl.NumberFormat("en-US", {
  style: "percent",
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

// formatMoney — "$12.34". Accepts string (Decimal scalar) or number.
export function formatMoney(value: number | string | null | undefined): string {
  const n = toNumber(value);
  if (n === null) return "—";
  return USD.format(n);
}

// formatMoneyCompact — "$1.2K" / "$3.4M" — for headline metrics where
// precision past the first decimal hurts readability.
export function formatMoneyCompact(value: number | string | null | undefined): string {
  const n = toNumber(value);
  if (n === null) return "—";
  return USD_COMPACT.format(n);
}

// formatNumber — "1,234,567". Pass nulls through as em-dash.
export function formatNumber(value: number | string | null | undefined): string {
  const n = toNumber(value);
  if (n === null) return "—";
  return NUMBER.format(n);
}

// formatPercent — input is 0..1 OR 0..100 depending on `basis`.
// Dashboards return grossMarginPct as 0..100; we default to that.
export function formatPercent(
  value: number | string | null | undefined,
  basis: "fraction" | "percent" = "percent",
): string {
  const n = toNumber(value);
  if (n === null) return "—";
  const f = basis === "percent" ? n / 100 : n;
  return PERCENT.format(f);
}

// formatDateTime — ISO string → "May 24, 2026, 11:42 AM" in caller's
// timezone.
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

// formatDate — "May 24, 2026"
export function formatDate(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

// formatDuration — seconds → "3m 12s" / "1h 04m" / "12s"
export function formatDuration(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || Number.isNaN(seconds)) return "—";
  const s = Math.max(0, Math.round(seconds));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rs = s % 60;
  if (m < 60) return `${m}m ${rs.toString().padStart(2, "0")}s`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  return `${h}h ${rm.toString().padStart(2, "0")}m`;
}

function toNumber(value: number | string | null | undefined): number | null {
  if (value === null || value === undefined || value === "") return null;
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : null;
}
