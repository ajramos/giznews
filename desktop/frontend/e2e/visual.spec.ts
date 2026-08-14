import { test, expect } from "@playwright/test";
import { gotoApp, press, shot, trackErrors } from "./helpers";

test.describe("visual review", () => {
  test("capture gallery + no overflow in panes", async ({ page }) => {
    const assertNoErrors = trackErrors(page);
    await gotoApp(page);
    await shot(page, "01-layout");

    await press(page, "Enter");
    await shot(page, "02-reader");

    await press(page, "d");
    await expect(page.locator(".digest-view")).toBeVisible({ timeout: 6000 });
    await shot(page, "03-digest");
    await press(page, "d");

    await press(page, "s");
    await page.locator(".search-panel input").fill("agents");
    await expect(page.locator(".search-panel .result-row").first()).toBeVisible({ timeout: 6000 });
    await shot(page, "04-search");
    await press(page, "Escape");

    await press(page, "g");
    await expect(page.locator(".graph-panel")).toBeVisible({ timeout: 4000 });
    await shot(page, "05-graph");
    await press(page, "Escape");

    await press(page, ":");
    await shot(page, "06-palette");
    await press(page, "Escape");
    await press(page, "?");
    await shot(page, "07-help");
    await press(page, "Escape");

    const overflow = await page.evaluate(() => {
      const sels = [".article-list", ".reader-scroll", ".digest-view", ".search-panel", ".graph-panel", ".sources-col", ".statusbar"];
      const out: Record<string, number> = {};
      for (const sel of sels) {
        const el = document.querySelector(sel);
        if (el) out[sel] = el.scrollWidth - el.clientWidth;
      }
      return out;
    });
    for (const [sel, v] of Object.entries(overflow)) {
      expect(v, `${sel} overflows horizontally`).toBeLessThanOrEqual(0);
    }
    assertNoErrors();
  });
});
