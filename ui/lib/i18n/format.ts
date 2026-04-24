import {
  DEFAULT_LOCALE,
  DICTIONARIES,
  type Dictionary,
  type Locale,
  type MessageKey,
} from "./types";

/**
 * ICU-lite: we only support `{name}` interpolation. No plural/select in
 * v1 — callers that need count-sensitive strings pick one of two keys
 * (e.g. `search.status.resultCount` vs `...CountOne`) and format them
 * with this helper.
 *
 * This module has no React dependency so it can be reused from
 * non-component code (e.g. error-translation helpers) and is trivial to
 * unit-test.
 */
export type MessageParams = Record<string, string | number>;

/** Development-only warning counter — collapses the same missing key
 * into a single console line so a forgotten translation doesn't flood
 * the DevTools console. Exposed as a module-local map; the test suite
 * clears it between runs via `__resetMissingKeyWarnings()`. */
const warnedKeys = new Set<string>();

export function __resetMissingKeyWarnings(): void {
  warnedKeys.clear();
}

function warnMissing(key: string, locale: Locale): void {
  if (typeof process !== "undefined" && process.env?.NODE_ENV === "test") {
    // Tests watch `__lastMissingKey` instead of stderr.
    return;
  }
  const id = `${locale}:${key}`;
  if (warnedKeys.has(id)) return;
  warnedKeys.add(id);
  if (typeof console !== "undefined" && console.warn) {
    console.warn(
      `[i18n] missing translation for key "${key}" in locale "${locale}"`,
    );
  }
}

/**
 * Replace every `{token}` in `template` with the matching `params`
 * value. Tokens without a matching param are left verbatim (a visible
 * `{token}` in the UI surfaces the bug faster than silently blanking).
 */
export function interpolate(template: string, params?: MessageParams): string {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (match, token: string) => {
    const value = params[token];
    if (value === undefined || value === null) return match;
    return String(value);
  });
}

/**
 * Lookup order:
 *   1. the active locale's dictionary
 *   2. the default (English) dictionary
 *   3. the key itself (after a dev-mode warning)
 *
 * Returning the key string means a missing translation is visible but
 * non-fatal — the UI never crashes because of a typo or a locale race.
 */
export function translate(
  locale: Locale,
  key: MessageKey | string,
  params?: MessageParams,
): string {
  const dict = DICTIONARIES[locale] as Dictionary | undefined;
  const fallback = DICTIONARIES[DEFAULT_LOCALE];
  const template =
    (dict && (dict as Record<string, string>)[key]) ??
    (fallback as Record<string, string>)[key];
  if (template === undefined) {
    warnMissing(String(key), locale);
    return interpolate(String(key), params);
  }
  return interpolate(template, params);
}

/** Factory used by the React provider; keeping this pure simplifies
 * both unit tests and SSR consumers. */
export function createTranslator(
  locale: Locale,
): (key: MessageKey | string, params?: MessageParams) => string {
  return (key, params) => translate(locale, key, params);
}
