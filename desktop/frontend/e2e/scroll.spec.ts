import { test, expect } from "@playwright/test";
import { shot } from "./helpers";

// Dense mode (?dense=1) simulates a full inbox (150 articles, long bodies) so
// scroll/wheel/layout-regression issues are reproducible.
async function gotoDense(page: import("@playwright/test").Page): Promise<void> {
  await page.addInitScript(() => localStorage.setItem("giznews-welcomed", "1"));
  await page.goto("/?dense=1");
  await expect(page.locator(".article-row").first()).toBeVisible({ timeout: 8000 });
}

test.describe("scroll & layout", () => {
  test("document never exceeds the viewport (no layout break)", async ({ page }) => {
    await gotoDense(page);
    const doc = await page.evaluate(() => document.body.scrollHeight);
    expect(doc).toBeLessThanOrEqual(900);
  });

  test("mouse wheel scrolls the article list", async ({ page }) => {
    await gotoDense(page);
    await page.mouse.move(200, 400); // list column is the left 380px now
    await page.mouse.wheel(0, 600);
    await expect.poll(() => page.locator(".article-list").evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
  });

  test("mouse wheel scrolls the reader", async ({ page }) => {
    await gotoDense(page);
    await page.keyboard.press("Enter");
    await expect(page.locator(".reader-scroll")).toBeVisible({ timeout: 6000 });
    await page.mouse.move(900, 500);
    await page.mouse.wheel(0, 600);
    await expect.poll(() => page.locator(".reader-scroll").evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
  });

  test("reader loads article content after Enter", async ({ page }) => {
    await gotoDense(page);
    await page.keyboard.press("Enter");
    await expect(page.locator(".reader-scroll .markdown").first()).toContainText("Paragraph 1", { timeout: 6000 });
    await shot(page, "12-dense-reader");
  });

  test("sources panel scrolls internally when dense", async ({ page }) => {
    await gotoDense(page);
    // sources only has 4 entries; ensure the pane doesn't stretch the document
    const doc = await page.evaluate(() => document.body.scrollHeight);
    expect(doc).toBeLessThanOrEqual(900);
  });

  test("wide images and long URLs do not overflow the reader", async ({ page }) => {
    await gotoDense(page);
    await page.keyboard.press("Enter");
    await expect(page.locator(".reader-scroll")).toBeVisible({ timeout: 6000 });
    // inject a very wide image + an unbreakable long token into the markdown
    await page.locator(".reader-scroll .markdown").evaluate((el) => {
      el.innerHTML =
        '<img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7" width="2000" height="40" alt="wide">' +
        '<p>https://very-long-url.example.com/path/' + "x".repeat(400) + "</p>";
    });
    const overflow = await page.locator(".reader-scroll").evaluate((el) => el.scrollWidth - el.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  });
});

test.describe("status bar", () => {
  test("llm pill shows provider when enabled and reachable", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("giznews-welcomed", "1"));
    await page.goto("/");
    await expect(page.locator(".statusbar .pill.llm.on")).toContainText("ollama");
  });
});
