import { expect, test } from '@playwright/test';

test('home fits on one screen with the headline and button', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle('Woolly | Local memory for your AI tools');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Memory for your AI, kept on your laptop.');

  const scrolls = await page.evaluate(
    () =>
      document.documentElement.scrollHeight > window.innerHeight ||
      document.documentElement.scrollWidth > window.innerWidth,
  );
  expect(scrolls).toBe(false);

  await page.getByRole('link', { name: 'Get started' }).click();
  await expect(page).toHaveURL(/\/docs\/quickstart$/);
});

test('nav links and theme toggle are visible and work', async ({ page }) => {
  await page.goto('/');
  const nav = page.getByRole('navigation', { name: 'Main' });
  await expect(nav.getByRole('link', { name: 'GitHub' })).toHaveAttribute('href', 'https://github.com/rjcuff/woolly');
  await expect(nav.getByRole('link', { name: 'Docs' })).toBeVisible();

  const toggle = nav.getByRole('button', { name: /switch to (dark|light) mode/i });
  await expect(toggle).toBeInViewport();
  const before = await page.evaluate(() => document.documentElement.classList.contains('dark'));
  await toggle.click();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.classList.contains('dark')))
    .toBe(!before);
});

test('link previews point at the Woolly image', async ({ page, request }) => {
  await page.goto('/');
  const image = await page.locator('meta[property="og:image"]').getAttribute('content');
  expect(image).toBe('https://usewoolly.com/og/docs/image.png');

  const res = await request.get('/og/docs/image.png');
  expect(res.status()).toBe(200);
  expect(res.headers()['content-type']).toContain('image/png');
});

test('sitemap, robots and install script are served', async ({ request }) => {
  for (const path of ['/sitemap.xml', '/robots.txt', '/install.sh', '/install.ps1', '/og/docs/profile.png']) {
    expect((await request.get(path)).status(), path).toBe(200);
  }
  expect(await (await request.get('/sitemap.xml')).text()).toContain('https://usewoolly.com/docs/quickstart');
});

test('integration logos name their app on hover', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByText('Integrated with')).toBeVisible();
  await page.getByRole('link', { name: 'Ollama' }).hover();
  await expect(page.getByRole('tooltip', { name: 'Ollama' })).toBeVisible();
});

test('download page offers the Mac app and install commands', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('link', { name: 'Download', exact: true }).click();
  await expect(page).toHaveURL(/\/download$/);
  await page.getByRole('tab', { name: 'macOS' }).click();
  await expect(page.getByRole('link', { name: 'Download for Mac' })).toHaveAttribute('href', /Woolly-mac\.dmg$/);
  await page.getByRole('tab', { name: 'Windows' }).click();
  await expect(page.getByText('irm https://usewoolly.com/install.ps1 | iex')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)).toBe(false);
});
