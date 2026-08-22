import { test, expect } from "@playwright/test";
import { gotoApp, press, shot, trackErrors } from "./helpers";

test.describe("panels", () => {
  test("search panel runs a query and opens a result", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await press(page, "s");
    await expect(page.locator(".search-panel")).toBeVisible();
    await page.locator(".search-panel input").fill("watermark");
    await expect(page.locator(".search-panel .result-row").first()).toBeVisible({ timeout: 6000 });
    await shot(page, "07-search");
    await page.locator(".search-panel .result-row").first().click();
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
    assertNoErrors();
  });

  test("graph shows the full knowledge graph", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await press(page, "g"); // single g → graph after the 300ms window
    await expect(page.locator(".graph-panel")).toBeVisible({ timeout: 4000 });
    // mock has 6 notes (atoms + electrons + molecule) and wikilink edges
    await expect(page.locator(".graph-node")).toHaveCount(6, { timeout: 6000 });
    await expect(page.locator(".links path")).not.toHaveCount(0);
    await expect(page.locator(".graph-legend")).toContainText("atom");
    await shot(page, "08-graph");
    await press(page, "Escape");
    await expect(page.locator(".graph-panel")).toHaveCount(0);
    assertNoErrors();
  });

  test("clicking a graph node opens the note", async ({ page }) => {
    await gotoApp(page);
    await press(page, "g");
    await expect(page.locator(".graph-node")).toHaveCount(6, { timeout: 6000 });
    await page.waitForTimeout(1200); // let the force layout + fit-zoom settle
    await page.locator(".graph-node circle").first().click({ force: true });
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
  });

  test("palette fetch command runs and shows a toast", async ({ page }) => {
    await gotoApp(page);
    await press(page, ":");
    await page.locator(".palette input").fill("fetch");
    await page.keyboard.press("Enter");
    await expect(page.locator(".toast")).toBeVisible({ timeout: 6000 });
  });

  test("digest article click opens it in the reader", async ({ page }) => {
    await gotoApp(page);
    await press(page, "d");
    await expect(page.locator(".digest-view")).toBeVisible({ timeout: 6000 });
    await page.locator(".digest-articles li").first().click();
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
    await shot(page, "09-digest-open");
  });

  test("digest keyboard: j moves focus, Enter opens", async ({ page }) => {
    await gotoApp(page);
    await press(page, "d");
    await expect(page.locator(".digest-view")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".digest-articles li.selected")).toHaveCount(1);
    const firstTitle = await page.locator(".digest-articles li.selected .dart-title").innerText();
    await press(page, "j");
    await expect(page.locator(".digest-articles li.selected .dart-title")).not.toHaveText(firstTitle);
    await press(page, "Enter");
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
  });
});

