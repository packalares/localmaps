/**
 * Human-readable byte formatter, IEC (1024-based) for disk sizes.
 * Rendered as "N unit" with one decimal below 10 and whole numbers
 * above. Returns em-dash for invalid input so callers can feed raw
 * response fields without pre-validating.
 */

const UNITS = ["B", "KB", "MB", "GB", "TB", "PB"] as const;

export function formatBytes(bytes: number | null | undefined): string {
  if (bytes === null || bytes === undefined) return "—";
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes === 0) return "0 B";
  let n = bytes;
  let i = 0;
  while (n >= 1024 && i < UNITS.length - 1) {
    n /= 1024;
    i++;
  }
  if (i === 0) return `${Math.round(n)} ${UNITS[i]}`;
  if (n < 10) return `${n.toFixed(1)} ${UNITS[i]}`;
  return `${Math.round(n)} ${UNITS[i]}`;
}
