import { env } from "node:process";
import { expect, test } from "@playwright/test";

test.describe("bundled local frontend", () => {
  test.skip(!env.ALADIN_BUNDLED_FRONTEND_URL, "Requires a running packaged Aladin app");

  test("serves the compiled app and rejects foreign origins", async ({ request }) => {
    const response = await request.get("/");
    expect(response.status()).toBe(200);
    expect(response.headers()["server"]).toContain("tiny-http");
    expect(response.headers()["x-frame-options"]).toBe("DENY");
    const html = await response.text();
    expect(html).toContain("/assets/");
    expect(html).not.toContain("@vite/client");
    const foreign = await request.get("/", { headers: { Origin: "https://untrusted.example" } });
    expect(foreign.status()).toBe(403);
  });

  test("board stays rendered after the license-validation window", async ({ page }) => {
    // Native-shell APIs are unavailable here; this smoke test covers the served board.
    await page.goto("/spike/board");
    await expect(page.locator(".tl-container")).toBeVisible();
    // A bundled custom-protocol build previously hid the editor after five seconds.
    await page.waitForTimeout(6500);
    await expect(page.locator(".tl-container")).toBeVisible();
    await expect(page.getByTestId("tl-license-expired")).toHaveCount(0);
    await page.screenshot({ path: "test-results/local-frontend-board.png" });
  });
});
