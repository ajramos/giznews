// Mock backend: implements the full API surface with realistic sample data so
// the UI runs in a plain browser (vite dev) for e2e/Playwright and quick
// manual checks. Mirrors the shape of the real Wails-bound backend.

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
import type { APIShape } from "./api";

const sampleSources: SourceDTO[] = [
  { id: 1, name: "HN RSS", type: "rss", url: "https://news.ycombinator.com/rss", group: "community", enabled: true, lastFetch: "2026-08-14T09:00:00Z" },
  { id: 2, name: "HN Algolia", type: "hackernews", url: "https://hn.algolia.com", group: "community", enabled: true, lastFetch: "2026-08-14T09:00:00Z" },
  { id: 3, name: "DeepMind Blog", type: "rss", url: "https://deepmind.google/blog/rss.xml", group: "labs", enabled: true },
  { id: 4, name: "arXiv cs.AI", type: "arxiv", url: "http://export.arxiv.org/rss/cs.AI", group: "research", enabled: false },
];

const lorem = (n: number) =>
  Array.from({ length: n }, (_, i) => `Paragraph ${i + 1}. This is a substantial body of text so the reader panel has enough height to scroll. It covers the key developments, the actors involved, and why the story matters for practitioners building on these systems.`)
    .join("\n\n");

const md = (body: string) => `# Title placeholder

${body}

${lorem(14)}

- item one
- item two

> A meaningful pull quote worth highlighting.

\`inline code\` and **bold** and [a link](https://example.com).`;

const sampleArticles: ArticleDTO[] = [
  { id: 1, sourceId: 1, sourceName: "HN RSS", url: "https://deepseek.com/harness", title: "DeepSeek Harness developer preview", importance: 3, status: "unread", category: "models", tags: ["deepseek", "llm"], summary: "DeepSeek released a developer preview of its new harness toolchain.", contentMD: md("DeepSeek **Harness** is a developer preview of a toolchain for evaluating and deploying language models at scale."), fetchedAt: "2026-08-14T09:00:00Z", published: "2026-08-14T09:00:00Z" },
  { id: 2, sourceId: 2, sourceName: "HN Algolia", url: "https://decrypt.co", title: "Anthropic Is Quietly Watermarking Every Claude AI Output", importance: 2, status: "unread", category: "regulation", tags: ["anthropic", "watermark"], summary: "Anthropic adds watermarks to all Claude outputs for traceability.", contentMD: md("Anthropic reportedly embeds **watermarks** in every Claude output, raising questions about builders who rely on those outputs."), fetchedAt: "2026-08-14T09:01:00Z", published: "2026-08-14T08:50:00Z" },
  { id: 3, sourceId: 1, sourceName: "HN RSS", url: "https://example.com/glm", title: "GLM-5.3: Frontier coding with emergent cyber capabilities", importance: 2, status: "unread", category: "models", tags: ["glm"], contentMD: md("GLM-5.3 pushes **frontier coding** capabilities and introduces new safety evaluations for autonomous cyber tasks."), fetchedAt: "2026-08-14T09:02:00Z" },
  { id: 4, sourceId: 2, sourceName: "HN Algolia", url: "https://example.com/rag", title: "Local RAG models beat cloud providers", importance: 1, status: "unread", category: "research", tags: ["rag"], summary: "New benchmarks show local retrieval pipelines winning.", contentMD: md("Local **RAG** pipelines now outperform cloud-hosted providers on latency-sensitive workloads."), fetchedAt: "2026-08-14T09:03:00Z" },
  { id: 5, sourceId: 2, sourceName: "HN Algolia", url: "https://example.com/agents", title: "Model Context Protocol (MCP): how agents connect to tools", importance: 2, status: "unread", category: "tools", tags: ["mcp", "agents"], contentMD: md("**MCP** standardizes how AI agents communicate with external tools and APIs."), fetchedAt: "2026-08-14T09:04:00Z" },
  { id: 6, sourceId: 3, sourceName: "DeepMind Blog", url: "https://deepmind.google/blog/agents", title: "Building more capable AI agents with world models", importance: 2, status: "unread", category: "research", tags: ["agents", "world-models"], contentMD: md("DeepMind explores **world models** as a substrate for longer-horizon agent planning."), fetchedAt: "2026-08-14T09:05:00Z" },
  { id: 7, sourceId: 2, sourceName: "HN Algolia", url: "https://example.com/eu", title: "The EU AI Act Is Now a Business-Blocking Risk for Vertical AI", importance: 3, status: "read", category: "regulation", tags: ["regulation", "eu"], summary: "Vertical AI firms face compliance hurdles under the EU AI Act.", contentMD: md("The **EU AI Act** now poses a business-blocking risk for vertical AI companies."), fetchedAt: "2026-08-14T09:06:00Z" },
  { id: 8, sourceId: 1, sourceName: "HN RSS", url: "https://example.com/banana", title: "Banana prices in Argentina jumped 40% this week", importance: 0, status: "unread", category: "general", tags: [], contentMD: md("An economic story far outside the AI beat."), fetchedAt: "2026-08-14T09:07:00Z" },
];

