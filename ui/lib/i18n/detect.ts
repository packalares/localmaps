import {
  DEFAULT_LOCALE,
  LOCALE_STORAGE_KEY,
  SUPPORTED_LOCALES,
  isLocale,
  type Locale,
} from "./types";

/**
 * Locale-selection precedence, highest to lowest:
 *
 *   1. user's explicit pick from the LocaleSelector (localStorage).
 *   2. `map.language` from `/api/settings` when non-default.
 *   3. browser `navigator.language` (and `languages[]` fallback).
 *   4. hard default `en`.
 *
 * Each helper is pure so the React provider can re-evaluate whenever a
 * source changes (user clicks the selector, settings load, etc.).
 */

export function readStoredLocale(
  storage?: Pick<Storage, "getItem"> | null,
): Locale | null {
  const s =
    storage ??
    (typeof window !== "undefined" ? window.localStorage : null);
  if (!s) return null;
  try {
    const raw = s.getItem(LOCALE_STORAGE_KEY);
    return isLocale(raw) ? raw : null;
  } catch {
    return null;
  }
}

/**
 * Normalise a locale tag to one of SUPPORTED_LOCALES. Accepts both
 * exact matches (`ro`) and BCP-47 tags with a region (`ro-RO`, `en-GB`)
 * — the latter collapses to its primary subtag.
 */
export function matchSupportedLocale(tag: string | null | undefined): Locale | null {
  if (!tag) return null;
  const primary = tag.toLowerCase().split(/[-_]/, 1)[0] ?? "";
  return isLocale(primary) ? (primary as Locale) : null;
}

/** Resolve a locale from the `map.language` setting. `"default"` or
 * missing returns null so the next precedence layer can answer. */
export function localeFromSettings(value: unknown): Locale | null {
  if (typeof value !== "string") return null;
  if (value === "default" || value === "") return null;
  return matchSupportedLocale(value);
}

/** Resolve a locale from the browser. SSR-safe — returns null server-side. */
export function localeFromBrowser(nav?: Pick<Navigator, "language" | "languages"> | null): Locale | null {
  const n =
    nav ?? (typeof navigator !== "undefined" ? navigator : null);
  if (!n) return null;
  const candidates: string[] = [];
  if (n.language) candidates.push(n.language);
  if (Array.isArray(n.languages)) candidates.push(...n.languages);
  for (const c of candidates) {
    const matched = matchSupportedLocale(c);
    if (matched) return matched;
  }
  return null;
}

/**
 * Combines all four precedence layers. Pure — every source is an
 * argument so this is trivially testable.
 */
export interface DetectSources {
  stored?: Locale | null;
  settingsLanguage?: unknown;
  browser?: Locale | null;
}

export function detectLocale(sources: DetectSources = {}): Locale {
  if (sources.stored && isLocale(sources.stored)) return sources.stored;
  const fromSettings = localeFromSettings(sources.settingsLanguage);
  if (fromSettings) return fromSettings;
  if (sources.browser && isLocale(sources.browser)) return sources.browser;
  return DEFAULT_LOCALE;
}

/** Convenience one-liner used by the provider at mount time. */
export function detectInitialLocale(): Locale {
  return detectLocale({
    stored: readStoredLocale(),
    browser: localeFromBrowser(),
  });
}

export { SUPPORTED_LOCALES };
