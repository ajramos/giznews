import { test, expect } from "@playwright/test";
import { gotoApp, shot } from "./helpers";

// The concepts picker is the knowledge graph seen from its concepts: which ones
// recur, which are still waiting for a note, and folding two spellings into one.
async function openConcepts(page: import("@playwright/test").Page) {
  await gotoApp(page);
  await page.keyboard.press(":");
  await page.locator(".palette input").fill("concepts");
  await page.keyboard.press("Enter");
  await expect(page.locator(".source-picker")).toBeVisible();
  // The list loads asynchronously; keys only act on a row once there is one.
  await expect(page.locator('[data-testid="concept-row"]').first()).toBeVisible();
}

test.describe("concepts", () => {
  test(":concepts lists them with their state and mention count", async ({ page }) => {
    await openConcepts(page);
    const rows = page.locator('[data-testid="concept-row"]');
    await expect(rows).toHaveCount(5);
    // Promoted concepts are marked; pending ones are not.
    await expect(rows.first().locator(".sp-dot")).toHaveAttribute("data-on", "true");
    await expect(rows.nth(2).locator(".sp-dot")).toHaveAttribute("data-on", "false");
    await expect(rows.first()).toContainText("OpenAI");
    await expect(rows.first()).toContainText("7×");
    await expect(page.locator(".palette-head").first()).toContainText("3 waiting for a note");
    await shot(page, "20-concepts");
    await page.keyboard.press("Escape");
    await expect(page.locator(".source-picker")).toHaveCount(0);
  });

  test("/ filters the list", async ({ page }) => {
    await openConcepts(page);
    await page.keyboard.press("/");
    await page.keyboard.type("attention");
    const rows = page.locator('[data-testid="concept-row"]');
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toContainText("Sparse Attention");
  });

  test("Enter gives a pending concept its note", async ({ page }) => {
    await openConcepts(page);
    // Third row is the first pending one (watermarking).
    await page.keyboard.press("j");
    await page.keyboard.press("j");
    await page.keyboard.press("Enter");
    // The picker closes and the note opens in the knowledge world.
    await expect(page.locator(".source-picker")).toHaveCount(0);
    await expect(page.locator(".reader .note-type")).toHaveText("electron", { timeout: 6000 });
    await expect(page.locator(".reader-head h1")).toHaveText("Watermarking");
  });

  test("m folds one concept into another", async ({ page }) => {
    await openConcepts(page);
    const rows = page.locator('[data-testid="concept-row"]');
    await expect(rows).toHaveCount(5);

    // Mark "Open AI" (last row) and fold it into "OpenAI" (first).
    for (let i = 0; i < 4; i++) await page.keyboard.press("j");
    await page.keyboard.press("m");
    await expect(page.locator('[data-testid="merge-hint"]')).toContainText("Open AI");

    for (let i = 0; i < 4; i++) await page.keyboard.press("k");
    await page.keyboard.press("m");

    await expect(rows).toHaveCount(4);
    await expect(page.locator(".toast")).toContainText("folded into");
    await expect(rows.first()).toContainText("8×");
  });

  test("Esc cancels a pending merge before closing", async ({ page }) => {
    await openConcepts(page);
    await page.keyboard.press("m");
    await expect(page.locator('[data-testid="merge-hint"]')).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.locator('[data-testid="merge-hint"]')).toHaveCount(0);
    await expect(page.locator(".source-picker")).toBeVisible();
  });
});
