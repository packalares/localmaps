import type { Metadata, Viewport } from "next";
import { ReactQueryProvider } from "@/components/providers/react-query";
import { ThemeProvider } from "@/components/providers/theme";
import { Toaster } from "@/components/ui/toaster";
import { TooltipProvider } from "@/components/ui/tooltip";
import { LocaleProvider } from "@/lib/i18n/provider";
import "./globals.css";

// The PWA manifest + service worker registration is intentionally omitted
// here; the login flow issues an HTTP-only session cookie and the
// manifest would be fetched anonymously. When the deploy terminates TLS
// in front of the gateway and handles auth via this app's own session
// cookie, the `manifest` + `<PwaRegister />` entries can be restored.
export const metadata: Metadata = {
  title: "LocalMaps",
  description:
    "Self-hosted maps — search, directions, POIs, and regional data from open sources.",
  applicationName: "LocalMaps",
  appleWebApp: {
    capable: true,
    title: "LocalMaps",
    statusBarStyle: "default",
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#0ea5e9" },
    { media: "(prefers-color-scheme: dark)", color: "#0b1220" },
  ],
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="h-full min-h-dvh bg-background text-foreground">
        <ThemeProvider>
          <LocaleProvider>
            <ReactQueryProvider>
              <TooltipProvider delayDuration={200}>
                <Toaster>{children}</Toaster>
              </TooltipProvider>
            </ReactQueryProvider>
          </LocaleProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
