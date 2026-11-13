import { expect, test } from '@playwright/test';

const unique = () => Math.random().toString(36).slice(2, 8);

test.beforeEach(async ({ page }) => {
  await page.goto(`/auth?token=${process.env.WOOLLY_TEST_TOKEN}&view=app`);
});

async function openTab(page: import('@playwright/test').Page, isMobile: boolean, name: string) {
  if (isMobile) await page.getByRole('button', { name: 'Open sidebar' }).click();
  await page.getByRole('navigation', { name: 'Sections' }).getByRole('button', { name, exact: true }).click();
}

test('home shows memories, apps and recent saves', async ({ page }) => {
  const text = `Grows tomatoes on the balcony ${unique()}`;
  await page.request.post('/api/remember', { data: { content: text }, headers: { 'X-Woolly-Client': 'cursor' } });
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Home' })).toBeVisible();
  await expect(page.getByText('This week')).toBeVisible();
  const recent = page.locator('.list li').filter({ hasText: text });
  await expect(recent).toContainText('Cursor');
  await expect(page.getByLabel(/Memories saved per day/)).toBeVisible();
});

test('apps lists what Woolly can connect', async ({ page, isMobile }) => {
  await openTab(page, isMobile, 'Apps');
  await expect(page.getByRole('heading', { name: 'Apps' })).toBeVisible();
  await expect(page.getByText('Claude Code', { exact: true })).toBeVisible();
});

test('settings sets up sync and shows a join command', async ({ page, isMobile }) => {
  await openTab(page, isMobile, 'Settings');
  await expect(page.getByText('Not running')).toBeVisible();
  await page.getByRole('button', { name: /Set up sync|Add a device/ }).click();
  await expect(page.locator('.set-code code')).toContainText('woolly sync join');
  await page.getByRole('button', { name: 'Stop syncing' }).click();
  await expect(page.getByRole('button', { name: 'Set up sync' })).toBeVisible();
});

test('settings opens trash, history, conflicts and activity in the app', async ({ page, isMobile }) => {
  const text = `Old note to trash ${unique()}`;
  const saved = await page.request.post('/api/remember', { data: { content: text } });
  const { memory } = await saved.json();
  await page.request.delete(`/api/memories/${memory.id}`);
  await openTab(page, isMobile, 'Settings');
  await page.getByRole('button', { name: 'Trash' }).click();
  await expect(page.getByRole('heading', { name: 'Trash' })).toBeVisible();
  const card = page.locator('.card').filter({ hasText: text });
  await card.getByRole('button', { name: 'Restore' }).click();
  await expect(card).toHaveCount(0);
  await page.locator('.back').click();
  await page.getByRole('button', { name: 'Activity' }).click();
  await expect(page.locator('.card').filter({ hasText: 'Restored' }).first()).toBeVisible();
  expect(page.url()).not.toContain('view=');
});

test('memories tab adds, searches, edits and forgets', async ({ page, isMobile }) => {
  const id = unique();
  await openTab(page, isMobile, 'Memories');
  await page.getByLabel('New memory').fill(`Keeps bees in the garden ${id}`);
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  const row = () => page.locator('.mem-row').filter({ hasText: id });
  await expect(row()).toContainText('Keeps bees in the garden');

  await page.getByLabel('Search memories').fill('bees garden');
  await expect(row()).toBeVisible();
  await page.getByLabel('Search memories').fill('');

  await row().getByTitle('Click to edit').click();
  await page.getByLabel('Edit memory').fill(`Keeps two hives of bees ${id}`);
  await page.keyboard.press('Enter');
  await expect(row()).toContainText('Keeps two hives of bees');

  await row().getByTitle('Forget').click();
  await expect(row()).toHaveCount(0);
  await page.getByRole('button', { name: 'Undo' }).click();
  await expect(row()).toContainText('Keeps two hives of bees');
});

test('memories tab filters decisions', async ({ page, isMobile }) => {
  const id = unique();
  await page.request.post('/api/remember', { data: { content: `Decided: use Postgres ${id}. Rejected: MySQL.`, tags: ['decision'] } });
  await page.request.post('/api/remember', { data: { content: `Likes green tea ${id}` } });
  await openTab(page, isMobile, 'Memories');
  await page.getByRole('button', { name: 'Decisions', exact: true }).click();
  await expect(page.locator('.mem-row').filter({ hasText: `use Postgres ${id}` })).toBeVisible();
  await expect(page.locator('.mem-row').filter({ hasText: `green tea ${id}` })).toHaveCount(0);
});

test('imports facts from a ChatGPT export', async ({ page, isMobile }) => {
  const city = `Springfield${unique()}`;
  const exportJSON = JSON.stringify([
    {
      title: 't',
      mapping: {
        a: { message: { author: { role: 'user' }, content: { content_type: 'text', parts: [`I live in ${city}. What should I see?`] }, create_time: 1700000000 } },
      },
    },
  ]);
  await openTab(page, isMobile, 'Memories');
  await page.getByRole('button', { name: 'Import', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Bring your memory with you' })).toBeVisible();
  await page.locator('input[type=file]').setInputFiles({ name: 'conversations.json', mimeType: 'application/json', buffer: Buffer.from(exportJSON) });
  await expect(page.getByRole('heading', { name: /Found \d+ fact/ })).toBeVisible();
  await expect(page.getByText(city)).toBeVisible();
  await page.getByRole('button', { name: /Save \d+ memor/ }).click();
  await expect(page.getByText(/Saved \d+ memor/)).toBeVisible();
  await page.getByRole('button', { name: 'See them' }).click();
  await expect(page.locator('.mem-row').filter({ hasText: city })).toContainText('ChatGPT import');
});

test('the manager links back to the app', async ({ page }) => {
  await page.goto('/');
  await page.getByTitle('Back to chat').click();
  await expect(page).toHaveURL(/\/app$/);
});

test('no horizontal scrolling', async ({ page }) => {
  expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)).toBe(false);
});
