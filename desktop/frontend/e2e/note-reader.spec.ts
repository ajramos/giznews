import { test, expect } from "@playwright/test";
import { gotoApp, press, shot } from "./helpers";

// Reading a note: j/k/arrows scroll the note; L opens the links picker.
test.describe("note reader", () => {
  test("j/k scroll the note instead of changing it", async ({ page }) => {
    await gotoApp(page);
    // open the notes picker and open the atom note (has long content)
    await press(page, "n");
    await expect(page.locator(".notes-picker")).toBeVisible();
    await page.locator(".notes-picker .source-picker-item", { hasText: "DeepSeek Harness" }).click();
    await expect(page.locator(".reader .note-type")).toBeVisible({ timeout: 6000 });

    const title = await page.locator(".reader-head h1").innerText();
    const el = page.locator(".reader-scroll");
    await el.evaluate((node) => { node.scrollTop = 0; });

    await press(page, "j");
    await expect
      .poll(() => el.evaluate((node) => node.scrollTop))
      .toBeGreaterThan(0);

    // the note is still the same (didn't switch to another article/note)
    await expect(page.locator(".reader-head h1")).toHaveText(title);
    await shot(page, "15-note-scroll");
  });

  test("L opens the links picker in the note reader", async ({ page }) => {
    await gotoApp(page);
    await press(page, "n");
    await expect(page.locator(".notes-picker")).toBeVisible();
    await page.locator(".notes-picker .source-picker-item", { hasText: "DeepSeek Harness" }).click();
    await expect(page.locator(".reader .note-type")).toBeVisible({ timeout: 6000 });

    await press(page, "L");
    await expect(page.locator(".links-picker")).toBeVisible();
    await expect(page.locator(".links-picker .palette-item").first()).toBeVisible();
    await press(page, "Escape");
    await expect(page.locator(".links-picker")).toHaveCount(0);
  });
});
