"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { Menu, Search, X } from "lucide-react";
import { useMapStore } from "@/lib/state/map";
import { cn } from "@/lib/utils";
import { SearchPanel } from "@/components/search/SearchPanel";
import { useMessages } from "@/lib/i18n/provider";

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
  const leftRailTab = useMapStore((s) => s.leftRailTab);
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
        if (query.trim().length > 0) {
          e.preventDefault();
          setFullSearch(true);
        }
      }
      if (e.key === "ArrowDown") {
        // Keyboard nav is delegated to SearchPanel; prevent the cursor
        // from wandering inside the input.
        e.preventDefault();
      }
    },
    [query],
  );

  const handleClear = useCallback(() => {
    setQuery("");
    setFullSearch(false);
    inputRef.current?.focus();
  }, []);

  const isOpen = useMemo(
    () => isFocused || leftRailTab === "search",
    [isFocused, leftRailTab],
  );

  return (
    <div className="flex flex-col gap-2">
      <div
        className={cn(
          "pointer-events-auto chrome-card flex w-full items-center gap-2 px-2 py-1.5",
        )}
      >
        <button
          type="button"
          className="inline-flex h-8 w-8 items-center justify-center rounded-full hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t("search.openMenu")}
        >
          <Menu className="h-5 w-5" aria-hidden="true" />
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
            type="search"
            inputMode="search"
            autoComplete="off"
            spellCheck={false}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setFullSearch(false);
            }}
            onFocus={handleFocus}
            onBlur={() => setIsFocused(false)}
            onKeyDown={handleKeyDown}
            placeholder={t("search.placeholder")}
            aria-label={t("search.ariaLabel")}
            aria-autocomplete="list"
            aria-controls={LISTBOX_ID}
            className={cn(
              "flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground",
            )}
          />
          {query.length > 0 ? (
            <button
              type="button"
              onClick={handleClear}
              aria-label={t("search.clear")}
              className={cn(
                "inline-flex h-6 w-6 items-center justify-center rounded-full text-muted-foreground",
                "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              )}
            >
              <X className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          ) : null}
        </div>
      </div>

      {embedPanel ? (
        <div
          id={LISTBOX_ID}
          className={cn(
            "pointer-events-auto chrome-card min-h-[3rem] max-h-[70vh] overflow-hidden",
            query.length === 0 && !isFocused ? "hidden" : "",
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
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
