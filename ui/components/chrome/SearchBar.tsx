"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { CornerUpRight, Menu, Search, X } from "lucide-react";
import { useMapStore, type PoiCategory } from "@/lib/state/map";
import { cn } from "@/lib/utils";
import { SearchPanel } from "@/components/search/SearchPanel";
import { useMessages } from "@/lib/i18n/provider";
import maplibregl from "maplibre-gl";
import {
  CATEGORY_DESCRIPTORS,
  descriptorFor,
} from "@/components/chrome/category-descriptors";

/**
 * Google-Maps-style search pill.
 *
 * The input lives in the left-rail chrome. Behaviour highlights:
 *
 * - Press `/` anywhere on the page (or Cmd/Ctrl-K) to focus.
 * - Focus / click opens the `search` tab of the left rail via the
 *   Zustand store — `openLeftRail('search')`.
 * - Typing fires the debounced `useGeocodeAutocomplete` in the panel.
 * - Enter on a non-empty query switches the panel to the fuller
 *   `/api/geocode/search` endpoint and highlights the top result.
 * - Escape clears the query and dismisses focus.
 *
 * The pill uses the WAI-ARIA 1.2 combobox pattern (role=combobox,
 * aria-autocomplete=list, aria-controls, aria-expanded).
 */
export interface SearchBarProps {
  /** Forwarded to the panel; operator default comes from settings. */
  debounceMs?: number;
  /** Optional lang forwarded to the autocomplete call. */
  lang?: string;
  /** Max results per query; gateway default is 10. */
  limit?: number;
  /**
   * Rendered inside the panel-less chrome. When false, SearchBar is just
   * an input and it's the parent's job to render <SearchPanel/>
   * somewhere. Primary may turn this off when LeftRail integrates tabs.
   */
  embedPanel?: boolean;
}

const LISTBOX_ID = "localmaps-search-results";

/** Serialise the live map bounds → bbox payload. Mirrors PoiSearchChips. */
function bboxFromMap(m: maplibregl.Map | null): string | null {
  if (!m) return null;
  try {
    const b = m.getBounds();
    return `${b.getWest()},${b.getSouth()},${b.getEast()},${b.getNorth()}`;
  } catch {
    return null;
  }
}

/**
 * If `text` exactly matches one of the chip display labels (case- and
 * whitespace-insensitive), return that chip's category key. Partial
 * matches don't qualify — we only auto-activate on a full-name type-in
 * (Change 4).
 */
function chipKeyForExactLabel(text: string): PoiCategory | null {
  const norm = text.trim().toLowerCase();
  if (!norm) return null;
  for (const desc of CATEGORY_DESCRIPTORS) {
    if (desc.label.toLowerCase() === norm) return desc.key;
    if (desc.short.toLowerCase() === norm) return desc.key;
  }
  return null;
}

