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
  published?: string;
  fetchedAt: string;
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
  byLLM: number;
  skippedNoLLM: number;
  batches: number;
  errors: string[];
}

export interface KBResult {
  atomsCreated: number;
  electronsCreated: number;
  electronsUpdated: number;
  moleculesCreated: number;
  articlesSkipped: number;
}

export interface IndexResult {
  notesEmbedded: number;
  articlesEmbedded: number;
  ftsNotes: number;
  ftsArticles: number;
  embeddingsFailed: number;
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
}

export interface ListArticlesOptions {
  status?: string;
  category?: string;
  sourceId?: number;
  group?: string;
  importanceMin?: number;
  query?: string;
  limit?: number;
  offset?: number;
}

export type ViewMode = "articles" | "digest";
export type PanelMode = "none" | "search" | "graph" | "palette";
