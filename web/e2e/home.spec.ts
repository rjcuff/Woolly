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
  await expect(nav.getByRole('link', { name: 'GitHub' })).toHaveAttribute('href', 'https://github.com/rjcuff/wooly');
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
  expect(image).toBe('https://usewooly.com/og/docs/image.png');

  const res = await request.get('/og/docs/image.png');
  expect(res.status()).toBe(200);
  expect(res.headers()['content-type']).toContain('image/png');
});

test('sitemap, robots and install script are served', async ({ request }) => {
  for (const path of ['/sitemap.xml', '/robots.txt', '/install.sh']) {
    expect((await request.get(path)).status(), path).toBe(200);
  }
  expect(await (await request.get('/sitemap.xml')).text()).toContain('https://usewooly.com/docs/quickstart');
});
