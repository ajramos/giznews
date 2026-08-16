// Keyboard grammar — vim-consistent, single-letter panels.
// Motions: j/k (with count prefix 5j), gg/G, Ctrl+d/u
// Verbs:  y summarize · a archive · t read · m star · O open · Enter
// Panels (toggle): s search · g graph · d digest · : palette · ? help
// Views: u unread · r read · x archived · * starred

export interface HelpCategory {
  title: string;
  rows: { keys: string; label: string }[];
}

export const HELP: HelpCategory[] = [
  {
    title: "Navigation",
    rows: [
      { keys: "j / k", label: "Next / previous (5j = jump 5) — auto-loads" },
      { keys: "gg / G", label: "Top / bottom of the list" },
      { keys: "Ctrl+d / Ctrl+u", label: "Half page down / up" },
      { keys: "J / K", label: "Open next / previous article" },
      { keys: "Enter", label: "Open article (and focus the reader)" },
      { keys: "Esc", label: "Back to the list (unfocus the reader)" },
    ],
  },
  {
    title: "Article actions",
    rows: [
      { keys: "Enter", label: "Open / close article (focuses the reader)" },
      { keys: "J / K", label: "Next / previous article (opens)" },
      { keys: "j / k", label: "Scroll the reader while focused" },
      { keys: "Space / Shift+Space", label: "Page down / up in the reader" },
      { keys: "y", label: "AI summary" },
      { keys: "a", label: "Archive (5a = archive 5) — undo via toast" },
      { keys: "t", label: "Mark read / unread" },
      { keys: "m", label: "Star" },
      { keys: "O", label: "Open in browser" },
    ],
  },
  {
    title: "Panels (toggle)",
    rows: [
      { keys: "s", label: "Semantic search" },
      { keys: "g", label: "Knowledge graph" },
      { keys: "f", label: "Knowledge vault (world): electrons → atoms → molecules" },
      { keys: "n", label: "Open the vault at the Atoms stage (quick note browse)" },
      { keys: "z", label: "Background jobs" },
      { keys: "d", label: "Daily digest" },
      { keys: ":", label: "Command palette" },
      { keys: "Esc", label: "Close panel / back" },
    ],
  },
  {
    title: "Worlds (news vs knowledge)",
    rows: [
      { keys: "u / r / x / *", label: "News world (article list) — also :news" },
      { keys: "f", label: "Toggle vault ↔ news" },
      { keys: "n", label: "Vault, Atoms stage" },
      { keys: "h/l · j/k · Enter", label: "In the vault: switch stage · move item · open (detail on the right)" },
      { keys: "f / u", label: "Back to the news world (mode shows in the status bar)" },
    ],
  },
  {
    title: "Filters (classification)",
    rows: [
      { keys: ";", label: "Filter by category (All / Unclassified / models / research / …)" },
      { keys: "[ / ]", label: "Importance filter (any → ≥1★ → ≥2★ → ≥3★)" },
      { keys: "chips", label: "Click the chips under the list to filter (category + importance)" },
    ],
  },
  {
    title: "List views",
    rows: [
      { keys: "u / r / x / *", label: "Unread · Read · Archived · Starred" },
    ],
  },
  {
    title: "Bulk mode (multi-select)",
    rows: [
      { keys: "v", label: "Enter bulk mode" },
      { keys: "space", label: "Toggle the current article" },
      { keys: "j / k", label: "Navigate (without changing selection)" },
      { keys: "a / t / m", label: "Apply archive · read · star to the selection" },
      { keys: "c", label: "Classify the selection (background job, priority over the queue)" },
      { keys: "Esc / v", label: "Exit bulk mode" },
    ],
  },
  {
    title: "App",
    rows: [
      { keys: ":process", label: "Full pipeline (fetch → classify → kb → index)" },
      { keys: ":url", label: "Add an article by URL" },
      { keys: ":jobs", label: "Browse background jobs (cancel / remove)" },
      { keys: ":flow", label: "Pipeline flow (live counts per stage)" },
      { keys: "q", label: "Quit GizNews" },
    ],
  },
];

export const CONTEXT_KEYS: Record<string, { key: string; label: string }[]> = {
  list: [
    { key: "j/k", label: "navigate" },
    { key: "Enter", label: "open" },
    { key: "y", label: "summarize" },
    { key: "a", label: "archive" },
    { key: "t", label: "read" },
    { key: "m", label: "star" },
    { key: ";", label: "filter" },
    { key: "[/]", label: "importance" },
    { key: "s", label: "search" },
    { key: "g", label: "graph" },
    { key: ":", label: "cmd" },
    { key: "?", label: "help" },
  ],
  reader: [
    { key: "j/k", label: "scroll" },
    { key: "J/K", label: "next/prev" },
    { key: "space", label: "page" },
    { key: "Esc", label: "back to list" },
    { key: "y", label: "summarize" },
    { key: "a", label: "archive" },
    { key: "m", label: "star" },
    { key: "O", label: "open web" },
  ],
  search: [
    { key: "↑/↓", label: "results" },
    { key: "Enter", label: "open" },
    { key: "Esc", label: "close" },
  ],
  graph: [
    { key: "click", label: "expand node" },
    { key: "dbl", label: "open note" },
    { key: "Esc", label: "close" },
  ],
  digest: [
    { key: "1..9", label: "count" },
    { key: "j/k", label: "navigate" },
    { key: "Esc", label: "back" },
  ],
  vault: [
    { key: "h/l", label: "stage" },
    { key: "j/k", label: "items" },
    { key: "Enter", label: "open" },
    { key: "L", label: "links" },
    { key: "f/u", label: "news" },
  ],
};
