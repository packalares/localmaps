import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

/**
 * Vitest runs in a jsdom env so we can render React components + probe
 * localStorage / window APIs. `@/` is aliased to the UI root so that
 * our tsconfig `paths` mapping also works under Vite.
 */
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    include: ["**/*.test.{ts,tsx}"],
    exclude: ["node_modules", ".next", "dist", ".turbo"],
    css: false,
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      reportsDirectory: "../coverage-report/ui",
      include: ["components/**", "lib/**", "app/**"],
      exclude: [
        "**/*.test.{ts,tsx}",
        "**/__tests__/**",
        "**/__fixtures__/**",
        "**/node_modules/**",
      ],
      thresholds: {
        // Phase 7 gate: docs/09-testing.md UI target.
        lines: 50,
        functions: 50,
        branches: 50,
        statements: 50,
      },
    },
  },
});
