import { Page, expect } from "@playwright/test";

// Collect console errors/page errors; call expectNoErrors() at the end.
export function trackErrors(page: Page): () => void {
  const errors: string[] = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") errors.push(msg.text());
  });
  page.on("pageerror", (err) => errors.push(String(err)));
  return () => {
    expect(errors, `console/page errors:\n${errors.join("\n")}`).toEqual([]);
  };
}

// Named screenshot saved to screenshots/<name>.png (stable dir for review).
export async function shot(page: Page, name: string): Promise<void> {
  await page.waitForTimeout(150); // let animations/settled state flush
  await page.screenshot({ path: `screenshots/${name}.png`, fullPage: false });
}

export async function gotoApp(page: Page): Promise<void> {
  // First-run welcome overlay is skipped in tests (a dedicated test covers it).
  await page.addInitScript(() => localStorage.setItem("giznews-welcomed", "1"));
  await page.goto("/");
  await expect(page.locator(".article-row").first()).toBeVisible({ timeout: 8000 });
}

export async function press(page: Page, key: string): Promise<void> {
  await page.keyboard.press(key);
}
