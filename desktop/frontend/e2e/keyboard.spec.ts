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
    await press(page, "a"); // aa = archive 1
    await expect(page.locator(".article-row")).toHaveCount(before - 1);
    await expect(page.locator(".toast")).toContainText("archived");
    await shot(page, "04-archive-undo");
    await page.locator(".toast .toast-undo").click();
    // the restored article reappears in Active (back to its original status)
    await expect(page.locator(".article-row", { hasText: "DeepSeek Harness" })).toHaveCount(1);
    await expect(page.locator(".article-row")).toHaveCount(before);
    // and it is no longer archived
    await press(page, "x");
    await expect(page.locator(".article-row")).toHaveCount(0);
  });

  test("a3a archives 3", async ({ page }) => {
    await gotoApp(page);
    const before = await page.locator(".article-row").count();
    await press(page, "a");
    await press(page, "3");
    await press(page, "a");
    await expect(page.locator(".article-row")).toHaveCount(before - 3);
  });

  test("t marks read (unread-badge disappears, article stays in Active)", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".article-row")).toHaveCount(8);
    // the mock's single pre-read article (id 7) has no unread badge; the rest do
    await expect(page.locator(".unread-badge")).toHaveCount(7);
    await press(page, "t");
    await press(page, "t"); // tt = toggle read 1
    await expect(page.locator(".article-row")).toHaveCount(8); // stays in Active
    await expect(page.locator(".unread-badge")).toHaveCount(6);
    await press(page, "t");
    await press(page, "t");
    await expect(page.locator(".unread-badge")).toHaveCount(7);
  });

  test("m stars the article (independent of status)", async ({ page }) => {
    await gotoApp(page);
    await press(page, "m");
    await press(page, "m"); // mm = star 1
    await expect(page.locator(".article-row")).toHaveCount(8); // still in Active
    await expect(page.locator(".article-row .star-badge")).toHaveCount(1);
    await press(page, "*"); // starred view
    await expect(page.locator(".article-row")).toHaveCount(1);
    await expect(page.locator(".article-row .star-badge")).toBeVisible();
  });

  test("views: Active shows all, u/r filter unread/read, x archived, * starred", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".view-tab.active")).toContainText("Active");
    await expect(page.locator(".article-row")).toHaveCount(8); // unread 7 + read 1

    // u → unread only (Active tab stays highlighted)
    await press(page, "u");
    await expect(page.locator(".view-tab.active")).toContainText("Active");
    await expect(page.locator(".read-filter button.active")).toHaveText("Unread");
    await expect(page.locator(".article-row")).toHaveCount(7);

    // r → read only
    await press(page, "r");
    await expect(page.locator(".read-filter button.active")).toHaveText("Read");
    await expect(page.locator(".article-row")).toHaveCount(1);

    // clicking the Active tab returns to all unread + read
    await page.locator(".view-tab", { hasText: "Active" }).click();
    await expect(page.locator(".read-filter button.active")).toHaveText("All");
    await expect(page.locator(".article-row")).toHaveCount(8);

    // archive one then view archived
    await press(page, "a");
    await press(page, "a");
    await press(page, "x");
    await expect(page.locator(".view-tab.active")).toContainText("Archived");
    await expect(page.locator(".article-row")).toHaveCount(1);

    // star one and view starred (starred stays in Active too)
    await press(page, "u");
    await expect(page.locator(".article-row")).toHaveCount(6); // unread only, archived one gone
    await press(page, "m");
    await press(page, "m");
    await expect(page.locator(".article-row .star-badge")).toHaveCount(1); // mm settled
    await press(page, "*");
    await expect(page.locator(".view-tab.active")).toContainText("Starred");
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

  test("bulk: clicking a row toggles its selection (checkbox)", async ({ page }) => {
    await gotoApp(page);
    await press(page, "v"); // enters bulk, selects current (row 0)
    await expect(page.locator(".article-row.bulk")).toHaveCount(1);
    // click row 2 → toggles it on
    await page.locator(".article-row").nth(2).click();
    await expect(page.locator(".article-row.bulk")).toHaveCount(2);
    // click row 0 → toggles it off
    await page.locator(".article-row").nth(0).click();
    await expect(page.locator(".article-row.bulk")).toHaveCount(1);
    await press(page, "Escape");
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

  test("bulk: c classifies the selection as a background job", async ({ page }) => {
    await gotoApp(page);
    await press(page, "v");       // select first
    await press(page, "j");
    await press(page, " ");       // select second
    await expect(page.locator(".article-row.bulk")).toHaveCount(2);
    await press(page, "c");       // classify selected
    await expect(page.locator(".statusbar .mode.bulk")).toHaveCount(0);
    await expect(page.locator(".jobs-picker")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".job-item", { hasText: "Classify 2 selected" })).toHaveCount(1, { timeout: 6000 });
    await page.keyboard.press("Escape");
  });

  test("? opens help, Esc closes", async ({ page }) => {
    await gotoApp(page);
    await press(page, "?");
    await expect(page.locator(".help-list")).toBeVisible();
    await expect(page.locator(".help-note")).toContainText("Archiving is logical");
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

test.describe("knowledge lifecycle actions", () => {
  test("c classifies the current article as a background job", async ({ page }) => {
    await gotoApp(page);
    await press(page, "c");
    await expect(page.locator(".jobs-picker")).toBeVisible({ timeout: 6000 });
    await expect(page.locator(".job-item", { hasText: "Classify 1 selected" })).toHaveCount(1, { timeout: 6000 });
    await press(page, "Escape");
  });

  test("p materializes the current article into the KB (context shows the note)", async ({ page }) => {
    await gotoApp(page);
    await press(page, "Enter"); // open the article so the reader has it
    await press(page, "p");
    await expect(page.locator(".toast")).toContainText("Note created", { timeout: 6000 });
    await press(page, "C"); // open the context pane
    await expect(page.locator(".ctx-pane")).toBeVisible();
    await expect(page.locator(".ctx-pane")).not.toContainText("Create note");
  });

  test("bulk: p materializes notes only for the selection", async ({ page }) => {
    await gotoApp(page);
    await press(page, "v");       // enter bulk, selects first
    await press(page, "j");       // move to second
    await press(page, " ");       // select second
    await expect(page.locator(".article-row.bulk")).toHaveCount(2);
    await press(page, "p");
    await expect(page.locator(".toast")).toContainText("2 note(s) created", { timeout: 6000 });
  });

  test("C toggles the context pane", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".context-tab")).toBeVisible();
    await press(page, "C");
    await expect(page.locator(".ctx-pane")).toBeVisible();
    await press(page, "C");
    await expect(page.locator(".ctx-pane")).toHaveCount(0);
    await expect(page.locator(".context-tab")).toBeVisible();
  });
});
