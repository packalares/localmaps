import { describe, expect, it } from "vitest";
import {
  detectLocale,
  localeFromBrowser,
  localeFromSettings,
  matchSupportedLocale,
  readStoredLocale,
} from "./detect";
import { LOCALE_STORAGE_KEY } from "./types";

describe("matchSupportedLocale", () => {
  it("maps BCP-47 tags down to the primary subtag", () => {
    expect(matchSupportedLocale("ro-RO")).toBe("ro");
    expect(matchSupportedLocale("en-GB")).toBe("en");
    expect(matchSupportedLocale("en_US")).toBe("en");
  });

  it("returns null for unsupported locales", () => {
    expect(matchSupportedLocale("fr")).toBeNull();
    expect(matchSupportedLocale("zh-CN")).toBeNull();
    expect(matchSupportedLocale("")).toBeNull();
    expect(matchSupportedLocale(null)).toBeNull();
  });
});

describe("localeFromSettings", () => {
  it("returns null for the `default` sentinel", () => {
    expect(localeFromSettings("default")).toBeNull();
    expect(localeFromSettings("")).toBeNull();
    expect(localeFromSettings(undefined)).toBeNull();
  });

  it("returns a supported locale for real language codes", () => {
    expect(localeFromSettings("ro")).toBe("ro");
    expect(localeFromSettings("en")).toBe("en");
  });

  it("ignores unsupported codes — detection falls through", () => {
    expect(localeFromSettings("ja")).toBeNull();
    expect(localeFromSettings(42)).toBeNull();
  });
});

describe("localeFromBrowser", () => {
  it("reads navigator.language first", () => {
    const nav = { language: "ro-RO", languages: ["en-US"] };
    expect(localeFromBrowser(nav)).toBe("ro");
  });

  it("falls through to navigator.languages when primary is unsupported", () => {
    const nav = { language: "fr-FR", languages: ["fr-FR", "ro-RO"] };
    expect(localeFromBrowser(nav)).toBe("ro");
  });

  it("returns null when nothing matches", () => {
    const nav = { language: "ja-JP", languages: ["ja-JP"] };
    expect(localeFromBrowser(nav)).toBeNull();
  });
});

describe("readStoredLocale", () => {
  it("returns a supported stored value", () => {
    const getItem = (key: string) =>
      key === LOCALE_STORAGE_KEY ? "ro" : null;
    expect(readStoredLocale({ getItem })).toBe("ro");
  });

  it("rejects corrupt / unsupported stored values", () => {
    const getItem = () => "klingon";
    expect(readStoredLocale({ getItem })).toBeNull();
  });

  it("survives a throwing storage", () => {
    const getItem = () => {
      throw new Error("blocked");
    };
    expect(readStoredLocale({ getItem })).toBeNull();
  });
});

describe("detectLocale (precedence)", () => {
  it("explicit stored pick wins over everything", () => {
    expect(
      detectLocale({
        stored: "ro",
        settingsLanguage: "en",
        browser: "en",
      }),
    ).toBe("ro");
  });

  it("settings.language wins over browser when nothing stored", () => {
    expect(
      detectLocale({
        stored: null,
        settingsLanguage: "ro",
        browser: "en",
      }),
    ).toBe("ro");
  });

  it("`default` settings value yields to the browser", () => {
    expect(
      detectLocale({
        stored: null,
        settingsLanguage: "default",
        browser: "ro",
      }),
    ).toBe("ro");
  });

  it("falls back to English when nothing resolves", () => {
    expect(detectLocale({})).toBe("en");
    expect(
      detectLocale({
        stored: null,
        settingsLanguage: "default",
        browser: null,
      }),
    ).toBe("en");
  });
});
