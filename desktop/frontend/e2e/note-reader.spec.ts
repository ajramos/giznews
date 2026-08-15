import { test, expect } from "@playwright/test";
import { gotoApp, press, shot } from "./helpers";

// Reading a note in the knowledge world (vault): the article list disappears
// and the note renders in the detail (right) column.
test.describe("note reader", () => {
  test("opening a note from the vault hides the article list", async ({ page }) => {
    await gotoApp(page);
    await press(page, "n"); // vault → Atoms stage
    await expect(page.locator(".vault-browser")).toBeVisible();
    await expect(page.locator(".vault-tab.active")).toContainText("Atoms");
    await expect(page.locator(".vault-browser .palette-item").first()).toBeVisible();
    await press(page, "Enter"); // open the first atom (DeepSeek Harness)
    await expect(page.locator(".reader .note-type")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".article-row")).toHaveCount(0);
    await shot(page, "15-note-in-vault");
  });

  test("space scrolls the note", async ({ page }) => {
    await gotoApp(page);
    await press(page, "n");
    await expect(page.locator(".vault-browser .palette-item").first()).toBeVisible();
    await press(page, "Enter");
    await expect(page.locator(".reader .note-type")).toBeVisible({ timeout: 6000 });
    const el = page.locator(".reader-scroll");
    await el.evaluate((node) => { node.scrollTop = 0; });
    await press(page, " ");
    await expect
      .poll(() => el.evaluate((node) => node.scrollTop))
      .toBeGreaterThan(0);
  });

  test("L opens the links picker in the note reader", async ({ page }) => {
    await gotoApp(page);
    await press(page, "n");
    await expect(page.locator(".vault-browser .palette-item").first()).toBeVisible();
    await press(page, "Enter");
    await expect(page.locator(".reader .note-type")).toBeVisible({ timeout: 6000 });

    await press(page, "L");
    await expect(page.locator(".links-picker")).toBeVisible();
    await expect(page.locator(".links-picker .palette-item").first()).toBeVisible();
    await press(page, "Escape");
    await expect(page.locator(".links-picker")).toHaveCount(0);
  });

  test("L on an article shows its note connections", async ({ page }) => {
    await gotoApp(page);
    // article 1 auto-loads; it has a matching atom note in the mock
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
    await press(page, "L");
    await expect(page.locator(".links-picker")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".links-picker .palette-item").first()).toBeVisible();
    await press(page, "Escape");
    await expect(page.locator(".links-picker")).toHaveCount(0);
  });

  test("L on an unprocessed article shows its URL and embedded links", async ({ page }) => {
    await gotoApp(page);
    await press(page, "j"); // article 2 has no atom note in the mock
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
    await press(page, "L");
    await expect(page.locator(".links-picker")).toBeVisible({ timeout: 6000 });
    // the article body embeds a link to example.com in the mock
    await expect(page.locator(".links-picker")).toContainText("example.com");
    await press(page, "Escape");
    await expect(page.locator(".links-picker")).toHaveCount(0);
  });

  test("Enter focuses the article reader: j/k scroll, Esc returns to the list", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
    const first = await page.locator(".reader-head h1").innerText();

    await press(page, "Enter"); // open + focus the reader
    const el = page.locator(".reader-scroll");
    await el.evaluate((node) => { node.scrollTop = 0; });

    await press(page, "j");
    await expect
      .poll(() => el.evaluate((node) => node.scrollTop))
      .toBeGreaterThan(0);
    // still reading article 1 (j scrolled, it did not navigate)
    await expect(page.locator(".reader-head h1")).toHaveText(first);

    await press(page, "Escape"); // back to list focus
    await press(page, "j"); // now j navigates the list
    await expect(page.locator(".reader-head h1")).not.toHaveText(first);
  });
});
