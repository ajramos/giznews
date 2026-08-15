import { test, expect } from "@playwright/test";
import { gotoApp, press, shot } from "./helpers";

// The vault flow: inbox (pending articles) → electrons → atoms → molecules.
test.describe("vault flow", () => {
  test("opens with f and shows the 4 stages with counts", async ({ page }) => {
    await gotoApp(page);
    await press(page, "f");
    await expect(page.locator(".vault-panel")).toBeVisible();
    await expect(page.locator(".vault-tab")).toHaveCount(4);
    await expect(page.locator(".vault-tab").nth(0)).toContainText("Inbox");
    await expect(page.locator(".vault-tab").nth(1)).toContainText("Electrons");
    // inbox stage lists pending articles (mock articles without a note)
    await expect(page.locator(".vault-list .palette-item").first()).toBeVisible();
    await shot(page, "13-vault");
    await press(page, "Escape");
    await expect(page.locator(".vault-panel")).toHaveCount(0);
  });

  test("h/l switch stage, j/k navigate, Enter opens a note", async ({ page }) => {
    await gotoApp(page);
    await press(page, "f");
    await expect(page.locator(".vault-panel")).toBeVisible();

    // move to atoms stage (inbox → electrons → atoms)
    await press(page, "l"); // electrons
    await press(page, "l"); // atoms
    await expect(page.locator(".vault-tab.active")).toContainText("Atoms");
    await expect(page.locator(".vault-list .palette-item").first()).toBeVisible();

    // open the first atom
    await press(page, "Enter");
    await expect(page.locator(".reader-head h1")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".reader .note-type")).toBeVisible();
  });

  test("L opens the links picker and follows a link", async ({ page }) => {
    await gotoApp(page);
    await press(page, "f");
    // go to molecules stage (which links to an atom in the mock)
    await press(page, "l"); await press(page, "l"); await press(page, "l"); // molecules
    await expect(page.locator(".vault-tab.active")).toContainText("Molecules");
    await expect(page.locator(".vault-list .palette-item").first()).toBeVisible();
    await press(page, "L");
    await expect(page.locator(".links-picker")).toBeVisible();
    await expect(page.locator(".links-picker .palette-item").first()).toBeVisible();
    await shot(page, "14-links-picker");
    // Enter follows the first link → vault navigates to it (an atom stage)
    await press(page, "Enter");
    await expect(page.locator(".links-picker")).toHaveCount(0);
    await expect(page.locator(".vault-tab.active")).toContainText("Atoms");
    await press(page, "Escape");
    await expect(page.locator(".vault-panel")).toHaveCount(0);
  });
});
