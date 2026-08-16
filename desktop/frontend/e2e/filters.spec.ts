import { test, expect } from "@playwright/test";
import { gotoApp, press } from "./helpers";

test.describe("classification filters", () => {
  test("category chips narrow the list", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".article-row")).toHaveCount(7); // unread
    await page.locator(".filter-chips .chip", { hasText: "models" }).click();
    await expect(page.locator(".article-row")).toHaveCount(2);
    await page.locator(".filter-chips .chip", { hasText: "All" }).click();
    await expect(page.locator(".article-row")).toHaveCount(7);
  });

  test("importance filter via [ ] cycles", async ({ page }) => {
    await gotoApp(page);
    await press(page, "]"); // ≥1★
    await expect(page.locator(".article-row")).toHaveCount(6);
    await press(page, "]"); // ≥2★
    await expect(page.locator(".article-row")).toHaveCount(5);
    await press(page, "]"); // ≥3★
    await expect(page.locator(".article-row")).toHaveCount(1);
    await press(page, "]"); // back to any
    await expect(page.locator(".article-row")).toHaveCount(7);
  });

  test("; opens the category picker", async ({ page }) => {
    await gotoApp(page);
    await press(page, ";");
    await expect(page.locator(".category-picker")).toBeVisible();
    await press(page, "Escape");
    await expect(page.locator(".category-picker")).toHaveCount(0);
  });

  test(":flow shows the pipeline with counts", async ({ page }) => {
    await gotoApp(page);
    await press(page, ":");
    await page.locator(".palette input").fill("flow");
    await page.keyboard.press("Enter");
    await expect(page.locator(".flow-panel")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".flow-node", { hasText: "Classify" })).toBeVisible();
    await expect(page.locator(".flow-node", { hasText: "Vault" })).toBeVisible();
    await press(page, "Escape");
    await expect(page.locator(".flow-panel")).toHaveCount(0);
  });

  test("digest is saved and appears in the history dropdown", async ({ page }) => {
    await gotoApp(page);
    await press(page, "d");
    await expect(page.locator(".digest-view")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".digest-head select option")).toHaveCount(2, { timeout: 6000 });
    await press(page, "Escape");
  });

  test(":logs opens the pipeline log", async ({ page }) => {
    await gotoApp(page);
    await press(page, ":");
    await page.locator(".palette input").fill("logs");
    await page.keyboard.press("Enter");
    await expect(page.locator(".logs-panel")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".logs-panel .logs-pre")).toContainText("classifying batch");
    await press(page, "Escape");
    await expect(page.locator(".logs-panel")).toHaveCount(0);
  });
});
