import type { Config } from "tailwindcss";
import animate from "tailwindcss-animate";

/**
 * Tailwind config for LocalMaps.
 *
 * The palette mimics the overall temperature of the Google Maps desktop
 * UI (warm off-white street tones for light, dark slate for dark) while
 * remaining our own palette — values are declared as CSS variables in
 * `app/globals.css` and referenced here via hsl(var(--token)) so that
 * themes can be swapped at runtime without re-building.
 */
const config: Config = {
  darkMode: ["class"],
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./lib/**/*.{ts,tsx}",
  ],
  theme: {
    container: {
      center: true,
      padding: "1rem",
    },
    extend: {
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        /** Map chrome surface tokens (panel-over-map). */
        chrome: {
          surface: "hsl(var(--chrome-surface))",
          border: "hsl(var(--chrome-border))",
          shadow: "hsl(var(--chrome-shadow))",
        },
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
      boxShadow: {
        chrome:
          "0 1px 2px hsl(var(--chrome-shadow) / 0.08), 0 4px 12px hsl(var(--chrome-shadow) / 0.12)",
        /** Subtle elevation for floating chrome (FabStack, LayersCard,
         *  SearchBar pill). Lighter than the panel-grade `chrome` shadow. */
        "chrome-sm":
          "0 1px 2px hsl(var(--chrome-shadow) / 0.06), 0 1px 3px hsl(var(--chrome-shadow) / 0.10)",
        /** Heavier elevation for sheet / dropdown / point-info card. */
        "chrome-lg":
          "0 4px 8px hsl(var(--chrome-shadow) / 0.10), 0 12px 24px hsl(var(--chrome-shadow) / 0.16)",
      },
      ringColor: {
        chrome: "hsl(var(--chrome-border))",
      },
      fontFamily: {
        sans: ["var(--font-sans)", "system-ui", "sans-serif"],
      },
      keyframes: {
        "accordion-down": {
          from: { height: "0" },
          to: { height: "var(--radix-accordion-content-height)" },
        },
        "accordion-up": {
          from: { height: "var(--radix-accordion-content-height)" },
          to: { height: "0" },
        },
      },
      animation: {
        "accordion-down": "accordion-down 0.2s ease-out",
        "accordion-up": "accordion-up 0.2s ease-out",
      },
    },
  },
  plugins: [animate],
};

export default config;
