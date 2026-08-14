import { test, expect } from "@playwright/test";
import { gotoApp, press, shot, trackErrors } from "./helpers";

test.describe("panels", () => {
  test("search panel opens, runs a query, and shows results", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await press(page, "s");
    await expect(page.locator(".search-panel")).toBeVisible();

    const input = page.locator(".search-panel input");
    await input.fill("watermark");
    await expect(page.locator(".search-panel .result-row").first()).toBeVisible({ timeout: 5000 });
    await shot(page, "08-search");

    // Opening an article result switches to the reader.
    await page.locator(".search-panel .result-row").first().click();
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 5000 });
    assertNoErrors();
  });

  test("graph panel opens with note and neighbors", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await press(page, "g");
    await press(page, "g"); // double-g within the window
    await expect(page.locator(".graph-panel")).toBeVisible();
    await expect(page.locator(".graph-current h2")).toContainText("DeepSeek Harness", { timeout: 5000 });
    await expect(page.locator(".graph-panel .result-row").first()).toBeVisible();
    await shot(page, "09-graph");
    await press(page, "Escape");
    await expect(page.locator(".graph-panel")).toHaveCount(0);
    assertNoErrors();
  });

  test("palette commands run (fetch shows toast)", async ({ page }) => {
    await gotoApp(page);
    await press(page, ":");
    const input = page.locator(".palette input");
    await input.fill("fetch");
    await page.keyboard.press("Enter");
    await expect(page.locator(".toast")).toBeVisible({ timeout: 5000 });
  });

  test("digest article click opens it in the reader", async ({ page }) => {
    await gotoApp(page);
    await press(page, "d");
    await expect(page.locator(".digest-view")).toBeVisible({ timeout: 5000 });
    await page.locator(".digest-articles li").first().click();
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 5000 });
    await shot(page, "10-digest-open");
  });
});
