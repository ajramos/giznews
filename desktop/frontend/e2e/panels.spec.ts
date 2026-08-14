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
    await expect(page.locator(".graph-canvas svg, svg.graph-canvas").first()).toBeVisible({ timeout: 4000 });
    await expect(page.locator(".graph-current h2")).toContainText("DeepSeek Harness");
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
});

test.describe("sources", () => {
  test("add source via modal", async ({ page }) => {
    await gotoApp(page);
    await page.locator(".sources-col").getByRole("button", { name: "Añadir" }).first().click();
    await expect(page.locator(".modal")).toBeVisible();
    await page.locator(".modal input").nth(0).fill("MIT Tech Review");
    await page.locator(".modal input").nth(1).fill("https://www.technologyreview.com/feed/");
    await page.locator(".modal").getByRole("button", { name: "Añadir" }).click();
    await expect(page.locator(".source-item", { hasText: "MIT Tech Review" })).toHaveCount(1);
    await shot(page, "10-source-added");
  });

  test("delete source removes it from the list", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".source-item")).toHaveCount(4);
    await page.locator(".source-item").first().hover();
    await page.locator(".source-item").first().getByTitle("Eliminar de la lista (los artículos se conservan)").click();
    await expect(page.locator(".modal")).toContainText("¿Eliminar");
    await page.locator(".modal").getByRole("button", { name: "Eliminar" }).click();
    await expect(page.locator(".source-item")).toHaveCount(3);
  });

  test("clicking a source filters the article list", async ({ page }) => {
    await gotoApp(page);
    await page.locator(".source-item", { hasText: "DeepMind Blog" }).locator(".source-name").click();
    await expect(page.locator(".article-row")).toHaveCount(1);
    await expect(page.locator(".topbar .pill.filter")).toBeVisible();
    await page.locator(".topbar .pill.filter").click();
    await expect(page.locator(".article-row")).toHaveCount(7);
  });
});

test.describe("themes", () => {
  test("theme switcher changes the data-theme attribute", async ({ page }) => {
    await gotoApp(page);
    await page.locator(".theme-select").selectOption("dracula");
    const attr = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    expect(attr).toBe("dracula");
    await shot(page, "11-dracula");
  });
});