const sampleNotes: NoteDTO[] = [
  { id: 101, type: "electron", title: "ai agents", slug: "ai-agents", content: "# ai agents\n\n## Definición\nConcepto recurrente — referenciado en 3 nota(s).\n\n## Fuentes\n- [[model-context-protocol-mcp-explained]] — MCP: how agents connect to tools\n- [[building-more-capable-ai-agents]] — World models\n", tags: ["ai", "concept"], wikilinks: ["mcp"], createdAt: "2026-08-14T10:00:00Z" },
  { id: 102, type: "atom", title: "DeepSeek Harness developer preview", slug: "deepseek-harness-developer-preview", content: "# DeepSeek Harness developer preview\n\n## Resumen\nDeepSeek released a dev preview.\n\n## Conexiones\n- [[deepseek]]\n", tags: ["atom", "ai", "deepseek"], wikilinks: ["deepseek"], createdAt: "2026-08-14T10:01:00Z" },
  { id: 103, type: "molecule", title: "Síntesis de models", slug: "sintesis-models", content: "# 🧪 Síntesis de models\n\n## Central Idea\nThe current landscape is defined by performance and safety.\n\n## Conexiones\n- [[deepseek-harness-developer-preview]]\n", tags: ["synthesis", "ai"], wikilinks: ["deepseek-harness-developer-preview"], createdAt: "2026-08-14T10:02:00Z" },
];

const sampleDigest: DigestDTO = {
  date: "2026-08-14",
  overview: "DeepSeek unveiled a developer preview and Anthropic began watermarking every Claude output, signaling a push toward evaluation and traceability in model releases.",
  themes: [
    {
      theme: "models",
      summary: "New previews and watermarking shift focus toward evaluation tooling.",
      articles: [sampleArticles[0], sampleArticles[2]],
    },
    {
      theme: "regulation",
      summary: "Traceability and compliance pressure mounts for AI builders.",
      articles: [sampleArticles[1], sampleArticles[6]],
    },
  ],
};

const sampleSearch: SearchResultDTO[] = [
  { kind: "article", id: 2, title: "Anthropic Is Quietly Watermarking Every Claude AI Output", source: "HN Algolia", snippet: "Anthropic adds watermarks to all Claude outputs for traceability.", score: 0.88 },
  { kind: "note", id: 102, title: "DeepSeek Harness developer preview", source: "atom", snippet: "DeepSeek released a dev preview.", score: 0.81 },
  { kind: "article", id: 5, title: "MCP: how agents connect to tools", source: "HN Algolia", snippet: "MCP standardizes tool access for agents.", score: 0.74 },
];

const delay = (ms = 30) => new Promise((r) => setTimeout(r, ms));

