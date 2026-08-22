// Types mirroring pkg/desktop DTOs. Wails exposes Go struct fields verbatim
// (PascalCase); the api bridge normalizes to these camelCase interfaces.

export interface SourceDTO {
  id: number;
  name: string;
  type: string;
  url: string;
  group: string;
  enabled: boolean;
  lastFetch?: string;
  lastError?: string;
  lastOk?: string;
  consecutiveFailures?: number;
  emptyCycles?: number;
}

export interface ArticleDTO {
  id: number;
  sourceId: number;
  sourceName?: string;
  url: string;
  title: string;
  author?: string;
  contentMD?: string;
  summary?: string;
  category?: string;
  tags: string[];
  importance: number;
  status: string;
  starred?: boolean;
  published?: string;
  fetchedAt: string;
  // How many outlets ran this story, and which. Absent when nobody else did.
  storySize?: number;
  storySources?: string[];
}

export interface NoteDTO {
  id: number;
  type: string;
  title: string;
  slug: string;
  content: string;
  tags: string[];
  wikilinks: string[];
  createdAt: string;
  category?: string;
  rating?: number;
  url?: string;
  source?: string;
}

export interface DigestThemeDTO {
  theme: string;
  summary: string;
  articles: ArticleDTO[];
}

export interface DigestDTO {
  date: string;
  overview: string;
  themes: DigestThemeDTO[];
}

export interface SearchResultDTO {
  kind: string;
  id: number;
  title: string;
  source: string;
  snippet: string;
  score: number;
}

export interface FetchResult {
  newArticles: number;
  updated: number;
  sourcesFetched: number;
  sourcesFailed: number;
  extracted: number;
  elapsedMs: number;
}

export interface ClassifyResult {
  classified: number;
  byRules: number;
  archived?: number;
  byLLM: number;
  skippedNoLLM: number;
  batches: number;
  pending?: number;
  errors: string[];
}

export interface KBResult {
  atomsCreated: number;
  electronsCreated: number;
  electronsUpdated: number;
  moleculesCreated: number;
  moleculesUpdated: number;
  articlesSkipped: number;
  conceptsTracked: number;
  atomsRefreshed: number;
  editedNotesKept: number;
  notesImported: number;
}

export interface IndexResult {
  notesEmbedded: number;
  articlesEmbedded: number;
  ftsNotes: number;
  ftsArticles: number;
  embeddingsFailed: number;
}

export interface JobDTO {
  id: number;
  name: string;
  type: string;
  status: "running" | "done" | "error" | "canceled";
  phase: string;
  done: number;
  total: number;
  message?: string;
  errMsg?: string;
  createdAt: string;
  finishedAt?: string;
}

export interface BulkResult {
  updated: number;
  total: number;
}

export interface StatusDTO {
  dbPath: string;
  vaultPath: string;
  llmProvider: string;
  llmEnabled: boolean;
  llmReachable: boolean;
  embeddingsModel: string;
  unreadArticles: number;
  totalArticles: number;
  totalNotes: number;
  pendingClassify: number;
  unhealthySources: number;
}

export interface ListArticlesOptions {
  status?: string;
  unarchived?: boolean;
  starred?: boolean;
  category?: string;
  sourceId?: number;
  group?: string;
  importanceMin?: number;
  importanceExact?: number;
  unclassified?: boolean;
  summarized?: boolean;
  query?: string;
  limit?: number;
  offset?: number;
}

export interface DigestMeta {
  date: string;
  overview: string;
}

export interface FlowStatus {
  sourcesTotal: number;
  sourcesEnabled: number;
  articlesTotal: number;
  classified: number;
  pendingClassify: number;
  atoms: number;
  electrons: number;
  molecules: number;
  vaultPath: string;
  notesEmbedded: number;
  articlesEmbedded: number;
  runningJobs: number;
}

export interface RuleActionDTO {
  type: string; // category | tag | importance | archive
  value: string;
}

export interface RuleDTO {
  id: number;
  name: string;
  query: string;
  actions: RuleActionDTO[];
  enabled: boolean;
}

// ConceptDTO is a recurring idea the knowledge graph tracks. Promoted ones have
// an Electron note; the rest are still dangling links waiting for mentions.
export interface ConceptDTO {
  slug: string;
  name: string;
  mentions: number;
  noteId?: number;
  promoted: boolean;
  firstSeen?: string;
  lastSeen?: string;
}

export interface MergeDTO {
  notesRelinked: number;
  mentions: number;
  redirected: boolean;
}

export type ViewMode = "articles" | "digest";
export type PanelMode = "none" | "search" | "graph" | "palette";
