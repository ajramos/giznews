// Theme system: CSS-variable token palettes following giztui's
// foundation/semantic structure. Switching sets [data-theme] on <html>.

export type ThemeName = "giznews" | "dracula" | "slate";

export interface Theme {
  name: ThemeName;
  label: string;
}

export const THEMES: Theme[] = [
  { name: "giznews", label: "GizNews Dark" },
  { name: "dracula", label: "Dracula" },
  { name: "slate", label: "Slate Blue" },
];

const KEY = "giznews-theme";

export function currentTheme(): ThemeName {
  const saved = localStorage.getItem(KEY) as ThemeName | null;
  if (saved && THEMES.some((t) => t.name === saved)) return saved;
  return "giznews";
}

export function applyTheme(name: ThemeName): void {
  document.documentElement.setAttribute("data-theme", name);
  localStorage.setItem(KEY, name);
}

export function initTheme(): void {
  applyTheme(currentTheme());
}