test.describe("sources", () => {
  test("source picker via :sources, keyboard toggle", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("sources");
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker")).toBeVisible();
    await expect(page.locator(".source-picker-item")).toHaveCount(4);
    // Enter toggles the focused source
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker-item").first().locator(".sp-dot")).toHaveAttribute("data-on", "false");
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker-item").first().locator(".sp-dot")).toHaveAttribute("data-on", "true");
    await page.keyboard.press("Escape");
    await expect(page.locator(".source-picker")).toHaveCount(0);
  });

  test("unhealthy source is flagged in the picker and the status bar", async ({ page }) => {
    await gotoApp(page);
    // The mock marks HN Algolia as failing (3 consecutive failures).
    await expect(page.locator(".pill.unhealthy")).toContainText("1 source(s) failing");
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("sources");
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker")).toBeVisible();
    const rows = page.locator(".source-picker-item");
    await expect(rows).toHaveCount(4);
    // Healthy rows carry no unhealthy flag.
    await expect(rows.first().locator(".sp-dot")).toHaveAttribute("data-unhealthy", "false");
    // HN Algolia (row 2) is red and shows the error in its meta.
    const bad = rows.nth(1);
    await expect(bad.locator(".sp-dot")).toHaveAttribute("data-unhealthy", "true");
    await expect(bad.locator(".sp-meta")).toContainText("timeout");
    await expect(bad).toHaveAttribute("title", /Last error: timeout after 15s/);
    await page.keyboard.press("Escape");
  });

  test("add source via picker modal", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("sources");
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker")).toBeVisible();
    await page.keyboard.press("a"); // add
    await expect(page.locator(".modal")).toBeVisible();
    await page.locator(".modal input").nth(0).fill("MIT Tech Review");
    await page.locator(".modal input").nth(1).fill("https://www.technologyreview.com/feed/");
    await page.locator(".modal").getByRole("button", { name: "Add" }).click();
    await expect(page.locator(".modal")).toHaveCount(0); // wait for save
    // back to the picker to confirm the new source
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("sources");
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker-item")).toHaveCount(5);
    await expect(page.locator(".source-picker-item", { hasText: "MIT Tech Review" })).toHaveCount(1);
    await shot(page, "10-source-added");
  });

  test("delete source via picker", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("sources");
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker-item")).toHaveCount(4);
    await page.keyboard.press("d"); // delete focused (first)
    await expect(page.locator(".modal")).toContainText("Delete");
    await page.locator(".modal").getByRole("button", { name: "Delete" }).click();
    await expect(page.locator(".modal")).toHaveCount(0); // wait for confirm
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("sources");
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker-item")).toHaveCount(3);
  });

  test("source picker filter (f) filters the article list", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("sources");
    await page.keyboard.press("Enter");
    await expect(page.locator(".source-picker")).toBeVisible();
    // mock order: HN RSS(0), HN Algolia(1), DeepMind Blog(2), arXiv(3)
    await page.keyboard.press("j");
    await page.keyboard.press("j");
    await page.keyboard.press("f"); // filter by DeepMind Blog
    await expect(page.locator(".source-picker")).toHaveCount(0);
    await expect(page.locator(".article-row")).toHaveCount(1);
    await expect(page.locator(".topbar .pill.filter")).toBeVisible();
    await page.locator(".topbar .pill.filter").click();
    await expect(page.locator(".article-row")).toHaveCount(8);
  });
});

test.describe("themes", () => {
  test("theme modal via :theme switches data-theme", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("theme");
    await page.keyboard.press("Enter");
    await expect(page.locator(".theme-modal")).toBeVisible();
    await page.locator(".theme-modal .theme-opt", { hasText: "Dracula" }).click();
    const attr = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    expect(attr).toBe("dracula");
    await shot(page, "11-dracula");
  });

  test("theme modal keyboard navigation", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("theme");
    await page.keyboard.press("Enter");
    await expect(page.locator(".theme-modal")).toBeVisible();
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Enter");
    const attr = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    expect(attr).toBe("dracula");
  });

  test(":theme command opens the theme modal", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("theme");
    await page.keyboard.press("Enter");
    await expect(page.locator(".theme-modal")).toBeVisible();
    await expect(page.locator(".theme-modal .theme-opt")).toHaveCount(5);
    await page.locator(".theme-modal .theme-opt", { hasText: "Nord" }).click();
    const attr = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    expect(attr).toBe("nord");
    await expect(page.locator(".theme-modal")).toHaveCount(0);
  });
});

