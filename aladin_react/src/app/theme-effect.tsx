import { useEffect } from "react";
import { useAppStore } from "@/app/state/store";

/**
 * Applies the active theme to <html data-theme="…">. Dark, cool, and soft are dark-family;
 * light steps out of it. The CSS in index.css resolves all tokens from the data-theme
 * attribute. Renders nothing.
 */
export function ThemeEffect() {
  const theme = useAppStore((state) => state.theme);
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);
  return null;
}
