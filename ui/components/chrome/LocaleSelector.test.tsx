import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import { LocaleProvider } from "@/lib/i18n/provider";
import {
  LOCALE_CHANGED_EVENT,
  LOCALE_STORAGE_KEY,
  type LocaleChangedEventDetail,
} from "@/lib/i18n/types";
import { LocaleSelector } from "./LocaleSelector";

function wrap(children: React.ReactNode) {
  return (
    <TooltipProvider delayDuration={0}>
      <LocaleProvider initialLocale="en">{children}</LocaleProvider>
    </TooltipProvider>
  );
}

describe("<LocaleSelector />", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });
  afterEach(() => {
    window.localStorage.clear();
  });

  it("renders an accessible trigger with the native label", () => {
    render(wrap(<LocaleSelector />));
    const btn = screen.getByRole("button", {
      name: /choose interface language/i,
    });
    expect(btn).toBeInTheDocument();
  });

  it("opens a menu listing all supported locales", async () => {
    const user = userEvent.setup();
    render(wrap(<LocaleSelector />));
    await user.click(
      screen.getByRole("button", { name: /choose interface language/i }),
    );
    expect(
      await screen.findByRole("menuitem", { name: /switch to english/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: /switch to română/i }),
    ).toBeInTheDocument();
  });

  it("persists the picked locale to localStorage", async () => {
    const user = userEvent.setup();
    render(wrap(<LocaleSelector />));
    await user.click(
      screen.getByRole("button", { name: /choose interface language/i }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: /switch to română/i }),
    );
    expect(window.localStorage.getItem(LOCALE_STORAGE_KEY)).toBe("ro");
  });

  it("dispatches a locale.changed event on pick", async () => {
    const user = userEvent.setup();
    const events: LocaleChangedEventDetail[] = [];
    const handler = (e: Event) => {
      events.push((e as CustomEvent<LocaleChangedEventDetail>).detail);
    };
    window.addEventListener(LOCALE_CHANGED_EVENT, handler);
    render(wrap(<LocaleSelector />));
    await user.click(
      screen.getByRole("button", { name: /choose interface language/i }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: /switch to română/i }),
    );
    window.removeEventListener(LOCALE_CHANGED_EVENT, handler);
    expect(events).toEqual([{ locale: "ro" }]);
  });
});
