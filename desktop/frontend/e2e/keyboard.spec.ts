import { test, expect } from "@playwright/test";
import { gotoApp, press, shot, trackErrors } from "./helpers";

test.describe("keyboard", () => {
  test("j/k moves selection", async ({ page }) => {
    await gotoApp(page);
    const rows = page.locator(".article-row");
    await expect(rows.nth(0)).toHaveClass(/selected/);

    await press(page, "j");
    await expect(rows.nth(1)).toHaveClass(/selected/);
    await press(page, "j");
    await expect(rows.nth(2)).toHaveClass(/selected/);
    await press(page, "k");
    await expect(rows.nth(1)).toHaveClass(/selected/);
    await shot(page, "04-nav");
  });

  test("g goes to top, G goes to bottom", async ({ page }) => {
    await gotoApp(page);
    await press(page, "G");
    await expect(page.locator(".article-row").last()).toHaveClass(/selected/);
    await press(page, "g");
    await expect(page.locator(".article-row").first()).toHaveClass(/selected/);
  });

  test("Enter opens article and marks it read", async ({ page }) => {
    await gotoApp(page);
    await press(page, "Enter");
    await expect(page.locator(".reader-head h1")).toBeVisible();
    // unread-dot removed from the selected row after opening.
    await expect(page.locator(".article-row.selected .unread-dot")).toHaveCount(0);
  });

  test("y summarizes selected article", async ({ page }) => {
    await gotoApp(page);
    await press(page, "y");
    await expect(page.locator(".reader .ai-summary")).toBeVisible({ timeout: 5000 });
    await shot(page, "05-summarize");
  });

  test("a archives selected article (row disappears)", async ({ page }) => {
    await gotoApp(page);
    const before = await page.locator(".article-row").count();
    await press(page, "a");
    await expect(page.locator(".article-row")).toHaveCount(before - 1);
  });

  test("t toggles read/unread", async ({ page }) => {
    await gotoApp(page);
    const dotBefore = await page.locator(".article-row.selected .unread-dot").count();
    await press(page, "t");
    await expect(page.locator(".article-row.selected .unread-dot")).toHaveCount(dotBefore === 0 ? 1 : 0);
  });

  test("d opens digest view", async ({ page }) => {
    await gotoApp(page);
    await press(page, "d");
    await expect(page.locator(".digest-view")).toBeVisible({ timeout: 5000 });
    await expect(page.locator(".digest-overview")).toContainText("DeepSeek");
    await shot(page, "06-digest");
    // back to articles with 1
    await press(page, "1");
    await expect(page.locator(".article-list")).toBeVisible();
  });

  test("? opens help, Esc closes", async ({ page }) => {
    await gotoApp(page);
    await press(page, "?");
    await expect(page.locator(".help-list")).toBeVisible();
    await press(page, "Escape");
    await expect(page.locator(".help-list")).toHaveCount(0);
  });

  test(": opens command palette", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await press(page, ":");
    await expect(page.locator(".palette")).toBeVisible();
    await expect(page.locator(".palette-item").first()).toBeVisible();
    await shot(page, "07-palette");
    await press(page, "Escape");
    await expect(page.locator(".palette")).toHaveCount(0);
    assertNoErrors();
  });
});
