import { test, expect } from "@playwright/test";
import { gotoApp, shot, trackErrors } from "./helpers";

test.describe("UI render", () => {
  test("three-pane layout renders with data and no console errors", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await shot(page, "01-layout");

    // Topbar: brand + status pills.
    await expect(page.locator(".topbar .brand")).toHaveText("GizNews");
    await expect(page.locator(".topbar .pill").first()).toBeVisible();

    // Sources column.
    const sources = page.locator(".sources-col .source-item");
    await expect(sources.first()).toBeVisible();
    await expect(sources).toHaveCount(4);

    // Article list.
    const rows = page.locator(".article-row");
    await expect(rows.first()).toBeVisible();
    await expect(rows).toHaveCount(7); // 7 unread in mock

    // Reader hint before selection.
    await expect(page.locator(".reader-col")).toContainText("Selecciona un artículo");

    // Layout does not overflow horizontally.
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);

    assertNoErrors();
  });

  test("article opens in reader with markdown rendered", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);

    await page.keyboard.press("Enter");
    await expect(page.locator(".reader-head h1").first()).toBeVisible();
    await expect(page.locator(".markdown strong").first()).toBeVisible();
    await shot(page, "02-reader");

    // Title matches the selected article.
    const firstTitle = await page.locator(".article-row.selected .article-title").innerText();
    await expect(page.locator(".reader-head h1").first()).toHaveText(firstTitle);

    assertNoErrors();
  });

  test("reader shows AI summary when present", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press("Enter");
    // Article id 1 has a summary in the mock.
    await expect(page.locator(".ai-summary")).toBeVisible();
    await shot(page, "03-reader-summary");
  });

  test("status pills reflect mock counts", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".topbar .pill").first()).toContainText("7 no leídos");
    await expect(page.locator(".topbar .pill").last()).toContainText("3 notas");
  });
});
