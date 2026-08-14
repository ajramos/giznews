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

  test("graph panel renders an SVG graph", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await press(page, "g"); // single g → graph after the 300ms window
    await expect(page.locator(".graph-panel")).toBeVisible({ timeout: 4000 });
    await expect(page.locator(".graph-current h2")).toContainText("DeepSeek Harness", { timeout: 4000 });
    // at least the center node + one neighbor are drawn
    await expect(page.locator(".graph-canvas circle")).not.toHaveCount(0);
    await expect(page.locator(".graph-canvas line")).not.toHaveCount(0);
    await shot(page, "08-graph");
    await press(page, "Escape");
    await expect(page.locator(".graph-panel")).toHaveCount(0);
    assertNoErrors();
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
  test("theme picker switches the data-theme attribute", async ({ page }) => {
    await gotoApp(page);
    await page.locator(".theme-picker button").click();
    await expect(page.locator(".theme-pop")).toBeVisible();
    await page.locator(".theme-opt", { hasText: "Dracula" }).click();
    const attr = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    expect(attr).toBe("dracula");
    await shot(page, "11-dracula");
  });

  test("theme picker keyboard navigation", async ({ page }) => {
    await gotoApp(page);
    await page.locator(".theme-picker button").click();
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
});
