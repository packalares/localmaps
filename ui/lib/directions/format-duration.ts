/**
 * Human-friendly duration formatter. Mirrors Google Maps ETA text:
 *
 * - < 60 s → "<1 min"
 * - < 60 min → "N min"
 * - >= 60 min → "Hh Mmin" (e.g. "1 h 5 min", "2 h")
 * - >= 24 h → "Nd Hh" (e.g. "1 d 6 h")
 *
 * Input is seconds per the OpenAPI contract.
 */

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "—";
  if (seconds < 60) return "<1 min";

  const totalMinutes = Math.round(seconds / 60);
  if (totalMinutes < 60) return `${totalMinutes} min`;

  const totalHours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes - totalHours * 60;
  if (totalHours < 24) {
    if (minutes === 0) return `${totalHours} h`;
    return `${totalHours} h ${minutes} min`;
  }

  const days = Math.floor(totalHours / 24);
  const hours = totalHours - days * 24;
  if (hours === 0) return `${days} d`;
  return `${days} d ${hours} h`;
}
