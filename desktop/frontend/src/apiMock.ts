// Mock backend: implements the full API surface with realistic sample data so
// the UI runs in a plain browser (vite dev) for e2e/Playwright and quick
// manual checks. Mirrors the shape of the real Wails-bound backend.

import type {
  ArticleDTO,
  BulkResult,
  ClassifyResult,
  ConceptDTO,
  DigestDTO,
  DigestMeta,
  FetchResult,
  FlowStatus,
  IndexResult,
  JobDTO,
  KBResult,
  ListArticlesOptions,
  MergeDTO,
  NoteDTO,
  RuleActionDTO,
  RuleDTO,
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
  { id: 1, sourceId: 1, sourceName: "HN RSS", url: "https://deepseek.com/harness", title: "DeepSeek Harness developer preview", importance: 3, status: "unread", category: "models", tags: ["deepseek", "llm"], summary: "DeepSeek released a developer preview of its new harness toolchain.", contentMD: md("DeepSeek **Harness** is a developer preview of a toolchain for evaluating and deploying language models at scale."), fetchedAt: "2026-08-14T09:00:00Z", published: "2026-08-14T09:00:00Z", storySize: 4, storySources: ["HN RSS", "The Verge", "Ars Technica", "TechCrunch"] },
  { id: 2, sourceId: 2, sourceName: "HN Algolia", url: "https://decrypt.co", title: "Anthropic Is Quietly Watermarking Every Claude AI Output", importance: 2, status: "unread", category: "regulation", tags: ["anthropic", "watermark"], summary: "Anthropic adds watermarks to all Claude outputs for traceability.", contentMD: md("Anthropic reportedly embeds **watermarks** in every Claude output, raising questions about builders who rely on those outputs."), fetchedAt: "2026-08-14T09:01:00Z", published: "2026-08-14T08:50:00Z" },
  { id: 3, sourceId: 1, sourceName: "HN RSS", url: "https://example.com/glm", title: "GLM-5.3: Frontier coding with emergent cyber capabilities", importance: 2, status: "unread", category: "models", tags: ["glm"], contentMD: md("GLM-5.3 pushes **frontier coding** capabilities and introduces new safety evaluations for autonomous cyber tasks."), fetchedAt: "2026-08-14T09:02:00Z" },
  { id: 4, sourceId: 2, sourceName: "HN Algolia", url: "https://example.com/rag", title: "Local RAG models beat cloud providers", importance: 1, status: "unread", category: "research", tags: ["rag"], summary: "New benchmarks show local retrieval pipelines winning.", contentMD: md("Local **RAG** pipelines now outperform cloud-hosted providers on latency-sensitive workloads."), fetchedAt: "2026-08-14T09:03:00Z" },
  { id: 5, sourceId: 2, sourceName: "HN Algolia", url: "https://example.com/agents", title: "Model Context Protocol (MCP): how agents connect to tools", importance: 2, status: "unread", category: "tools", tags: ["mcp", "agents"], contentMD: md("**MCP** standardizes how AI agents communicate with external tools and APIs."), fetchedAt: "2026-08-14T09:04:00Z" },
  { id: 6, sourceId: 3, sourceName: "DeepMind Blog", url: "https://deepmind.google/blog/agents", title: "Building more capable AI agents with world models", importance: 2, status: "unread", category: "research", tags: ["agents", "world-models"], contentMD: md("DeepMind explores **world models** as a substrate for longer-horizon agent planning."), fetchedAt: "2026-08-14T09:05:00Z" },
  { id: 7, sourceId: 2, sourceName: "HN Algolia", url: "https://example.com/eu", title: "The EU AI Act Is Now a Business-Blocking Risk for Vertical AI", importance: 3, status: "read", category: "regulation", tags: ["regulation", "eu"], summary: "Vertical AI firms face compliance hurdles under the EU AI Act.", contentMD: md("The **EU AI Act** now poses a business-blocking risk for vertical AI companies."), fetchedAt: "2026-08-14T09:06:00Z" },
  { id: 8, sourceId: 1, sourceName: "HN RSS", url: "https://example.com/banana", title: "Banana prices in Argentina jumped 40% this week", importance: 0, status: "unread", category: "general", tags: [], contentMD: md("An economic story far outside the AI beat."), fetchedAt: "2026-08-14T09:07:00Z" },
];

