import { test, expect } from "@playwright/test";
import { gotoApp, press, shot } from "./helpers";

// The vault flow: electrons → atoms → molecules (the news list is the inbox,
// so it is not a vault stage). It is a master-detail world: the vault is the
// left column and the note renders on the right, hiding the news article list.
test.describe("vault flow", () => {
  test("f opens the vault and shows the 3 stages", async ({ page }) => {
    await gotoApp(page);
    await press(page, "f");
    await expect(page.locator(".vault-browser")).toBeVisible();
    await expect(page.locator(".vault-tab")).toHaveCount(3);
    await expect(page.locator(".vault-tab").nth(0)).toContainText("Electrons");
    await expect(page.locator(".vault-tab").nth(1)).toContainText("Atoms");
    await expect(page.locator(".vault-tab").nth(2)).toContainText("Molecules");
    // the news article list is gone while in the knowledge world
    await expect(page.locator(".article-row")).toHaveCount(0);
    await shot(page, "13-vault");
    await press(page, "f"); // back to news
    await expect(page.locator(".vault-browser")).toHaveCount(0);
    await expect(page.locator(".article-row").first()).toBeVisible();
  });

  test("h/l switch stage, j/k navigate, Enter opens a note", async ({ page }) => {
    await gotoApp(page);
    await press(page, "f"); // opens at Atoms (default stage)
    await expect(page.locator(".vault-tab.active")).toContainText("Atoms");
    await expect(page.locator(".vault-browser .palette-item").first()).toBeVisible();

    await press(page, "h"); // electrons
    await expect(page.locator(".vault-tab.active")).toContainText("Electrons");
    await press(page, "l"); // atoms
    await press(page, "l"); // molecules
    await expect(page.locator(".vault-tab.active")).toContainText("Molecules");
    await press(page, "h"); // back to atoms
    await expect(page.locator(".vault-tab.active")).toContainText("Atoms");
    await expect(page.locator(".vault-browser .palette-item").first()).toBeVisible();

    await press(page, "Enter");
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".reader .note-type")).toBeVisible();
    // reading a note: articles are still hidden
    await expect(page.locator(".article-row")).toHaveCount(0);
  });

  test("L opens the links picker and follows a link", async ({ page }) => {
    await gotoApp(page);
    await press(page, "f"); // atoms
    await press(page, "l"); // molecules
    await expect(page.locator(".vault-tab.active")).toContainText("Molecules");
    await expect(page.locator(".vault-browser .palette-item").first()).toBeVisible();
    await press(page, "L");
    await expect(page.locator(".links-picker")).toBeVisible();
    await expect(page.locator(".links-picker .palette-item").first()).toBeVisible();
    await shot(page, "14-links-picker");
    // Enter follows the first link → vault navigates to it (an atom stage)
    await press(page, "Enter");
    await expect(page.locator(".links-picker")).toHaveCount(0);
    await expect(page.locator(".vault-tab.active")).toContainText("Atoms");
    await press(page, "f");
    await expect(page.locator(".vault-browser")).toHaveCount(0);
  });

  test("tags filter across stages (transversal taxonomy)", async ({ page }) => {
    await gotoApp(page);
    await press(page, "f"); // vault at Atoms
    await expect(page.locator(".vb-tags")).toBeVisible();
    // "ai" spans electron + atom + molecule in the mock (count 3)
    await expect(page.locator(".vb-tags .tag-chip", { hasText: "ai" })).toContainText("3");
    await page.locator(".vb-tags .tag-chip", { hasText: "ai" }).click();
    // transversal: it shows all 3 notes, not just the atoms stage
    await expect(page.locator(".vault-list .palette-item")).toHaveCount(3);
    await page.locator(".vb-tags .tag-chip", { hasText: "All" }).click();
    await expect(page.locator(".vault-list .palette-item")).toHaveCount(1); // atoms only
  });
});
