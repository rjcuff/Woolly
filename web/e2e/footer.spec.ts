import { expect, test } from '@playwright/test';

test('footer shows the trademark and links on one screen', async ({ page }) => {
  await page.goto('/');
  const footer = page.getByRole('contentinfo');
  await expect(footer).toBeInViewport();
  await expect(footer).toContainText(`© ${new Date().getFullYear()} Woolly™`);

  const links = footer.getByRole('navigation', { name: 'Footer' });
  await expect(links.getByRole('link', { name: 'GitHub' })).toHaveAttribute('href', 'https://github.com/rjcuff/wooly');

  await links.getByRole('link', { name: 'Privacy' }).click();
  await expect(page).toHaveURL(/\/privacy$/);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Privacy');
  await expect(page).toHaveTitle('Privacy | Woolly');
});

test('terms page loads and links back to privacy', async ({ page }) => {
  await page.goto('/terms');
  await expect(page).toHaveTitle('Terms | Woolly');
  await expect(page.getByRole('heading', { name: 'Trademark' })).toBeVisible();
  await page.getByRole('main').getByRole('link', { name: 'privacy policy' }).click();
  await expect(page).toHaveURL(/\/privacy$/);
});

test('legal pages are in the sitemap', async ({ request }) => {
  const xml = await (await request.get('/sitemap.xml')).text();
  expect(xml).toContain('https://usewooly.com/privacy');
  expect(xml).toContain('https://usewooly.com/terms');
});
