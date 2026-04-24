import en from "./locales/en.json";
import ro from "./locales/ro.json";

/**
 * Supported locales. Adding a new locale means:
 *   1. drop a `locales/<code>.json` with every key from `en.json`
 *   2. add the code here
 *   3. add the native name in `LOCALE_NATIVE_NAMES`
 * The `MessageKey` union below is derived from `en.json`, so any missing
 * key in another dictionary surfaces as a typecheck error on the
 * `Dictionary<MessageKey>` assertions in `provider.tsx`.
 */
export const SUPPORTED_LOCALES = ["en", "ro"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];

export const DEFAULT_LOCALE: Locale = "en";

/**
 * `en.json` is the canonical source of truth for the set of message
 * keys. Keeping the type narrow means:
 *   - `useMessages().t("searh.placeholder")` is a compile error
 *   - `ro.json` missing a key is a compile error
 *
 * JSON imports are typed as their literal shape by TypeScript, so
 * `keyof typeof en` is precisely the set of keys in the canonical file.
 */
export type MessageKey = keyof typeof en;

/**
 * A locale dictionary is exactly the set of `MessageKey` strings. The
 * `satisfies` check below is what enforces parity between `en.json` and
 * any peer dictionary at typecheck time.
 */
export type Dictionary = Record<MessageKey, string>;

// These assertions are purely structural — TS flags missing keys here.
export const EN_DICTIONARY: Dictionary = en satisfies Dictionary;
export const RO_DICTIONARY: Dictionary = ro satisfies Dictionary;

export const DICTIONARIES: Record<Locale, Dictionary> = {
  en: EN_DICTIONARY,
  ro: RO_DICTIONARY,
};

/**
 * Native labels shown in the LocaleSelector — intentionally hard-coded
 * rather than localised, because a user scanning the menu wants to see
 * "Română" even if the UI is in English.
 */
export const LOCALE_NATIVE_NAMES: Record<Locale, string> = {
  en: "English",
  ro: "Română",
};

/** Single source for the localStorage key — prevents typos at callsites. */
export const LOCALE_STORAGE_KEY = "localmaps.locale";

/** Event name dispatched when the user picks a new locale. */
export const LOCALE_CHANGED_EVENT = "locale.changed";

export interface LocaleChangedEventDetail {
  locale: Locale;
}

export function isLocale(value: string | null | undefined): value is Locale {
  if (!value) return false;
  return (SUPPORTED_LOCALES as readonly string[]).includes(value);
}
