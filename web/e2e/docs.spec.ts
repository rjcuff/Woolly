import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.goto('/docs/quickstart');
});

test('page has its own title and description', async ({ page }) => {
  await expect(page).toHaveTitle('Quickstart | Woolly Docs');
  await expect(page.locator('meta[name="description"]')).toHaveAttribute(
    'content',
    'Install Woolly, save a memory, and get it back.',
  );
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute('href', 'https://usewooly.com/docs/quickstart');
});

test('search finds pages', async ({ page, isMobile }) => {
  if (isMobile) {
    await page.getByRole('button', { name: 'Open Search' }).first().click();
  } else {
    await page.keyboard.press('ControlOrMeta+k');
  }
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await page.keyboard.type('ollama');
  await expect(dialog.getByText('Using Ollama').first()).toBeVisible();
});

test('copy markdown gives feedback', async ({ page, context, browserName }) => {
  test.skip(browserName !== 'chromium', 'clipboard permissions are Chromium only');
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.getByRole('button', { name: 'Copy Markdown' }).click();
  await expect(page.getByRole('button', { name: 'Copied Markdown' })).toBeVisible();
  const copied = await page.evaluate(() => navigator.clipboard.readText());
  expect(copied).toContain('# Quickstart');
});

test('open menu lists the page actions', async ({ page }) => {
  await page.getByRole('button', { name: 'Open', exact: true }).click();
  await expect(page.getByRole('link', { name: /Open in Claude/ })).toBeVisible();
  await expect(page.getByRole('link', { name: /View as Markdown/ })).toBeVisible();
});

test('markdown copy of the page is served', async ({ request }) => {
  const res = await request.get('/llms.mdx/docs/quickstart/content.md');
  expect(res.status()).toBe(200);
  expect(await res.text()).toContain('Quickstart');
});

test('sidebar navigates between pages', async ({ page, isMobile }) => {
  if (isMobile) {
    await page.getByRole('button', { name: /open sidebar|toggle sidebar/i }).first().click();
  }
  await page.getByRole('link', { name: 'Import your history' }).click();
  await expect(page).toHaveURL(/\/docs\/import$/);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Import your history');
});

test('no horizontal scrolling', async ({ page }) => {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
  expect(overflow).toBe(false);
});

test('unknown pages return 404', async ({ request }) => {
  expect((await request.get('/docs/does-not-exist')).status()).toBe(404);
});
