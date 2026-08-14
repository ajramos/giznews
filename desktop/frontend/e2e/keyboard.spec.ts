import { test, expect } from "@playwright/test";
import { gotoApp, press, shot, trackErrors } from "./helpers";

test.describe("vim keyboard grammar", () => {
  test("j/k moves selection", async ({ page }) => {
    await gotoApp(page);
    const rows = page.locator(".article-row");
    await expect(rows.nth(0)).toHaveClass(/selected/);
    await press(page, "j");
    await expect(rows.nth(1)).toHaveClass(/selected/);
    await press(page, "k");
    await expect(rows.nth(0)).toHaveClass(/selected/);
  });

  test("count prefix: 5j moves 5", async ({ page }) => {
    await gotoApp(page);
    await press(page, "5");
    await press(page, "j");
    await expect(page.locator(".article-row").nth(5)).toHaveClass(/selected/);
  });

  test("gg goes to top, G goes to bottom", async ({ page }) => {
    await gotoApp(page);
    await press(page, "G");
    await expect(page.locator(".article-row").last()).toHaveClass(/selected/);
    await press(page, "g");
    await press(page, "g"); // gg within window
    await expect(page.locator(".article-row").first()).toHaveClass(/selected/);
  });

  test("Enter opens article and marks it read", async ({ page }) => {
    await gotoApp(page);
    await press(page, "Enter");
    await expect(page.locator(".reader-head h1")).toBeVisible();
    await expect(page.locator(".article-row.selected .unread-badge")).toHaveCount(0);
  });

  test("y summarizes selected article", async ({ page }) => {
    await gotoApp(page);
    await press(page, "y");
    await expect(page.locator(".reader .ai-summary")).toBeVisible({ timeout: 6000 });
  });

  test("a archives with undo; undo restores the article", async ({ page }) => {
    await gotoApp(page);
    const before = await page.locator(".article-row").count();
    await press(page, "a");
    await expect(page.locator(".article-row")).toHaveCount(before - 1);
    await expect(page.locator(".toast")).toContainText("archivado");
    await shot(page, "04-archive-undo");
    // Undo restores the archived article (it was read by auto-load, so it
    // returns to the read view, not the unread list).
    await page.locator(".toast .toast-undo").click();
    await press(page, "x"); // archived view
    await expect(page.locator(".article-row")).toHaveCount(0);
    await press(page, "r"); // read view: mock's pre-read article + the restored one
    await expect(page.locator(".article-row")).toHaveCount(2);
  });

  test("count prefix: 3a archives 3", async ({ page }) => {
    await gotoApp(page);
    const before = await page.locator(".article-row").count();
    await press(page, "3");
    await press(page, "a");
    await expect(page.locator(".article-row")).toHaveCount(before - 3);
  });

  test("t marks read (leaves unread view); t in read view restores", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".article-row")).toHaveCount(7);
    await press(page, "t");
    // selected article becomes read → leaves the unread view
    await expect(page.locator(".article-row")).toHaveCount(6);
    // read view now has 2 (the mock's read one + the one just read)
    await press(page, "r");
    await expect(page.locator(".article-row")).toHaveCount(2);
    await press(page, "t");
    await expect(page.locator(".article-row")).toHaveCount(1);
  });

  test("m stars the article", async ({ page }) => {
    await gotoApp(page);
    await press(page, "m");
    await expect(page.locator(".article-row.selected .star-badge")).toBeVisible();
  });

  test("views: r shows read, u unread, x archived, * starred", async ({ page }) => {
    await gotoApp(page);
    // mock: article 7 is read
    await press(page, "r");
    await expect(page.locator(".view-tab.active")).toContainText("Leídos");
    await expect(page.locator(".article-row")).toHaveCount(1);

    await press(page, "u");
    await expect(page.locator(".view-tab.active")).toContainText("No leídos");
    await expect(page.locator(".article-row")).toHaveCount(7);

    // archive one then view archived
    await press(page, "a");
    await press(page, "x");
    await expect(page.locator(".view-tab.active")).toContainText("Archivados");
    await expect(page.locator(".article-row")).toHaveCount(1);

    // star one and view starred
    await press(page, "u");
    await press(page, "m");
    await press(page, "*");
    await expect(page.locator(".view-tab.active")).toContainText("Destacados");
    await expect(page.locator(".article-row")).toHaveCount(1);
  });

  test("d opens digest (generates), Esc returns to list", async ({ page }) => {
    await gotoApp(page);
    await press(page, "d");
    await expect(page.locator(".digest-view")).toBeVisible({ timeout: 6000 });
    await shot(page, "05-digest");
    await press(page, "Escape");
    await expect(page.locator(".article-list")).toBeVisible();
  });

  test("J/K open next/previous article", async ({ page }) => {
    await gotoApp(page);
    await press(page, "Enter");
    const first = await page.locator(".reader-head h1").innerText();
    await press(page, "J");
    await expect(page.locator(".reader-head h1")).not.toHaveText(first, { timeout: 4000 });
    const second = await page.locator(".reader-head h1").innerText();
    await press(page, "K");
    await expect(page.locator(".reader-head h1")).toHaveText(first, { timeout: 4000 });
  });

  test("space scrolls the reader", async ({ page }) => {
    await gotoApp(page);
    await press(page, "Enter");
    const el = page.locator(".reader-scroll");
    await el.evaluate((node) => { node.scrollTop = 0; });
    await press(page, " ");
    const top = await el.evaluate((node) => node.scrollTop);
    expect(top).toBeGreaterThan(0);
    // Shift+space scrolls back up
    await press(page, "Shift+Space");
    const top2 = await el.evaluate((node) => node.scrollTop);
    expect(top2).toBeLessThan(top);
  });

  test("v bulk: space toggles selection, Esc exits", async ({ page }) => {
    await gotoApp(page);
    await press(page, "v");
    await expect(page.locator(".statusbar .mode.bulk")).toContainText("BULK");
    // current item pre-selected
    await expect(page.locator(".article-row.bulk")).toHaveCount(1);
    await press(page, " ");
    await expect(page.locator(".article-row.bulk")).toHaveCount(0);
    await press(page, " ");
    await expect(page.locator(".article-row.bulk")).toHaveCount(1);
    await press(page, "Escape");
    await expect(page.locator(".article-row.bulk")).toHaveCount(0);
  });

  test("bulk: space to select multiple, a archives them", async ({ page }) => {
    await gotoApp(page);
    const before = await page.locator(".article-row").count();
    await press(page, "v");       // select first
    await press(page, "j");       // move to second (selection unchanged)
    await press(page, " ");       // select second
    await expect(page.locator(".article-row.bulk")).toHaveCount(2);
    await press(page, "a");       // archive both
    await expect(page.locator(".article-row")).toHaveCount(before - 2);
    await expect(page.locator(".statusbar .mode.bulk")).toHaveCount(0);
  });

  test("? opens help, Esc closes", async ({ page }) => {
    await gotoApp(page);
    await press(page, "?");
    await expect(page.locator(".help-list")).toBeVisible();
    await expect(page.locator(".help-note")).toContainText("Archivar es lógico");
    await press(page, "Escape");
    await expect(page.locator(".help-list")).toHaveCount(0);
  });

  test(": opens command palette", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await press(page, ":");
    await expect(page.locator(".palette")).toBeVisible();
    await expect(page.locator(".palette-item").first()).toBeVisible();
    await shot(page, "06-palette");
    await press(page, "Escape");
    await expect(page.locator(".palette")).toHaveCount(0);
    assertNoErrors();
  });
});
