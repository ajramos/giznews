import { test, expect } from "@playwright/test";

// Simulates the REAL Wails wire shape (snake_case json tags) so the
// serialization contract is tested end-to-end against the frontend.
const FAKE = `
  window.__wailsShape = true;
  const delay = (ms = 20) => new Promise(r => setTimeout(r, ms));
  window.go = window.go || {};
  window.go.main = { App: {
    ListSources: async () => [
      { id: 1, name: "HN RSS", type: "rss", url: "u", group: "community", enabled: true },
      { id: 2, name: "DeepMind", type: "rss", url: "d", group: "labs", enabled: true },
    ],
    ListArticles: async (opts) => {
      const items = [
        { id: 1, source_id: 1, source_name: "HN RSS", url: "https://x.com/1", title: "Wire-shape article one", content_md: "# One\\n\\nBody with enough content to render markdown in the reader panel.", summary: "s1", importance: 3, status: "unread", starred: false, category: "models", tags: ["ai"], published: new Date().toISOString(), fetched_at: new Date().toISOString(), story_size: 3, story_sources: ["HN RSS", "DeepMind", "Reuters"] },
        { id: 2, source_id: 2, source_name: "DeepMind", url: "https://x.com/2", title: "Wire-shape article two", content_md: "", importance: 1, status: "read", starred: true, category: "tools", tags: [], published: new Date().toISOString(), fetched_at: new Date().toISOString() },
      ];
      return opts && opts.source_id ? items.filter(i => i.source_id === opts.source_id) : items;
    },
    GetArticleContent: async (id) => ({ id, source_id: 1, source_name: "HN RSS", url: "https://x.com/" + id, title: id === 1 ? "Wire-shape article one" : "Wire-shape article two", content_md: "# Loaded\\n\\nThis content arrived through the real snake_case contract.", summary: "s", importance: 3, status: "unread", category: "models", tags: ["ai"], fetched_at: new Date().toISOString() }),
    SetArticleStatus: async () => {},
    Status: async () => ({ db_path: "/tmp/db", vault_path: "/tmp/vault", llm_provider: "ollama", llm_enabled: true, llm_reachable: true, embeddings_model: "nomic-embed-text", unread_articles: 1, total_articles: 2, total_notes: 3 }),
    Fetch: async () => ({ new_articles: 0, updated: 0, sources_fetched: 0, sources_failed: 0, extracted: 0, elapsed_ms: 1 }),
    Classify: async () => ({ classified: 8, by_rules: 2, by_llm: 6, skipped_no_llm: 0, batches: 1, errors: [] }),
    ClassifyArticles: async () => ({ classified: 2, by_rules: 1, by_llm: 1, skipped_no_llm: 0, batches: 1, errors: [] }),
    KBuild: async () => ({ atoms_created: 3, electrons_created: 1, electrons_updated: 0, molecules_created: 0, articles_skipped: 0 }),
    KThemes: async () => ({ atoms_created: 0, electrons_created: 0, electrons_updated: 0, molecules_created: 2, molecules_updated: 3, articles_skipped: 0 }),
    ListConcepts: async () => ([
      { slug: "openai", name: "OpenAI", mentions: 7, note_id: 42, promoted: true, first_seen: "2026-08-01T09:00:00Z", last_seen: "2026-08-14T09:00:00Z" },
      { slug: "watermarking", name: "Watermarking", mentions: 1, promoted: false, first_seen: "2026-08-09T09:00:00Z", last_seen: "2026-08-09T09:00:00Z" },
    ]),
    GetNote: async (id) => ({ id, type: "electron", title: "OpenAI", slug: "openai", content: "A concept note.", tags: ["ai", "concept"], wikilinks: [], created_at: "2026-08-14T09:00:00Z" }),
    ListJobs: async () => [],
    AddSource: async (a,b,c,d) => ({ id: 9, name: a, type: b, url: c, group: d, enabled: true }),
  } };
`;

test("real Wails snake_case wire shape renders correctly", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("giznews-welcomed", "1"));
  await page.addInitScript(FAKE);
  await page.goto("/");
  await expect(page.locator(".article-row").first()).toBeVisible({ timeout: 8000 });

  // snake_case fields map to the camelCase TS fields:
  await expect(page.locator(".article-row")).toHaveCount(2);
  await expect(page.locator(".article-row").first()).toContainText("Wire-shape article one");
  await expect(page.locator(".list-head .view-count").first()).toHaveText("1");
  // story_size / story_sources map through to the coverage badge
  await expect(page.locator(".article-row").first().locator(".src-count")).toHaveText("+2 more");
  // starred field maps through (article two is starred + read, stays in Active)
  await expect(page.locator(".article-row .star-badge")).toHaveCount(1);
  // LLM enabled + reachable → green pill with provider name
  await expect(page.locator(".statusbar .pill.llm.on")).toContainText("ollama");

  // Source filter arg is sent as source_id (via the :sources picker → f)
  await page.keyboard.press(":");
  await page.locator(".palette input").fill("sources");
  await page.keyboard.press("Enter");
  await expect(page.locator(".source-picker")).toBeVisible();
  await page.keyboard.press("f"); // filter by the first source (HN RSS, id 1)
  await expect(page.locator(".article-row")).toHaveCount(1);

  // Content loads through GetArticleContent
  await page.keyboard.press("Enter");
  await expect(page.locator(".reader-scroll .markdown")).toContainText("snake_case contract", { timeout: 6000 });
});

test("kb build maps snake_case KBResult into the toast", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("giznews-welcomed", "1"));
  await page.addInitScript(FAKE);
  await page.goto("/");
  await expect(page.locator(".article-row").first()).toBeVisible({ timeout: 8000 });

  await page.keyboard.press(":");
  await page.locator(".palette input").fill("kb build");
  await page.keyboard.press("Enter");
  await expect(page.locator(".toast")).toContainText("3 atoms · 1 electrons", { timeout: 6000 });
});

test("kb themes maps molecules_created/updated into the toast", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("giznews-welcomed", "1"));
  await page.addInitScript(FAKE);
  await page.goto("/");
  await expect(page.locator(".article-row").first()).toBeVisible({ timeout: 8000 });

  await page.keyboard.press(":");
  await page.locator(".palette input").fill("kb themes");
  await page.keyboard.press("Enter");
  await expect(page.locator(".toast")).toContainText("2 theme(s) written · 3 refreshed", { timeout: 6000 });
});

test("concepts wire shape: note_id reaches the note it opens", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("giznews-welcomed", "1"));
  await page.addInitScript(FAKE);
  await page.goto("/");
  await expect(page.locator(".article-row").first()).toBeVisible({ timeout: 8000 });

  await page.keyboard.press(":");
  await page.locator(".palette input").fill("concepts");
  await page.keyboard.press("Enter");
  const rows = page.locator('[data-testid="concept-row"]');
  await expect(rows).toHaveCount(2);
  // mentions and promoted map through from snake_case
  await expect(rows.first()).toContainText("7×");
  await expect(rows.first().locator(".sp-dot")).toHaveAttribute("data-on", "true");
  await expect(rows.nth(1).locator(".sp-dot")).toHaveAttribute("data-on", "false");

  // note_id → noteId: Enter on a promoted concept opens its note rather than
  // trying to create one. Getting this name wrong is invisible in the mock.
  await page.keyboard.press("Enter");
  await expect(page.locator(".reader-head h1")).toHaveText("OpenAI", { timeout: 6000 });
  await expect(page.locator(".reader .note-type")).toHaveText("electron");
});
