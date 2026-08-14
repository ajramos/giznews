// Keyboard shortcuts (mirror giztui muscle memory).
export const KEYS = {
  down: ["j", "ArrowDown"],
  up: ["k", "ArrowUp"],
  open: ["Enter"],
  top: ["g", "Home"],
  bottom: ["G", "End"],
  search: ["s"],
  summarize: ["y"],
  graph: ["g"], // hmm, conflicts with top; handled in App with timing
  archive: ["a"],
  toggleRead: ["t"],
  digest: ["d"],
  articles: ["1"],
  palette: [":"],
  help: ["?"],
  esc: ["Escape"],
};

export interface ShortcutRow {
  key: string;
  label: string;
}

export const HELP_ROWS: ShortcutRow[] = [
  { key: "j / k", label: "Navegar por artículos" },
  { key: "Enter", label: "Abrir artículo" },
  { key: "g / G", label: "Ir al inicio / final" },
  { key: "s", label: "Búsqueda semántica" },
  { key: "y", label: "Resumen IA del artículo" },
  { key: "g g", label: "Grafo del artículo (knowledge graph)" },
  { key: "a", label: "Archivar artículo" },
  { key: "t", label: "Marcar leído / no leído" },
  { key: "d", label: "Digest diario" },
  { key: "1 / 2", label: "Vista artículos / digest" },
  { key: ":", label: "Command palette" },
  { key: "Esc", label: "Cerrar panel" },
];
