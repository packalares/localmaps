"use client";

import { Check, Languages } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { useMessages } from "@/lib/i18n/provider";
import {
  LOCALE_NATIVE_NAMES,
  SUPPORTED_LOCALES,
  type Locale,
} from "@/lib/i18n/types";

/**
 * Tiny globe-icon button next to the RegionSwitcher. Opens a dropdown
 * with the supported locales labelled in their native language — that
 * way a user scanning the menu in English can still find "Română".
 *
 * The actual persistence + event dispatch is handled in
 * `LocaleProvider.setLocale`, so this component is a thin UI wrapper.
 */
export interface LocaleSelectorProps {
  className?: string;
}

export function LocaleSelector({ className }: LocaleSelectorProps) {
  const { locale, setLocale, t } = useMessages();

  const triggerClass = cn(
    "chrome-card pointer-events-auto inline-flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-medium",
    "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
    className,
  );

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t("locale.ariaLabel")}
          className={triggerClass}
        >
          <Languages className="h-4 w-4" aria-hidden="true" />
          <span className="sr-only sm:not-sr-only">
            {LOCALE_NATIVE_NAMES[locale]}
          </span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-[10rem]">
        {SUPPORTED_LOCALES.map((code) => (
          <LocaleItem
            key={code}
            code={code}
            active={code === locale}
            onSelect={() => setLocale(code)}
          />
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function LocaleItem({
  code,
  active,
  onSelect,
}: {
  code: Locale;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <DropdownMenuItem
      onSelect={onSelect}
      aria-label={`Switch to ${LOCALE_NATIVE_NAMES[code]}`}
    >
      <span className="flex w-5 items-center justify-center">
        {active ? <Check className="h-4 w-4" aria-hidden="true" /> : null}
      </span>
      <span>{LOCALE_NATIVE_NAMES[code]}</span>
    </DropdownMenuItem>
  );
}