test.describe("workflows", () => {
  test("auto-refresh toggle in the status bar", async ({ page }) => {
    await gotoApp(page);
    const pill = page.locator(".statusbar .pill.auto");
    await expect(pill).toContainText("auto 15m");
    await pill.click();
    await expect(pill).toContainText("auto off");
    await pill.click();
    await expect(pill).toContainText("auto 15m");
  });

  test(":process runs the pipeline as background jobs", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("process");
    await page.keyboard.press("Enter");
    await expect(page.locator(".jobs-picker")).toBeVisible();
    await expect(page.locator(".job-item")).toHaveCount(4, { timeout: 10000 });
    await expect(page.locator(".job-status.done")).toHaveCount(4, { timeout: 10000 });
    await page.keyboard.press("Escape");
    await expect(page.locator(".jobs-picker")).toHaveCount(0);
  });

  test(":classify runs in the background and shows progress in the jobs panel", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("classify");
    await page.keyboard.press("Enter");
    await expect(page.locator(".jobs-picker")).toBeVisible();
    await expect(page.locator(".job-item", { hasText: "Classify articles" })).toHaveCount(1, { timeout: 10000 });
    await expect(page.locator(".job-status.done")).toHaveCount(1, { timeout: 10000 });
    await page.keyboard.press("Escape");
  });

  test(":classify rules-only applies the prefilter and leaves the rest pending", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("classify rules-only");
    await page.keyboard.press("Enter");
    await expect(page.locator(".jobs-picker")).toBeVisible();
    await expect(page.locator(".job-item", { hasText: "Apply rules" })).toHaveCount(1, { timeout: 10000 });
    await expect(page.locator(".toast")).toContainText("resolved by rules", { timeout: 6000 });
    await page.keyboard.press("Escape");
  });

  test(":status opens the status modal", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("status");
    await page.keyboard.press("Enter");
    await expect(page.locator(".status-modal")).toBeVisible();
    await expect(page.locator(".status-modal")).toContainText("Articles");
    await expect(page.locator(".status-modal")).toContainText("Atoms");
    await page.keyboard.press("Escape");
    await expect(page.locator(".status-modal")).toHaveCount(0);
  });

  test(":url adds an article by URL and opens it", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("url");
    await page.keyboard.press("Enter");
    await expect(page.locator(".prompt-modal")).toBeVisible();
    await page.locator(".prompt-modal input").fill("https://example.com/blog/ingested");
    await page.keyboard.press("Enter");
    await expect(page.locator(".toast")).toContainText("Added", { timeout: 6000 });
    await expect(page.locator(".reader-head h1")).toContainText("example.com", { timeout: 6000 });
  });

  test("archive runs as a bulk background job", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press("a"); // range verb
    await page.keyboard.press("5"); // count
    await page.keyboard.press("a"); // apply → archive 5
    await page.keyboard.press("z"); // open jobs panel
    await expect(page.locator(".jobs-picker")).toBeVisible();
    await expect(page.locator(".job-item", { hasText: "Mark 5 archived" })).toHaveCount(1, { timeout: 6000 });
    await page.keyboard.press("Escape");
  });

  test(":kb synth opens a prompt modal", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("kb synth");
    await page.keyboard.press("Enter");
    await expect(page.locator(".prompt-modal")).toBeVisible();
    await page.locator(".prompt-modal input").fill("models");
    await page.keyboard.press("Enter");
    await expect(page.locator(".toast")).toBeVisible({ timeout: 6000 });
  });
});

test.describe("digest export", () => {
  test(":digest export writes a file and says where", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("digest export");
    await page.keyboard.press("Enter");
    await expect(page.locator(".toast")).toContainText("Digest written to", { timeout: 6000 });
    await expect(page.locator(".toast")).toContainText(".html");
  });
});

test.describe("ask", () => {
  test(":ask answers from the notes, and every citation opens one", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("ask");
    await page.keyboard.press("Enter");
    await expect(page.locator(".ask-panel")).toBeVisible();

    await page.locator(".ask-panel input").fill("what do I know about sparse attention?");
    await page.keyboard.press("Enter");
    await expect(page.locator('[data-testid="ask-answer"]')).toBeVisible({ timeout: 6000 });

    // Citations are buttons, not decoration.
    const citations = page.locator(".ask-answer .citation");
    await expect(citations.first()).toHaveText("ai-agents");
    await shot(page, "21-ask");

    // An invented citation is reported rather than hidden.
    await expect(page.locator('[data-testid="ask-dropped"]')).toContainText("imaginary-note");

    // Clicking one opens that note in the reader.
    await citations.first().click();
    await expect(page.locator(".ask-panel")).toHaveCount(0);
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
  });

  test("a question the vault knows nothing about is not answered", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("ask");
    await page.keyboard.press("Enter");
    await page.locator(".ask-panel input").fill("zzz nothing at all");
    await page.keyboard.press("Enter");
    await expect(page.locator('[data-testid="ask-ungrounded"]')).toContainText("Nothing in your vault");
  });
});
