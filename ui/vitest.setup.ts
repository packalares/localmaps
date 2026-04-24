import "@testing-library/jest-dom/vitest";

// jsdom doesn't implement matchMedia; components that read it at mount
// time (theme provider, media hooks) rely on this tiny shim.
if (typeof window !== "undefined" && typeof window.matchMedia !== "function") {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

// jsdom doesn't ship ResizeObserver, which cmdk observes. A no-op shim
// is sufficient for unit tests that never assert on resize events.
if (typeof globalThis.ResizeObserver === "undefined") {
  class ResizeObserverShim {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).ResizeObserver = ResizeObserverShim;
}

// jsdom implements scrollIntoView as undefined on Element; cmdk calls
// it when the selected item changes. No-op is fine.
if (
  typeof Element !== "undefined" &&
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  typeof (Element.prototype as any).scrollIntoView !== "function"
) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (Element.prototype as any).scrollIntoView = function () {};
}

// jsdom stubs `URL.createObjectURL` but not reliably across versions;
// maplibre-gl calls it at module load to bootstrap its web-worker, which
// otherwise aborts the whole Vitest run with an uncaught exception.
if (
  typeof window !== "undefined" &&
  typeof window.URL.createObjectURL !== "function"
) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (window.URL as any).createObjectURL = () => "blob:mock";
}
if (
  typeof window !== "undefined" &&
  typeof window.URL.revokeObjectURL !== "function"
) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (window.URL as any).revokeObjectURL = () => {};
}
if (typeof globalThis.Worker === "undefined") {
  class WorkerShim {
    postMessage() {}
    terminate() {}
    addEventListener() {}
    removeEventListener() {}
    onmessage: ((ev: unknown) => void) | null = null;
    onerror: ((ev: unknown) => void) | null = null;
  }
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).Worker = WorkerShim;
}
