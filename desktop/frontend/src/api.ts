// Typed bridge to the Wails-bound Go API (window.go.main.App.*).
// Wails injects context.Context automatically, so calls pass ONLY the declared
// method arguments (no leading null).

import type {
  ArticleDTO,
  ClassifyResult,
  DigestDTO,
  FetchResult,
  IndexResult,
  KBResult,
  ListArticlesOptions,
  NoteDTO,
  SearchResultDTO,
  SourceDTO,
  StatusDTO,
} from "./types";

declare global {
  interface Window {
    go: {
      main: {
        App: {
          [key: string]: (...args: unknown[]) => Promise<unknown>;
        };
      };
    };
  }
}

// normalize maps a Wails JSON object (PascalCase Go field names) onto a
// camelCase interface, ignoring unknown keys.
function normalize<T>(obj: unknown): T {
  if (obj === null || obj === undefined) return {} as T;
  const src = obj as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(src)) {
    const key = k.charAt(0).toLowerCase() + k.slice(1);
    out[key] = v;
  }
  return out as T;
}

function arr<T>(v: unknown): T[] {
  return Array.isArray(v) ? v.map((x) => normalize<T>(x)) : [];
}

async function call<T>(name: string, ...args: unknown[]): Promise<T> {
  const fn = window.go?.main?.App?.[name];
  if (!fn) throw new Error(`Wails binding ${name} not available`);
  return fn(...args) as Promise<T>;
}

export const api = {
  // Sources
  listSources: () => call("ListSources").then((v) => arr<SourceDTO>(v)),
  addSource: (name: string, type: string, url: string, group: string) =>
    call("AddSource", name, type, url, group).then((v) => normalize<SourceDTO>(v)),
  setSourceEnabled: (id: number, enabled: boolean) =>
    call("SetSourceEnabled", id, enabled).then(() => undefined),

  // Articles
  listArticles: (opts: ListArticlesOptions) =>
    call("ListArticles", opts).then((v) => arr<ArticleDTO>(v)),
  getArticle: (id: number) =>
    call("GetArticle", id).then((v) => normalize<ArticleDTO>(v)),
  setArticleStatus: (id: number, status: string) =>
    call("SetArticleStatus", id, status).then(() => undefined),
  setArticleImportance: (id: number, importance: number) =>
    call("SetArticleImportance", id, importance).then(() => undefined),

  // Pipeline
  fetch: () => call<FetchResult>("Fetch"),
  classify: (limit: number) => call<ClassifyResult>("Classify", limit),
  summarizeArticle: (id: number) =>
    call("SummarizeArticle", id).then((v) => normalize<ArticleDTO>(v)),
  digest: () => call<DigestDTO>("Digest"),

  // Knowledge graph
  kbuild: () => call<KBResult>("KBuild"),
  ksynthesize: (category: string) => call<KBResult>("KSynthesize", category),
  listNotes: (type: string) => call("ListNotes", type).then((v) => arr<NoteDTO>(v)),
  getNote: (id: number) => call("GetNote", id).then((v) => normalize<NoteDTO>(v)),
  graphNeighbors: (id: number) => call("GraphNeighbors", id).then((v) => arr<NoteDTO>(v)),

  // Search
  searchIndex: () => call<IndexResult>("SearchIndex"),
  search: (query: string, limit: number) =>
    call("Search", query, limit).then((v) => arr<SearchResultDTO>(v)),

  // Meta
  status: () => call("Status").then((v) => normalize<StatusDTO>(v)),
};
