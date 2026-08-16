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
        { id: 1, source_id: 1, source_name: "HN RSS", url: "https://x.com/1", title: "Wire-shape article one", content_md: "# One\\n\\nBody with enough content to render markdown in the reader panel.", summary: "s1", importance: 3, status: "unread", category: "models", tags: ["ai"], published: new Date().toISOString(), fetched_at: new Date().toISOString() },
        { id: 2, source_id: 2, source_name: "DeepMind", url: "https://x.com/2", title: "Wire-shape article two", content_md: "", importance: 1, status: "read", category: "tools", tags: [], published: new Date().toISOString(), fetched_at: new Date().toISOString() },
      ];
      return opts && opts.source_id ? items.filter(i => i.source_id === opts.source_id) : items;
    },
    GetArticleContent: async (id) => ({ id, source_id: 1, source_name: "HN RSS", url: "https://x.com/" + id, title: id === 1 ? "Wire-shape article one" : "Wire-shape article two", content_md: "# Loaded\\n\\nThis content arrived through the real snake_case contract.", summary: "s", importance: 3, status: "unread", category: "models", tags: ["ai"], fetched_at: new Date().toISOString() }),
    SetArticleStatus: async () => {},
    Status: async () => ({ db_path: "/tmp/db", vault_path: "/tmp/vault", llm_provider: "ollama", llm_enabled: true, llm_reachable: true, embeddings_model: "nomic-embed-text", unread_articles: 1, total_articles: 2, total_notes: 3 }),
    Fetch: async () => ({ new_articles: 0, updated: 0, sources_fetched: 0, sources_failed: 0, extracted: 0, elapsed_ms: 1 }),
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