export function SearchBar({
  debounceMs = 300,
  lang,
  limit,
  embedPanel = true,
}: SearchBarProps = {}) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [query, setQuery] = useState("");
  const [fullSearch, setFullSearch] = useState(false);
  const [isFocused, setIsFocused] = useState(false);

  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const activeCategoryChip = useMapStore((s) => s.activeCategoryChip);
  const runCategorySearch = useMapStore((s) => s.runCategorySearch);
  const closeCategoryResults = useMapStore((s) => s.closeCategoryResults);
  const setSearchDropdownOpen = useMapStore((s) => s.setSearchDropdownOpen);
  const map = useMapStore((s) => s.map);

  // Mirror local isFocused state into the store so SelectedFeatureSync
  // can cascade-close the dropdown on map clicks.
  useEffect(() => {
    setSearchDropdownOpen(isFocused);
    return () => setSearchDropdownOpen(false);
  }, [isFocused, setSearchDropdownOpen]);

  // External clear (e.g. the side-panel close X) bumps this token.
  const searchClearToken = useMapStore((s) => s.searchClearToken);
  useEffect(() => {
    if (searchClearToken === 0) return;
    setQuery("");
    setFullSearch(false);
  }, [searchClearToken]);
  const { t } = useMessages();

  // Shortcut keys: `/` and Cmd/Ctrl-K both focus the input.
  useEffect(() => {
    const handler = (e: globalThis.KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const inEditable =
        !!target &&
        (target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.isContentEditable);
      if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        inputRef.current?.focus();
        openLeftRail("search");
        return;
      }
      if (e.key === "/" && !inEditable) {
        e.preventDefault();
        inputRef.current?.focus();
        openLeftRail("search");
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [openLeftRail]);

  const handleFocus = useCallback(() => {
    setIsFocused(true);
    openLeftRail("search");
    // Restore autocomplete mode whenever focus returns.
    setFullSearch(false);
  }, [openLeftRail]);

  const tryActivateChipFromText = useCallback(
    (text: string): boolean => {
      const matched = chipKeyForExactLabel(text);
      if (!matched) return false;
      runCategorySearch(matched, bboxFromMap(map));
      setQuery("");
      return true;
    },
    [map, runCategorySearch],
  );

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Escape") {
        e.preventDefault();
        setQuery("");
        setFullSearch(false);
        inputRef.current?.blur();
        return;
      }
      if (e.key === "Enter") {
        const trimmed = query.trim();
        if (trimmed.length > 0) {
          e.preventDefault();
          // Auto-activate the chip ONLY when the user explicitly hits
          // Enter on a full chip-label (Change-12). Otherwise mid-typing
          // "Food market" would silently swap to the Food chip search.
          if (tryActivateChipFromText(trimmed)) return;
          setFullSearch(true);
        }
      }
      if (e.key === "ArrowDown") {
        // Keyboard nav is delegated to SearchPanel; prevent the cursor
        // from wandering inside the input.
        e.preventDefault();
      }
    },
    [query, tryActivateChipFromText],
  );

  const handleBlur = useCallback(() => {
    setIsFocused(false);
    // Same fall-through as Enter — when the user types a full chip
    // label and tabs/clicks away, activate the chip. Match is
    // case-insensitive and trims surrounding whitespace, so the
    // "Hotels " case still flips to the lodging chip.
    const trimmed = query.trim();
    if (trimmed.length > 0) tryActivateChipFromText(trimmed);
  }, [query, tryActivateChipFromText]);

  const handleClear = useCallback(() => {
    setQuery("");
    setFullSearch(false);
    // If a chip is active, clearing the bar = full reset (Change 3 +
    // closing-the-result-panel = full reset).
    if (activeCategoryChip) closeCategoryResults();
    inputRef.current?.focus();
  }, [activeCategoryChip, closeCategoryResults]);

  // Dropdown is visible only while the input is focused. Selecting a
  // result, clicking outside, or pressing Escape blurs the input and
  // hides it. This mirrors Google Maps desktop.
  const showDropdown = embedPanel && isFocused;

  // Combobox `aria-expanded` must reflect ACTUAL listbox visibility —
  // not just "is the search tab the active rail". Otherwise a screen
  // reader hears "expanded" while no listbox exists.
  const isOpen = showDropdown;

  // Outside-click closes the dropdown. We watch pointerdown on the
  // document; if the click lands outside the wrapper, blur the input.
  const wrapperRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!isFocused) return;
    const onDown = (e: PointerEvent) => {
      const root = wrapperRef.current;
      if (!root) return;
      if (!root.contains(e.target as Node)) {
        setIsFocused(false);
        inputRef.current?.blur();
      }
    };
    document.addEventListener("pointerdown", onDown);
    return () => document.removeEventListener("pointerdown", onDown);
  }, [isFocused]);

  return (
    <div ref={wrapperRef} className="flex w-full flex-col">
      <div
        className={cn(
          "chrome-surface-sm pointer-events-auto flex h-12 w-full items-center gap-2 rounded-full px-2",
        )}
      >
        <button
          type="button"
          onClick={() => openLeftRail("recents")}
          className="inline-flex h-8 w-8 items-center justify-center rounded-full hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t("search.openMenu")}
          title={t("search.openMenu")}
        >
          <Menu className="h-4 w-4" aria-hidden="true" />
        </button>
        <div
          role="combobox"
          aria-expanded={isOpen}
          aria-controls={LISTBOX_ID}
          aria-haspopup="listbox"
          aria-owns={LISTBOX_ID}
          className="flex flex-1 items-center gap-2"
        >
          <Search
            className="h-4 w-4 text-muted-foreground"
            aria-hidden="true"
          />
          <input
            ref={inputRef}
            type="text"
            role="searchbox"
            inputMode="search"
            autoComplete="off"
            spellCheck={false}
            // While a chip is active, the input mirrors the chip's
            // display label so the user sees what they're searching.
            // Typing replaces the chip with a free-text search.
            value={
              activeCategoryChip
                ? descriptorFor(activeCategoryChip).label
                : query
            }
            onChange={(e) => {
              let next = e.target.value;
              if (activeCategoryChip) {
                const label = descriptorFor(activeCategoryChip).label;
                if (next.startsWith(label)) next = next.slice(label.length);
                closeCategoryResults();
              }
              setQuery(next);
              setFullSearch(false);
              // No chip auto-activation on every keystroke — that path
              // was racing the user mid-type. The Enter / blur handlers
              // own that flip now (Change-12).
            }}
            onFocus={handleFocus}
            onBlur={handleBlur}
            onKeyDown={handleKeyDown}
            placeholder={t("search.placeholder")}
            aria-label={t("search.ariaLabel")}
            aria-autocomplete="list"
            aria-controls={LISTBOX_ID}
            className={cn(
              "flex-1 bg-transparent text-[15px] outline-none placeholder:text-muted-foreground",
            )}
          />
          {(activeCategoryChip || query.length > 0) ? (
            <button
              type="button"
              onClick={handleClear}
              aria-label={t("search.clear")}
              className={cn(
                "inline-flex h-6 w-6 items-center justify-center rounded-full text-muted-foreground",
                "hover:bg-neutral-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring dark:hover:bg-neutral-800",
              )}
            >
              <X className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          ) : null}
        </div>
        <button
          type="button"
          onClick={() => openLeftRail("directions")}
          aria-label={t("search.directions")}
          title={t("search.directions")}
          className={cn(
            "inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-primary",
            "hover:bg-primary/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        >
          <CornerUpRight className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>

      {showDropdown ? (
        <div
          id={LISTBOX_ID}
          // Prevent the input's onBlur firing before the click on a
          // result registers — without this, isFocused flips to false
          // first and the dropdown unmounts mid-click, swallowing the
          // selection.
          onMouseDown={(e) => e.preventDefault()}
          className={cn(
            "chrome-surface-lg pointer-events-auto mt-1 max-h-[70vh] overflow-hidden rounded-xl",
          )}
        >
          <SearchPanel
            query={query}
            debounceMs={debounceMs}
            lang={lang}
            limit={limit}
            fullSearch={fullSearch}
            onResultSelected={(r) => {
              setQuery(r.label);
              setFullSearch(false);
              // Close the dropdown after a pick: blur the input so
              // showDropdown flips to false.
              setIsFocused(false);
              inputRef.current?.blur();
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