const sampleNotes: NoteDTO[] = [
  { id: 101, type: "electron", title: "ai agents", slug: "ai-agents", content: "# ai agents\n\n## Definition\nRecurring concept — referenced in 3 note(s).\n\n## Sources\n- [[model-context-protocol-mcp-explained]] — MCP: how agents connect to tools\n- [[building-more-capable-ai-agents]] — World models\n", tags: ["ai", "concept"], wikilinks: ["mcp"], createdAt: "2026-08-14T10:00:00Z" },
  { id: 102, type: "atom", title: "DeepSeek Harness developer preview", slug: "deepseek-harness-developer-preview", content: "---\ntype: atom\ncategory: models\nsource: HN RSS\nurl: https://deepseek.com/harness\nrating: 3\n---\n# DeepSeek Harness developer preview\n\n## Summary\nDeepSeek released a dev preview.\n\n" + lorem(20) + "\n\n## Connections\n- [[deepseek]]\n", tags: ["atom", "ai", "deepseek"], wikilinks: ["deepseek"], category: "models", rating: 3, url: "https://deepseek.com/harness", source: "HN RSS", createdAt: "2026-08-14T10:01:00Z" },
  { id: 103, type: "molecule", title: "Synthesis of models", slug: "synthesis-models", content: "# Synthesis of models\n\n## Central Idea\nThe current landscape is defined by performance and safety.\n\n## Connections\n- [[deepseek-harness-developer-preview]]\n", tags: ["synthesis", "ai"], wikilinks: ["deepseek-harness-developer-preview"], createdAt: "2026-08-14T10:02:00Z" },
  { id: 104, type: "atom", title: "Building more capable AI agents with world models", slug: "building-more-capable-ai-agents", content: "# Building more capable AI agents\n\n## Summary\nDeepMind explores world models.\n\n" + lorem(8) + "\n", tags: ["atom", "ai", "agents", "world-models"], wikilinks: ["world-models"], createdAt: "2026-08-14T10:03:00Z" },
  { id: 105, type: "atom", title: "Model Context Protocol (MCP): how agents connect to tools", slug: "model-context-protocol-mcp-explained", content: "# MCP explained\n\n## Summary\nMCP standardizes tool access.\n\n" + lorem(8) + "\n", tags: ["atom", "ai", "mcp"], wikilinks: ["mcp"], createdAt: "2026-08-14T10:04:00Z" },
  { id: 106, type: "electron", title: "world models", slug: "world-models", content: "# world models\n\n## Definition\nA substrate for longer-horizon agent planning.\n\n## Sources\n- [[building-more-capable-ai-agents]]\n", tags: ["ai", "concept"], wikilinks: ["building-more-capable-ai-agents"], createdAt: "2026-08-14T10:05:00Z" },
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

// In-memory jobs store so the jobs picker works in browser/mock mode. Mirrors
// the backend JobManager: running jobs show progress, finished ones persist.
let jobSeq = 0;
const mockJobs: JobDTO[] = [];
const mockDigests: DigestDTO[] = [];
const mockRules: RuleDTO[] = [
  { id: 1, name: "openai", query: "openai|gpt|chatgpt", actions: [{ type: "category", value: "industry" }, { type: "importance", value: "2" }], enabled: true },
  { id: 2, name: "anthropic", query: "anthropic|claude", actions: [{ type: "category", value: "models" }], enabled: true },
  // The two shapes the shipped ruleset is made of: a shield and a noise rule.
  { id: 3, name: "keep: labs and models", query: "\\b(openai|anthropic|gemini)\\b", actions: [{ type: "keep", value: "" }], enabled: true },
  { id: 4, name: "noise: crypto", query: "\\b(bitcoin|crypto|web3)\\b", actions: [{ type: "archive", value: "" }], enabled: true },
];

// A promotion queue with a couple of concepts already promoted, so the picker
// has both states to show.
const mockConcepts: ConceptDTO[] = [
  { slug: "openai", name: "OpenAI", mentions: 7, noteId: 301, promoted: true, firstSeen: "2026-08-01T09:00:00Z", lastSeen: "2026-08-14T09:00:00Z" },
  { slug: "rag", name: "RAG", mentions: 4, noteId: 302, promoted: true, firstSeen: "2026-08-03T09:00:00Z", lastSeen: "2026-08-13T09:00:00Z" },
  { slug: "watermarking", name: "Watermarking", mentions: 2, promoted: false, firstSeen: "2026-08-09T09:00:00Z", lastSeen: "2026-08-14T08:50:00Z" },
  { slug: "sparse-attention", name: "Sparse Attention", mentions: 1, promoted: false, firstSeen: "2026-08-12T09:00:00Z", lastSeen: "2026-08-12T09:00:00Z" },
  { slug: "open-ai", name: "Open AI", mentions: 1, promoted: false, firstSeen: "2026-08-11T09:00:00Z", lastSeen: "2026-08-11T09:00:00Z" },
];

function beginJob(name: string, type: string): number {
  const id = ++jobSeq;
  mockJobs.unshift({ id, name, type, status: "running", phase: "", done: 0, total: 0, createdAt: new Date().toISOString() });
  return id;
}
function patchJob(id: number, patch: Partial<JobDTO>): void {
  const j = mockJobs.find((x) => x.id === id);
  if (j) Object.assign(j, patch);
}
function finishJob(id: number, errMsg?: string): void {
  const j = mockJobs.find((x) => x.id === id);
  if (!j || j.status !== "running") return; // do not clobber a canceled job
  Object.assign(j, { status: errMsg ? "error" : "done", errMsg, finishedAt: new Date().toISOString() } as Partial<JobDTO>);
}

// Dense mode (?dense=1) simulates a full inbox (150 articles, long bodies) so
// scroll/wheel/layout issues can be reproduced and tested.
function denseArticles(): ArticleDTO[] {
  const titles = [
    "OpenAI ships a new frontier model with agentic tool use",
    "Anthropic expands Claude into enterprise workflows",
    "DeepSeek open-sources a reasoning model",
    "Google DeepMind improves world-model planning",
    "Meta releases an efficient small-language model",
    "EU AI Act enters a new enforcement phase",
    "A new RAG benchmark shows local beats cloud",
    "Quantization at the edge: 2-bit LLMs arrive",
    "Vector databases mature for production RAG",
    "Agentic coding assistants reach a plateau",
  ];
  const cats = ["models", "research", "industry", "regulation", "tools", "open-source", "funding", "opinion"];
  const out: ArticleDTO[] = [];
  for (let i = 0; i < 150; i++) {
    const t = titles[i % titles.length];
    const n = i + 1;
    out.push({
      id: n,
      sourceId: (i % 3) + 1,
      sourceName: ["HN RSS", "HN Algolia", "DeepMind Blog"][i % 3],
      url: `https://example.com/ai/${n}`,
      title: `${t} — report #${n}`,
      importance: (i % 4),
      status: "unread",
      category: cats[i % cats.length],
      tags: ["ai", "report"],
      summary: `Summary for report ${n}: a key development in the AI landscape that practitioners should track.`,
      contentMD: `${lorem(30)}\n\n## Key points\n\n- point one for #${n}\n- point two for #${n}`,
      fetchedAt: new Date(Date.now() - n * 3600_000).toISOString(),
    });
  }
  return out;
}

const dense = typeof window !== "undefined" && window.location?.search?.includes("dense");
const SOURCES = dense ? sampleSources : sampleSources;
const ARTICLES: ArticleDTO[] = dense ? denseArticles() : sampleArticles;
const NOTES = dense ? sampleNotes : sampleNotes;
const DIGEST = dense ? { ...sampleDigest, themes: sampleDigest.themes } : sampleDigest;

export const mockBackend: APIShape = {
  listSources: async (): Promise<SourceDTO[]> => { await delay(); return SOURCES.map((s) => ({ ...s })); },
  addSource: async (name: string, type: string, url: string, group: string): Promise<SourceDTO> => {
    await delay();
    const s: SourceDTO = { id: Date.now(), name, type, url, group: group || "general", enabled: true };
    SOURCES.push(s);
    return s;
  },
  setSourceEnabled: async (id: number, enabled: boolean): Promise<void> => {
    await delay();
    const s = SOURCES.find((x) => x.id === id);
    if (s) s.enabled = enabled;
  },
  deleteSource: async (id: number): Promise<void> => {
    await delay();
    const i = SOURCES.findIndex((s) => s.id === id);
    if (i >= 0) SOURCES.splice(i, 1);
  },

  listArticles: async (opts: ListArticlesOptions): Promise<ArticleDTO[]> => {
    await delay();
    let list = ARTICLES;
    if (opts.status) list = list.filter((a) => a.status === opts.status);
    if (opts.importanceMin) list = list.filter((a) => a.importance >= (opts.importanceMin ?? 0));
    if (opts.sourceId) list = list.filter((a) => a.sourceId === opts.sourceId);
    if (opts.category) list = list.filter((a) => a.category === opts.category);
    if (opts.unclassified) list = list.filter((a) => !a.category);
    if (opts.summarized) list = list.filter((a) => !!a.summary);
    if (opts.unarchived) list = list.filter((a) => a.status === "unread" || a.status === "read");
    if (opts.starred != null) list = list.filter((a) => (a.starred === true) === opts.starred);
    if (opts.importanceExact != null) list = list.filter((a) => a.importance === opts.importanceExact);
    if (opts.query) {
      const q = opts.query.toLowerCase();
      list = list.filter((a) => a.title.toLowerCase().includes(q) || (a.author ?? "").toLowerCase().includes(q));
    }
    return list;
  },
  listInbox: async (limit: number): Promise<ArticleDTO[]> => {
    await delay();
    // pending articles = those that have no matching atom note in the mock
    const titled = new Set(NOTES.map((n) => n.title));
    return ARTICLES.filter((a) => !titled.has(a.title)).slice(0, limit || 50);
  },
  getArticle: async (id: number): Promise<ArticleDTO> => {
    await delay();
    const a = ARTICLES.find((x) => x.id === id);
    if (!a) throw new Error("article not found");
    return { ...a };
  },
  getArticleContent: async (id: number): Promise<ArticleDTO> => {
    await delay(120); // simulate extraction
    const a = ARTICLES.find((x) => x.id === id);
    if (!a) throw new Error("article not found");
    return { ...a };
  },
  setArticleStatus: async (id: number, status: string): Promise<void> => {
    await delay();
    const a = ARTICLES.find((x) => x.id === id);
    if (a) a.status = status as ArticleDTO["status"];
  },
  setArticleStarred: async (id: number, starred: boolean): Promise<void> => {
    await delay();
    const a = ARTICLES.find((x) => x.id === id);
    if (a) a.starred = starred;
  },
  setArticleImportance: async (_id: number, _importance: number): Promise<void> => { await delay(); },

  fetch: async (): Promise<FetchResult> => {
    const id = beginJob("Fetch articles", "fetch");
    await delay(60);
    finishJob(id);
    return { newArticles: 3, updated: 0, sourcesFetched: 4, sourcesFailed: 0, extracted: 2, elapsedMs: 500 };
  },
  classify: async (_limit: number): Promise<ClassifyResult> => {
    const id = beginJob("Classify articles", "classify");
    patchJob(id, { phase: "rules", done: 8, total: 8 });
    await delay(50);
    patchJob(id, { phase: "llm", done: 8, total: 8 });
    await delay(80);
    finishJob(id);
    return { classified: 8, byRules: 2, byLLM: 6, skippedNoLLM: 0, batches: 1, errors: [] };
  },
  classifyArticles: async (ids: number[]): Promise<ClassifyResult> => {
    const id = beginJob(`Classify ${ids.length} selected`, "classify");
    patchJob(id, { phase: "rules", done: ids.length, total: ids.length });
    await delay(40);
    patchJob(id, { phase: "llm", done: ids.length, total: ids.length });
    await delay(80);
    for (const aid of ids) {
      const a = ARTICLES.find((x) => x.id === aid);
      if (a && !a.category) a.category = "general";
    }
    finishJob(id);
    return { classified: ids.length, byRules: 0, byLLM: ids.length, skippedNoLLM: 0, batches: 1, errors: [] };
  },
  summarizeArticle: async (id: number): Promise<ArticleDTO> => {
    const jid = beginJob("Summarize article", "summarize");
    await delay(80);
    const a = await mockBackend.getArticle(id);
    a.summary = "Mock summary: this article covers a key development in the AI landscape and why it matters for practitioners.";
    finishJob(jid);
    return a;
  },
  digest: async (): Promise<DigestDTO> => {
    const id = beginJob("Generate digest", "digest");
    await delay(80);
    finishJob(id);
    const d = { ...DIGEST, date: new Date().toISOString().slice(0, 10) };
    const idx = mockDigests.findIndex((x) => x.date === d.date);
    if (idx >= 0) mockDigests[idx] = d; else mockDigests.unshift(d);
    return d;
  },
  listDigests: async (): Promise<DigestMeta[]> => {
    await delay();
    return mockDigests.map((d) => ({ date: d.date, overview: d.overview }));
  },
  getDigest: async (date: string): Promise<DigestDTO | null> => {
    await delay();
    return mockDigests.find((d) => d.date === date) ?? null;
  },
  flow: async (): Promise<FlowStatus> => {
    await delay();
    return {
      sourcesTotal: SOURCES.length, sourcesEnabled: SOURCES.filter((s) => s.enabled).length,
      articlesTotal: ARTICLES.length,
      classified: ARTICLES.filter((a) => a.category).length,
      pendingClassify: ARTICLES.filter((a) => !a.category).length,
      atoms: NOTES.filter((n) => n.type === "atom").length,
      electrons: NOTES.filter((n) => n.type === "electron").length,
      molecules: NOTES.filter((n) => n.type === "molecule").length,
      vaultPath: "/mock/vault",
      notesEmbedded: NOTES.length, articlesEmbedded: ARTICLES.length,
      runningJobs: mockJobs.filter((j) => j.status === "running").length,
    };
  },
  logs: async (): Promise<string> => {
    await delay();
    return [
      "giznews: 2026/08/16 10:00:01 fetching HN RSS (rss)",
      "giznews: 2026/08/16 10:00:02 source HN RSS: 12 new, 0 updated, 3 dups",
      "giznews: 2026/08/16 10:00:03 classifying batch 1/10 (10 articles)",
      "giznews: 2026/08/16 10:00:05 batch 1/10: 10 classified in 2.1s",
      "giznews: 2026/08/16 10:00:07 classifying batch 2/10 (10 articles)",
      "giznews: 2026/08/16 10:00:09 batch 2/10: 10 classified in 1.9s",
      "giznews: 2026/08/16 10:00:11 extracted 4 article bodies",
    ].join("\n");
  },

  listRules: async (): Promise<RuleDTO[]> => { await delay(); return mockRules.map((r) => ({ ...r, actions: [...r.actions] })); },
  addRule: async (name: string, query: string, actions: RuleActionDTO[], enabled: boolean): Promise<RuleDTO> => {
    await delay();
    const r: RuleDTO = { id: Date.now(), name, query, actions, enabled };
    mockRules.unshift(r);
    return { ...r };
  },
  updateRule: async (id: number, name: string, query: string, actions: RuleActionDTO[], enabled: boolean): Promise<RuleDTO> => {
    await delay();
    const r = mockRules.find((x) => x.id === id);
    if (!r) throw new Error("rule not found");
    r.name = name; r.query = query; r.actions = actions; r.enabled = enabled;
    return { ...r, actions: [...r.actions] };
  },
  setRuleEnabled: async (id: number, enabled: boolean): Promise<void> => {
    await delay();
    const r = mockRules.find((x) => x.id === id);
    if (r) r.enabled = enabled;
  },
  deleteRule: async (id: number): Promise<void> => {
    await delay();
    const i = mockRules.findIndex((x) => x.id === id);
    if (i >= 0) mockRules.splice(i, 1);
  },

  listConcepts: async (): Promise<ConceptDTO[]> => {
    await delay();
    return mockConcepts.map((c) => ({ ...c }));
  },
  promoteConcept: async (slug: string): Promise<NoteDTO> => {
    await delay(60);
    const c = mockConcepts.find((x) => x.slug === slug);
    if (!c) throw new Error("concept not found");
    const note: NoteDTO = {
      id: 900 + mockConcepts.indexOf(c),
      type: "electron",
      title: c.name,
      slug: c.slug,
      content: `# ${c.name}\n\nReferenced in ${c.mentions} note(s).`,
      tags: ["ai", "concept"],
      wikilinks: [],
      createdAt: new Date().toISOString(),
    };
    c.promoted = true;
    c.noteId = note.id;
    NOTES.push(note);
    return note;
  },
  mergeConcepts: async (from: string, to: string): Promise<MergeDTO> => {
    await delay(60);
    const i = mockConcepts.findIndex((c) => c.slug === from);
    const target = mockConcepts.find((c) => c.slug === to);
    if (i < 0 || !target) throw new Error("concept not found");
    target.mentions += mockConcepts[i].mentions;
    const redirected = mockConcepts[i].promoted;
    mockConcepts.splice(i, 1);
    return { notesRelinked: 1, mentions: target.mentions, redirected };
  },

  kbuild: async (): Promise<KBResult> => {
    const id = beginJob("Build knowledge graph", "kb");
    await delay(60);
    finishJob(id);
    return { atomsCreated: 5, electronsCreated: 2, electronsUpdated: 0, moleculesCreated: 1, moleculesUpdated: 2, articlesSkipped: 0, conceptsTracked: 18, atomsRefreshed: 2, editedNotesKept: 1, notesImported: 3 };
  },
  kthemes: async (): Promise<KBResult> => {
    const id = beginJob("Gather themes", "kb");
    await delay(60);
    finishJob(id);
    return { atomsCreated: 0, electronsCreated: 0, electronsUpdated: 0, moleculesCreated: 1, moleculesUpdated: 2, articlesSkipped: 0, conceptsTracked: 0, atomsRefreshed: 0, editedNotesKept: 0, notesImported: 0 };
  },
  ksynthesize: async (_category: string): Promise<KBResult> => {
    const id = beginJob("Synthesize category", "kb");
    await delay(60);
    finishJob(id);
    return { atomsCreated: 0, electronsCreated: 0, electronsUpdated: 0, moleculesCreated: 1, moleculesUpdated: 0, articlesSkipped: 0, conceptsTracked: 0, atomsRefreshed: 0, editedNotesKept: 0, notesImported: 0 };
  },
  ensureArticleNote: async (articleID: number): Promise<NoteDTO> => {
    await delay(60);
    const art = ARTICLES.find((a) => a.id === articleID);
    const title = art ? art.title : `Article ${articleID}`;
    const note: NoteDTO = {
      id: 10000 + articleID,
      type: "atom",
      title,
      slug: title.toLowerCase().replace(/[^a-z0-9]+/g, "-"),
      content: `# ${title}\n\n## Summary\n\nNote generated for this article.\n`,
      tags: ["atom", "ai"],
      wikilinks: [],
      createdAt: new Date().toISOString(),
    };
    if (!NOTES.some((n) => n.title === title)) NOTES.push(note);
    return note;
  },
  getArticleNote: async (articleID: number): Promise<NoteDTO | null> => {
    await delay();
    const art = ARTICLES.find((a) => a.id === articleID);
    if (!art) return null;
    return NOTES.find((n) => n.title === art.title) ?? null;
  },
  listNotes: async (type: string): Promise<NoteDTO[]> => { await delay(); return NOTES.filter((n) => (type ? n.type === type : true)); },
  getNote: async (id: number): Promise<NoteDTO> => {
    await delay();
    const n = NOTES.find((x) => x.id === id);
    if (!n) throw new Error("note not found");
    return { ...n };
  },
  graphNeighbors: async (_id: number): Promise<NoteDTO[]> => {
    await delay();
    return NOTES.slice(0, 2);
  },

  searchIndex: async (): Promise<IndexResult> => {
    const id = beginJob("Index search", "index");
    await delay();
    finishJob(id);
    return { notesEmbedded: 3, articlesEmbedded: 8, ftsNotes: 3, ftsArticles: 8, embeddingsFailed: 0 };
  },
  search: async (_q: string, _limit: number): Promise<SearchResultDTO[]> => { await delay(60); return sampleSearch; },

  listJobs: async (): Promise<JobDTO[]> => { await delay(5); return mockJobs.map((j) => ({ ...j })); },
  removeJob: async (id: number): Promise<void> => {
    const i = mockJobs.findIndex((j) => j.id === id);
    if (i >= 0) mockJobs.splice(i, 1);
  },
  clearFinishedJobs: async (): Promise<void> => {
    for (let i = mockJobs.length - 1; i >= 0; i--) {
      if (mockJobs[i].status !== "running") mockJobs.splice(i, 1);
    }
  },
  cancelJob: async (id: number): Promise<void> => {
    const j = mockJobs.find((x) => x.id === id);
    if (j && j.status === "running") Object.assign(j, { status: "canceled", finishedAt: new Date().toISOString() });
  },
  bulkSetStatus: async (ids: number[], status: string): Promise<BulkResult> => {
    const id = beginJob(`Mark ${ids.length} ${status}`, "bulk");
    for (let i = 0; i < ids.length; i++) {
      await delay(30);
      patchJob(id, { phase: "bulk", done: i + 1, total: ids.length });
      const a = ARTICLES.find((x) => x.id === ids[i]);
      if (a) a.status = status as ArticleDTO["status"];
    }
    finishJob(id);
    return { updated: ids.length, total: ids.length };
  },
  ingestURL: async (url: string): Promise<ArticleDTO> => {
    const id = beginJob("Ingest URL", "ingest");
    patchJob(id, { phase: "fetch", done: 0, total: 1 });
    await delay(90);
    patchJob(id, { phase: "done", done: 1, total: 1 });
    const article: ArticleDTO = {
      id: Date.now(),
      sourceId: 0,
      sourceName: "Manual",
      url,
      title: url.replace(/^https?:\/\//, "").replace(/\/$/, ""),
      importance: 0,
      status: "unread",
      category: "",
      tags: [],
      contentMD: md(`Ingested from ${url}.\n\nThis is the extracted body of the linked article.`),
      fetchedAt: new Date().toISOString(),
    };
    ARTICLES.unshift(article);
    finishJob(id);
    return { ...article };
  },

  status: async (): Promise<StatusDTO> => {
    await delay();
    return { dbPath: "/mock/db", vaultPath: "/mock/vault", llmProvider: "ollama", llmEnabled: true, llmReachable: true, embeddingsModel: "nomic-embed-text", unreadArticles: ARTICLES.filter((a) => a.status === "unread").length, totalArticles: ARTICLES.length, totalNotes: NOTES.length, pendingClassify: 12 };
  },

  openURL: async (_url: string): Promise<void> => { await delay(); },
  openVault: async (): Promise<void> => { await delay(); },
  quit: async (): Promise<void> => { await delay(); },
};
