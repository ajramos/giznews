// Typed bridge to the Wails-bound Go API (window.go.main.App.*).
// Wails injects context.Context automatically, so calls pass ONLY the declared
// method arguments (no leading null).
//
// In a plain browser (vite dev / Playwright) window.go is absent, so the api
// falls back to the mock backend for fully offline UI development.

import { mockBackend } from "./apiMock";
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

// isWails reports whether the real backend bindings are present.
export const isWails = (): boolean =>
  typeof window !== "undefined" && !!window.go?.main?.App;

// runtimeShim exposes a minimal window.runtime for browser mode so palette
// actions (OpenVault/Quit) degrade gracefully.
export function installRuntimeShim(): void {
  const win = window as unknown as { runtime?: unknown };
  if (typeof window === "undefined" || win.runtime) return;
  win.runtime = {
    WindowStartDragging: () => {},
    EventsOn: () => () => {},
    EventsOff: () => {},
  };
}

// camel converts a Wails wire key (snake_case json tag) to the camelCase
// field used by the TS interfaces: content_md → contentMD, content_html →
// contentHTML, source_id → sourceId, llm_enabled → llmEnabled.
function camel(key: string): string {
  return key.replace(/_([a-z]+)/g, (_, w: string) => {
    if (w === "md") return "MD";
    if (w === "html") return "HTML";
    return w[0].toUpperCase() + w.slice(1);
  });
}

// snake converts camelCase back to the snake_case json tag Wails expects in
// struct arguments: importanceMin → importance_min, sourceId → source_id.
function snake(key: string): string {
  return key.replace(/[A-Z]/g, (c) => "_" + c.toLowerCase());
}

function toSnakeArgs(obj: object): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(obj)) out[snake(k)] = v;
  return out;
}

// normalize maps a Wails JSON object onto the camelCase interface, ignoring
// unknown keys.
function normalize<T>(obj: unknown): T {
  if (obj === null || obj === undefined) return {} as T;
  const src = obj as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(src)) {
    out[camel(k)] = v;
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

// Build the API surface: real Wails bindings or the mock backend.
export interface APIShape {
  listSources: () => Promise<SourceDTO[]>;
  addSource: (name: string, type: string, url: string, group: string) => Promise<SourceDTO>;
  setSourceEnabled: (id: number, enabled: boolean) => Promise<void>;
  deleteSource: (id: number) => Promise<void>;
  listArticles: (opts: ListArticlesOptions) => Promise<ArticleDTO[]>;
  getArticle: (id: number) => Promise<ArticleDTO>;
  getArticleContent: (id: number) => Promise<ArticleDTO>;
  setArticleStatus: (id: number, status: string) => Promise<void>;
  setArticleImportance: (id: number, importance: number) => Promise<void>;
  fetch: () => Promise<FetchResult>;
  classify: (limit: number) => Promise<ClassifyResult>;
  summarizeArticle: (id: number) => Promise<ArticleDTO>;
  digest: () => Promise<DigestDTO>;
  kbuild: () => Promise<KBResult>;
  ksynthesize: (category: string) => Promise<KBResult>;
  listNotes: (type: string) => Promise<NoteDTO[]>;
  getNote: (id: number) => Promise<NoteDTO>;
  graphNeighbors: (id: number) => Promise<NoteDTO[]>;
  searchIndex: () => Promise<IndexResult>;
  search: (query: string, limit: number) => Promise<SearchResultDTO[]>;
  status: () => Promise<StatusDTO>;
  openURL: (url: string) => Promise<void>;
  openVault: () => Promise<void>;
  quit: () => Promise<void>;
}

const realApi: APIShape = {
  listSources: () => call("ListSources").then((v) => arr<SourceDTO>(v)),
  addSource: (name: string, type: string, url: string, group: string) =>
    call("AddSource", name, type, url, group).then((v) => normalize<SourceDTO>(v)),
  setSourceEnabled: (id: number, enabled: boolean) =>
    call("SetSourceEnabled", id, enabled).then(() => undefined),
  deleteSource: (id: number) =>
    call("DeleteSource", id).then(() => undefined),

  listArticles: (opts: ListArticlesOptions) =>
    call("ListArticles", toSnakeArgs(opts)).then((v) => arr<ArticleDTO>(v)),
  getArticle: (id: number) =>
    call("GetArticle", id).then((v) => normalize<ArticleDTO>(v)),
  getArticleContent: (id: number) =>
    call("GetArticleContent", id).then((v) => normalize<ArticleDTO>(v)),
  setArticleStatus: (id: number, status: string) =>
    call("SetArticleStatus", id, status).then(() => undefined),
  setArticleImportance: (id: number, importance: number) =>
    call("SetArticleImportance", id, importance).then(() => undefined),

  fetch: () => call<FetchResult>("Fetch"),
  classify: (limit: number) => call<ClassifyResult>("Classify", limit),
  summarizeArticle: (id: number) =>
    call("SummarizeArticle", id).then((v) => normalize<ArticleDTO>(v)),
  digest: () => call<DigestDTO>("Digest"),

  kbuild: () => call<KBResult>("KBuild"),
  ksynthesize: (category: string) => call<KBResult>("KSynthesize", category),
  listNotes: (type: string) => call("ListNotes", type).then((v) => arr<NoteDTO>(v)),
  getNote: (id: number) => call("GetNote", id).then((v) => normalize<NoteDTO>(v)),
  graphNeighbors: (id: number) => call("GraphNeighbors", id).then((v) => arr<NoteDTO>(v)),

  searchIndex: () => call<IndexResult>("SearchIndex"),
  search: (query: string, limit: number) =>
    call("Search", query, limit).then((v) => arr<SearchResultDTO>(v)),

  status: () => call("Status").then((v) => normalize<StatusDTO>(v)),

  openURL: (url: string) =>
    call("OpenURL", url).then(() => undefined),
  openVault: () => call("OpenVault").then(() => undefined),
  quit: () => call("Quit").then(() => undefined),
};

export const api: APIShape = isWails() ? realApi : mockBackend;