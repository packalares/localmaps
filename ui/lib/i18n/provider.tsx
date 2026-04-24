"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { detectInitialLocale, localeFromSettings } from "./detect";
import { createTranslator, type MessageParams } from "./format";
import {
  DEFAULT_LOCALE,
  LOCALE_CHANGED_EVENT,
  LOCALE_STORAGE_KEY,
  isLocale,
  type Locale,
  type LocaleChangedEventDetail,
  type MessageKey,
} from "./types";

/**
 * React plumbing for i18n. The heavy lifting (lookup, interpolation,
 * detection precedence) lives in the pure `format.ts` / `detect.ts`
 * helpers, so this file stays a thin context wrapper.
 *
 * Design notes:
 * - Server-rendered initial locale is `DEFAULT_LOCALE` to keep markup
 *   deterministic. A client-only effect swaps to the detected locale
 *   after hydration; `suppressHydrationWarning` on the layout's <html>
 *   absorbs the one-frame mismatch, same pattern as the theme provider.
 * - The provider listens for the global `locale.changed` event so the
 *   LocaleSelector (or any imperative caller) can swap locale without
 *   owning a direct reference to the context.
 */

export interface LocaleContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: MessageKey | string, params?: MessageParams) => string;
}

const LocaleContext = createContext<LocaleContextValue | null>(null);

export interface LocaleProviderProps {
  children: ReactNode;
  /** Override for tests / storybook / SSR. */
  initialLocale?: Locale;
  /** `map.language` from `/api/settings`; when present, precedence #2. */
  settingsLanguage?: unknown;
}

export function LocaleProvider({
  children,
  initialLocale,
  settingsLanguage,
}: LocaleProviderProps) {
  const [locale, setLocaleState] = useState<Locale>(
    initialLocale ?? DEFAULT_LOCALE,
  );

  // Hydration pass: upgrade from SSR-safe default to the detected locale
  // once we're in the browser. `initialLocale` wins when provided.
  useEffect(() => {
    if (initialLocale) return;
    const next = detectInitialLocale();
    if (next !== locale) setLocaleState(next);
    // Only on mount — subsequent changes flow via setLocale / event.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Settings-driven layer: when the user has no explicit pick stored
  // and settings supply a non-default `map.language`, adopt it.
  useEffect(() => {
    if (typeof window === "undefined") return;
    try {
      const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
      if (stored && isLocale(stored)) return;
    } catch {
      // localStorage unavailable; fall through.
    }
    const fromSettings = localeFromSettings(settingsLanguage);
    if (fromSettings && fromSettings !== locale) {
      setLocaleState(fromSettings);
    }
  }, [settingsLanguage, locale]);

  // Cross-component broadcast channel.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const handler = (e: Event) => {
      const detail = (e as CustomEvent<LocaleChangedEventDetail>).detail;
      if (detail && isLocale(detail.locale) && detail.locale !== locale) {
        setLocaleState(detail.locale);
      }
    };
    window.addEventListener(LOCALE_CHANGED_EVENT, handler);
    return () => window.removeEventListener(LOCALE_CHANGED_EVENT, handler);
  }, [locale]);

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    if (typeof window !== "undefined") {
      try {
        window.localStorage.setItem(LOCALE_STORAGE_KEY, next);
      } catch {
        // Ignore (private mode, etc.).
      }
      window.dispatchEvent(
        new CustomEvent<LocaleChangedEventDetail>(LOCALE_CHANGED_EVENT, {
          detail: { locale: next },
        }),
      );
    }
  }, []);

  const value = useMemo<LocaleContextValue>(() => {
    const t = createTranslator(locale);
    return { locale, setLocale, t };
  }, [locale, setLocale]);

  return (
    <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>
  );
}

/**
 * Primary hook for components. When no provider is mounted (e.g. unit
 * tests that render a leaf component without wrapping), we degrade to a
 * `DEFAULT_LOCALE` translator so the UI never hard-crashes. That
 * mirrors the theme provider's "throw on missing" contract but skews
 * looser because i18n is advisory — a missing wrapper is strictly a
 * test ergonomics thing.
 */
export function useMessages(): LocaleContextValue {
  const ctx = useContext(LocaleContext);
  if (ctx) return ctx;
  return {
    locale: DEFAULT_LOCALE,
    setLocale: () => {
      /* no-op without provider */
    },
    t: createTranslator(DEFAULT_LOCALE),
  };
}
