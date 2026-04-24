"use client";

import { useState, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError } from "@/lib/api/client";

/**
 * Wraps the app in a TanStack Query client. The client is lazily created
 * inside the component so it is not shared across React re-renders during
 * Next.js streaming.
 *
 * Errors that are explicitly non-retryable (per our `ApiError.retryable`
 * flag) short-circuit retries to avoid hammering a stable 404 / 400.
 */
export function ReactQueryProvider({ children }: { children: ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            refetchOnWindowFocus: false,
            retry: (failureCount, error) => {
              if (error instanceof ApiError && !error.retryable) return false;
              return failureCount < 2;
            },
            staleTime: 30_000,
          },
        },
      }),
  );

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
