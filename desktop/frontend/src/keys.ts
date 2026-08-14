// Keyboard grammar — vim-consistent, single-letter panels.
// Motions: j/k (con prefijo de conteo 5j), gg/G, Ctrl+d/u
// Verbs:  y resumen · a archivar · t leído · m destacar · O abrir · Enter
// Paneles (toggle): s búsqueda · g grafo · d digest · : palette · ? ayuda
// Vistas: u no leídos · r leídos · x archivados · * destacados

export interface HelpCategory {
  title: string;
  rows: { keys: string; label: string }[];
}

export const HELP: HelpCategory[] = [
  {
    title: "Navegación",
    rows: [
      { keys: "j / k", label: "Siguiente / anterior (5j = saltar 5)" },
      { keys: "gg / G", label: "Inicio / final de la lista" },
      { keys: "Ctrl+d / Ctrl+u", label: "Media página abajo / arriba" },
      { keys: "Enter", label: "Abrir artículo seleccionado" },
    ],
  },
  {
    title: "Acciones sobre el artículo",
    rows: [
      { keys: "Enter", label: "Abrir / cerrar artículo" },
      { keys: "J / K", label: "Siguiente / anterior artículo (abre)" },
      { keys: "Espacio / Shift+Espacio", label: "Página abajo / arriba en el lector" },
      { keys: "y", label: "Resumen IA" },
      { keys: "a", label: "Archivar (5a = archivar 5) — deshacer con toast" },
      { keys: "t", label: "Marcar leído / no leído" },
      { keys: "m", label: "Destacar (star)" },
      { keys: "O", label: "Abrir en el navegador" },
    ],
  },
  {
    title: "Paneles (toggle)",
    rows: [
      { keys: "s", label: "Búsqueda semántica" },
      { keys: "g", label: "Grafo de conocimiento" },
      { keys: "d", label: "Digest diario" },
      { keys: ":", label: "Command palette" },
      { keys: "Esc", label: "Cerrar panel / volver" },
    ],
  },
  {
    title: "Vistas de la lista",
    rows: [
      { keys: "u / r / x / *", label: "No leídos · Leídos · Archivados · Destacados" },
    ],
  },
  {
    title: "App",
    rows: [
      { keys: "q", label: "Salir de GizNews" },
    ],
  },
];

export const CONTEXT_KEYS: Record<string, { key: string; label: string }[]> = {
  list: [
    { key: "j/k", label: "navegar" },
    { key: "Enter", label: "abrir" },
    { key: "y", label: "resumen" },
    { key: "a", label: "archivar" },
    { key: "t", label: "leído" },
    { key: "m", label: "destacar" },
    { key: "s", label: "buscar" },
    { key: "g", label: "grafo" },
    { key: ":", label: "cmd" },
    { key: "?", label: "ayuda" },
  ],
  reader: [
    { key: "J/K", label: "siguiente/prev" },
    { key: "space", label: "página" },
    { key: "y", label: "resumen" },
    { key: "a", label: "archivar" },
    { key: "m", label: "destacar" },
    { key: "O", label: "abrir web" },
  ],
  search: [
    { key: "↑/↓", label: "resultados" },
    { key: "Enter", label: "abrir" },
    { key: "Esc", label: "cerrar" },
  ],
  graph: [
    { key: "clic", label: "expandir nodo" },
    { key: "dbl", label: "abrir nota" },
    { key: "Esc", label: "cerrar" },
  ],
  digest: [
    { key: "1..9", label: "conteo" },
    { key: "j/k", label: "navegar" },
    { key: "Esc", label: "volver" },
  ],
};