export const mockBackend: APIShape = {
  listSources: async (): Promise<SourceDTO[]> => { await delay(); return sampleSources; },
  addSource: async (name: string, type: string, url: string, group: string): Promise<SourceDTO> => {
    await delay();
    const s: SourceDTO = { id: Date.now(), name, type, url, group: group || "general", enabled: true };
    sampleSources.push(s);
    return s;
  },
  setSourceEnabled: async (id: number, enabled: boolean): Promise<void> => {
    await delay();
    const s = sampleSources.find((x) => x.id === id);
    if (s) s.enabled = enabled;
  },
  deleteSource: async (id: number): Promise<void> => {
    await delay();
    const i = sampleSources.findIndex((s) => s.id === id);
    if (i >= 0) sampleSources.splice(i, 1);
  },

  listArticles: async (opts: ListArticlesOptions): Promise<ArticleDTO[]> => {
    await delay();
    let list = sampleArticles;
    if (opts.status) list = list.filter((a) => a.status === opts.status);
    if (opts.importanceMin) list = list.filter((a) => a.importance >= (opts.importanceMin ?? 0));
    if (opts.sourceId) list = list.filter((a) => a.sourceId === opts.sourceId);
    return list;
  },
  getArticle: async (id: number): Promise<ArticleDTO> => {
    await delay();
    const a = sampleArticles.find((x) => x.id === id);
    if (!a) throw new Error("article not found");
    return { ...a };
  },
  getArticleContent: async (id: number): Promise<ArticleDTO> => {
    await delay(120); // simulate extraction
    const a = sampleArticles.find((x) => x.id === id);
    if (!a) throw new Error("article not found");
    return { ...a };
  },
  setArticleStatus: async (id: number, status: string): Promise<void> => {
    await delay();
    const a = sampleArticles.find((x) => x.id === id);
    if (a) a.status = status as ArticleDTO["status"];
  },
  setArticleImportance: async (_id: number, _importance: number): Promise<void> => { await delay(); },

  fetch: async (): Promise<FetchResult> => { await delay(60); return { newArticles: 3, updated: 0, sourcesFetched: 4, sourcesFailed: 0, extracted: 2, elapsedMs: 500 }; },
  classify: async (_limit: number): Promise<ClassifyResult> => { await delay(60); return { classified: 8, byRules: 2, byLLM: 6, skippedNoLLM: 0, batches: 1, errors: [] }; },
  summarizeArticle: async (id: number): Promise<ArticleDTO> => {
    await delay(80);
    const a = await mockBackend.getArticle(id);
    a.summary = "Mock summary: this article covers a key development in the AI landscape and why it matters for practitioners.";
    return a;
  },
  digest: async (): Promise<DigestDTO> => { await delay(80); return sampleDigest; },

  kbuild: async (): Promise<KBResult> => { await delay(60); return { atomsCreated: 5, electronsCreated: 2, electronsUpdated: 0, moleculesCreated: 0, articlesSkipped: 12 }; },
  ksynthesize: async (_category: string): Promise<KBResult> => { await delay(60); return { atomsCreated: 0, electronsCreated: 0, electronsUpdated: 0, moleculesCreated: 1, articlesSkipped: 0 }; },
  listNotes: async (type: string): Promise<NoteDTO[]> => { await delay(); return sampleNotes.filter((n) => (type ? n.type === type : true)); },
  getNote: async (id: number): Promise<NoteDTO> => {
    await delay();
    const n = sampleNotes.find((x) => x.id === id);
    if (!n) throw new Error("note not found");
    return { ...n };
  },
  graphNeighbors: async (_id: number): Promise<NoteDTO[]> => {
    await delay();
    return sampleNotes.slice(0, 2);
  },

  searchIndex: async (): Promise<IndexResult> => { await delay(); return { notesEmbedded: 3, articlesEmbedded: 8, ftsNotes: 3, ftsArticles: 8, embeddingsFailed: 0 }; },
  search: async (_q: string, _limit: number): Promise<SearchResultDTO[]> => { await delay(60); return sampleSearch; },

  status: async (): Promise<StatusDTO> => {
    await delay();
    return { dbPath: "/mock/db", vaultPath: "/mock/vault", llmProvider: "ollama", llmEnabled: true, llmReachable: true, embeddingsModel: "nomic-embed-text", unreadArticles: sampleArticles.filter((a) => a.status === "unread").length, totalArticles: sampleArticles.length, totalNotes: sampleNotes.length };
  },

  openURL: async (_url: string): Promise<void> => { await delay(); },
  openVault: async (): Promise<void> => { await delay(); },
  quit: async (): Promise<void> => { await delay(); },
};
