import { test, expect } from "@playwright/test";
import { gotoApp, shot, trackErrors } from "./helpers";

test.describe("UI render", () => {
  test("three-pane layout renders with data and no console errors", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await shot(page, "01-layout");

    await expect(page.locator(".topbar .brand-name")).toHaveText("GizNews");
    await expect(page.locator(".topbar .pill").first()).toContainText("no leídos");

    await expect(page.locator(".sources-col .source-item")).toHaveCount(4);
    await expect(page.locator(".article-row")).toHaveCount(7); // 7 unread in mock

    await expect(page.locator(".reader-col")).toContainText("Selecciona un artículo");
    await expect(page.locator(".statusbar")).toBeVisible();

    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth
    );
    expect(overflow).toBeLessThanOrEqual(0);
    assertNoErrors();
  });

  test("article opens in reader with markdown rendered", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press("Enter");
    await expect(page.locator(".reader-head h1")).toBeVisible();
    await expect(page.locator(".markdown strong").first()).toBeVisible();
    await shot(page, "02-reader");
    const firstTitle = await page.locator(".article-row.selected .article-title").innerText();
    await expect(page.locator(".reader-head h1")).toHaveText(firstTitle);
  });

  test("reader shows AI summary when present", async ({ page }) => {
    await gotoApp(page);
    await page.keyboard.press("Enter");
    await expect(page.locator(".ai-summary")).toBeVisible();
  });

  test("column headers and category chips render", async ({ page }) => {
    await gotoApp(page);
    await expect(page.locator(".col-header")).toContainText("Título");
    await expect(page.locator(".cat-chip").first()).toBeVisible();
  });

  test("welcome overlay shows on first run and is dismissed", async ({ page }) => {
    await page.addInitScript(() => localStorage.removeItem("giznews-welcomed"));
    await page.goto("/");
    await expect(page.locator(".welcome")).toBeVisible({ timeout: 8000 });
    await expect(page.locator(".welcome")).toContainText("Bienvenido a GizNews");
    await page.locator(".welcome").getByRole("button", { name: "Empezar" }).click();
    await expect(page.locator(".welcome")).toHaveCount(0);
    await expect(page.locator(".article-row").first()).toBeVisible();
  });
});
