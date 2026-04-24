/**
 * Rough client-side estimates for "how big is the install" and
 * "how long will it take". The server knows better (it has the real
 * pbf size after download) but the dialog needs a number to show up
 * front.
 *
 * The ratios are deliberately conservative — we would rather surface
 * "this may take 45 minutes" and finish in 20 than the other way
 * around. All ratios are tied to the source pbf size reported by
 * Geofabrik in the catalogue.
 */

/** Multiplier: final on-disk bytes ≈ pbf bytes × this. */
export const DISK_MULTIPLIER = 4.5;

/** Pbf bytes processed per second on a typical single-node deploy. */
export const THROUGHPUT_BYTES_PER_SECOND = 2_500_000;

/** Floor on build duration so tiny regions don't show "<1 min". */
export const MIN_BUILD_SECONDS = 90;

export interface InstallEstimate {
  diskBytes: number | null;
  durationSeconds: number | null;
}

export function estimateInstall(
  sourcePbfBytes: number | null | undefined,
): InstallEstimate {
  if (!sourcePbfBytes || sourcePbfBytes <= 0) {
    return { diskBytes: null, durationSeconds: null };
  }
  const disk = Math.round(sourcePbfBytes * DISK_MULTIPLIER);
  const duration = Math.max(
    MIN_BUILD_SECONDS,
    Math.round(sourcePbfBytes / THROUGHPUT_BYTES_PER_SECOND),
  );
  return { diskBytes: disk, durationSeconds: duration };
}

/** Format a duration given in seconds as "N min" or "Nh Mmin". */
export function formatDurationShort(seconds: number | null): string {
  if (seconds === null || !Number.isFinite(seconds) || seconds < 0) return "—";
  if (seconds < 60) return "<1 min";
  const mins = Math.round(seconds / 60);
  if (mins < 60) return `${mins} min`;
  const h = Math.floor(mins / 60);
  const m = mins - h * 60;
  return m === 0 ? `${h} h` : `${h} h ${m} min`;
}
