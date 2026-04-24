/**
 * Trigger a browser download of a string payload as a named file.
 * Stays out of the render path so it can be called from anywhere;
 * guards against SSR where `document` is undefined.
 */

export interface DownloadBlobOptions {
  filename: string;
  content: string;
  mimeType: string;
}

export function downloadBlob({
  filename,
  content,
  mimeType,
}: DownloadBlobOptions): void {
  if (typeof document === "undefined") return;
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Give the browser a tick to start the download before reclaiming.
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
