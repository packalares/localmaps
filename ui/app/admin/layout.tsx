"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";
import { Home, LogOut, Map, Settings, Wrench } from "lucide-react";
import { useCurrentUser, useLogout } from "@/lib/api/hooks";

/**
 * Admin shell — persistent sidebar + content area + user menu. Gates
 * access: if no session, bounce to /login?rd=<current>. If the user's
 * role is not `admin`, show a small notice (but let the UI load so the
 * individual pages can render their own "unauthorized" banners).
 */
export default function AdminLayout({ children }: { children: ReactNode }) {
  const router = useRouter();
  const me = useCurrentUser();
  const logout = useLogout();

  useEffect(() => {
    if (me.isLoading) return;
    if (me.data?.user) return;
    const here =
      typeof window !== "undefined"
        ? window.location.pathname + window.location.search
        : "/admin";
    router.replace(`/login?rd=${encodeURIComponent(here)}`);
  }, [me.data, me.isLoading, router]);

  const user = me.data?.user;
  const notAdmin = user && user.role !== "admin";

  return (
    <div className="flex h-dvh w-screen overflow-hidden bg-muted/30">
      <aside
        className="flex w-56 shrink-0 flex-col border-r border-border bg-background"
        aria-label="Admin navigation"
      >
        <div className="border-b border-border px-4 py-3 text-sm font-semibold">
          LocalMaps admin
        </div>
        <nav className="flex flex-col gap-1 p-2 text-sm">
          <SidebarLink href="/admin/regions" icon={<Map className="h-4 w-4" />}>
            Regions
          </SidebarLink>
          <SidebarLink
            href="/admin/settings"
            icon={<Settings className="h-4 w-4" />}
          >
            Settings
          </SidebarLink>
          <SidebarLink
            href="/admin/jobs"
            icon={<Wrench className="h-4 w-4" />}
          >
            Jobs
          </SidebarLink>
          <div className="my-2 h-px bg-border" />
          <SidebarLink href="/" icon={<Home className="h-4 w-4" />}>
            Back to map
          </SidebarLink>
        </nav>

        <div className="mt-auto border-t border-border p-3 text-xs">
          {user ? (
            <div className="flex flex-col gap-2">
              <div>
                <div className="font-medium text-foreground">
                  {user.username}
                </div>
                <div className="text-muted-foreground">{user.role}</div>
              </div>
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded border border-border px-2 py-1 text-muted-foreground hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() => {
                  logout.mutate(undefined, {
                    onSuccess: () => router.replace("/login"),
                  });
                }}
              >
                <LogOut className="h-3.5 w-3.5" />
                Sign out
              </button>
            </div>
          ) : null}
        </div>
      </aside>

      <section className="flex-1 overflow-y-auto">
        {notAdmin ? (
          <div
            role="alert"
            className="m-4 rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm"
          >
            Your account role ({user!.role}) cannot modify admin data.
          </div>
        ) : null}
        {children}
      </section>
    </div>
  );
}

function SidebarLink({
  href,
  icon,
  children,
}: {
  href: string;
  icon: ReactNode;
  children: ReactNode;
}) {
  return (
    <Link
      href={href}
      className="flex items-center gap-2 rounded-md px-3 py-2 text-foreground hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      {icon}
      <span>{children}</span>
    </Link>
  );
}
