"use client";

import { Suspense, useEffect, useState, type FormEvent } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ApiError } from "@/lib/api/client";
import { useCurrentUser, useLogin } from "@/lib/api/hooks";

/**
 * /login — native username + password sign-in for the LocalMaps admin.
 *
 * Flow: POST /api/auth/login, the server sets the HttpOnly session
 * cookie; we then navigate to `?rd=<path>` (defaulting to `/`).
 */
export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <LoginForm />
    </Suspense>
  );
}

function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const me = useCurrentUser();
  const login = useLogin();

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);

  const rd = params.get("rd") || "/";

  // If a session already exists, bounce straight to the target.
  useEffect(() => {
    if (me.data?.user) {
      router.replace(rd);
    }
  }, [me.data, rd, router]);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await login.mutateAsync({ username, password });
      router.replace(rd);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || "Sign-in failed");
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("Sign-in failed");
      }
    }
  };

  return (
    <main className="flex min-h-dvh items-center justify-center bg-muted/30 p-6">
      <form
        onSubmit={onSubmit}
        className="w-full max-w-sm space-y-4 rounded-lg border border-border bg-background p-6 shadow-sm"
        aria-label="Sign in"
      >
        <div className="space-y-1">
          <h1 className="text-xl font-semibold">Sign in</h1>
          <p className="text-sm text-muted-foreground">
            Enter your LocalMaps admin credentials.
          </p>
        </div>

        <label className="block text-sm font-medium">
          Username
          <input
            className="mt-1 block w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            name="username"
            autoComplete="username"
            required
            minLength={1}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
        </label>

        <label className="block text-sm font-medium">
          Password
          <input
            className="mt-1 block w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            name="password"
            type="password"
            autoComplete="current-password"
            required
            minLength={1}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>

        {error ? (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}

        <button
          type="submit"
          disabled={login.isPending}
          className="inline-flex w-full items-center justify-center rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground shadow-sm disabled:opacity-60"
        >
          {login.isPending ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </main>
  );
}
