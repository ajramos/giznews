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
    // mock has 3 notes (atom + electron + molecule) and 1 wikilink edge
    await expect(page.locator(".graph-node")).toHaveCount(3, { timeout: 6000 });
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
    await expect(page.locator(".graph-node")).toHaveCount(3, { timeout: 6000 });
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
    await page.locator(".modal").getByRole("button", { name: "Añadir" }).click();
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
    await expect(page.locator(".modal")).toContainText("¿Eliminar");
    await page.locator(".modal").getByRole("button", { name: "Eliminar" }).click();
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
    await expect(page.locator(".article-row")).toHaveCount(7);
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

  test(":procesar runs the pipeline and shows per-step results", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("procesar");
    await page.keyboard.press("Enter");
    await expect(page.locator(".pipeline-modal")).toBeVisible();
    await expect(page.locator(".pipeline-step")).toHaveCount(4);
    // all steps finish (mock is fast)
    await expect(page.locator(".pipeline-step .pl-status.done")).toHaveCount(4, { timeout: 10000 });
    await expect(page.locator(".pipeline-step").nth(0)).toContainText("nuevos");
    await page.locator(".pipeline-modal").getByRole("button", { name: "Cerrar" }).click();
    await expect(page.locator(".pipeline-modal")).toHaveCount(0);
  });

  test(":status opens the status modal", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press(":");
    await page.locator(".palette input").fill("status");
    await page.keyboard.press("Enter");
    await expect(page.locator(".status-modal")).toBeVisible();
    await expect(page.locator(".status-modal")).toContainText("Artículos");
    await expect(page.locator(".status-modal")).toContainText("Atoms");
    await page.keyboard.press("Escape");
    await expect(page.locator(".status-modal")).toHaveCount(0);
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
