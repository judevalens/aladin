import { expect, test } from "@playwright/test";

test("login screen renders", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByText("Welcome back")).toBeVisible();
});
