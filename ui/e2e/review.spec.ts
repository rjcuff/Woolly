import { expect, type Page, test } from '@playwright/test';

const unique = () => Math.random().toString(36).slice(2, 8);

async function add(page: Page, text: string, key?: string) {
  await page.getByLabel('New memory').fill(text);
  if (key) {
    const more = page.getByRole('button', { name: 'Add a key or tags' });
    if (await more.isVisible()) await more.click();
    await page.getByLabel('Key', { exact: true }).fill(key);
  }
  const saved = page.waitForResponse((r) => r.url().endsWith('/api/remember'));
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await saved;
}

test.beforeEach(async ({ page }) => {
  await page.goto(`/auth?token=${process.env.WOOLLY_TEST_TOKEN}`);
  await expect(page.getByText('Woolly', { exact: true })).toBeVisible();
});

test('adds a memory and finds it with search', async ({ page }) => {
  const text = `I like hiking ${unique()}`;
  await add(page, text);
  await expect(page.getByRole('status')).toHaveText(/Saved/);
  await expect(page.getByText(text)).toBeVisible();

  await page.getByRole('searchbox').fill('hiking');
  await expect(page.getByText(text)).toBeVisible();
});

test('refuses to save a secret', async ({ page }) => {
  await add(page, 'My OpenAI key is sk-proj-abcdefghijklmnopqrstuvwxyz123456');
  await expect(page.getByRole('alert')).toContainText('an OpenAI API key');
  await expect(page.getByText('sk-proj-abcdefghijklmnopqrstuvwxyz123456')).toHaveCount(0);
});

test('edits a memory', async ({ page }) => {
  const id = unique();
  await add(page, `Drinks coffee ${id}`);
  const row = page.getByRole('listitem').filter({ hasText: `Drinks coffee ${id}` });
  await row.getByRole('button', { name: 'Edit' }).click();
  await page.getByRole('textbox', { name: 'Edit memory' }).fill(`Drinks tea now ${id}`);
  await page.getByRole('listitem').getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText(`Drinks tea now ${id}`)).toBeVisible();
  await expect(page.getByText(`Drinks coffee ${id}`)).toHaveCount(0);
});

test('forget moves to trash and undo brings it back', async ({ page }) => {
  const text = `Temporary note ${unique()}`;
  await add(page, text);
  await page.getByRole('listitem').filter({ hasText: text }).getByRole('button', { name: 'Forget' }).click();
  await expect(page.getByRole('status')).toContainText('Moved to trash');
  await expect(page.getByRole('listitem').filter({ hasText: text })).toHaveCount(0);

  await page.getByRole('button', { name: 'Undo' }).click();
  await expect(page.getByRole('listitem').filter({ hasText: text })).toHaveCount(1);
});

test('trash restores and empties', async ({ page }) => {
  const text = `Trash me ${unique()}`;
  await add(page, text);
  await page.getByRole('listitem').filter({ hasText: text }).getByRole('button', { name: 'Forget' }).click();

  await page.getByRole('button', { name: 'Trash' }).click();
  const row = page.getByRole('listitem').filter({ hasText: text });
  await row.getByRole('button', { name: 'Restore' }).click();
  await expect(row).toHaveCount(0);
  await page.getByRole('button', { name: 'Memories' }).click();
  await expect(page.getByRole('listitem').filter({ hasText: text })).toHaveCount(1);

  await page.getByRole('listitem').filter({ hasText: text }).getByRole('button', { name: 'Forget' }).click();
  await page.getByRole('button', { name: 'Trash' }).click();
  await page.getByRole('button', { name: 'Empty trash' }).click();
  await page.getByRole('button', { name: 'Delete all for good' }).click();
  await expect(page.getByText('The trash is empty.')).toBeVisible();
});

test('without the token the page explains how to open it', async ({ browser, baseURL }) => {
  const context = await browser.newContext({ baseURL, extraHTTPHeaders: {} });
  const page = await context.newPage();
  await page.goto('/');
  await expect(page.getByRole('alert')).toContainText('woolly ui');
  await context.close();
});

test('a new fact with the same key replaces the old one', async ({ page }) => {
  const key = `user.city_${unique()}`;
  await add(page, 'Lives in Austin', key);
  await add(page, 'Lives in Denver', key);
  const current = page.getByRole('listitem').filter({ hasText: key });
  await expect(current).toHaveCount(1);
  await expect(current).toContainText('Lives in Denver');

  await page.getByRole('button', { name: 'History' }).click();
  await expect(page.getByRole('listitem').filter({ hasText: 'Lives in Austin' }).first()).toContainText('Replaced');
});

test('resolves a conflict', async ({ page, request }) => {
  const key = `code.indent_${unique()}`;
  await request.post('/api/remember', { data: { content: 'Prefers tabs', key, source: 'user' } });
  await request.post('/api/remember', { data: { content: 'Prefers spaces', key, source: 'agent' } });
  await page.reload();

  await page.getByRole('button', { name: /Conflicts/ }).click();
  const row = page.getByRole('listitem').filter({ hasText: key });
  await expect(row).toContainText('Prefers spaces');
  await expect(row).toContainText('Prefers tabs');
  await row.getByRole('button', { name: 'Keep this' }).click();
  await expect(page.getByRole('listitem').filter({ hasText: key })).toHaveCount(0);

  await page.getByRole('button', { name: 'Memories' }).click();
  await expect(page.getByRole('listitem').filter({ hasText: key })).toContainText('Prefers spaces');
});

test('shows and copies the profile', async ({ page, context, browserName }) => {
  test.skip(browserName !== 'chromium', 'clipboard permissions are Chromium only');
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  const text = `I prefer short answers ${unique()}`;
  await add(page, text);
  await page.getByRole('button', { name: 'Profile' }).click();
  await expect(page.locator('pre')).toContainText(text);
  await page.getByRole('button', { name: 'Copy profile' }).click();
  await expect(page.getByRole('status')).toHaveText(/Copied/);
  expect(await page.evaluate(() => navigator.clipboard.readText())).toContain('# About me');
});

test('shows which app saved a memory and what each app did', async ({ page }) => {
  const text = `Uses tabs for indentation ${unique()}`;
  const res = await page.request.post('/api/remember', {
    data: { content: text, source: 'agent' },
    headers: { 'X-Woolly-Client': 'cursor' },
  });
  expect(res.ok()).toBe(true);
  await page.reload();
  await expect(page.getByRole('listitem').filter({ hasText: text })).toContainText('via Cursor');

  const mine = `Saved from the page ${unique()}`;
  await add(page, mine);
  await expect(page.getByRole('listitem').filter({ hasText: mine })).toContainText('via review page');

  await page.getByRole('button', { name: 'Activity' }).click();
  const row = page.getByRole('listitem').filter({ hasText: text });
  await expect(row).toContainText('Saved');
  await expect(row).toContainText('Cursor');
  await page.getByLabel('App').selectOption('Cursor');
  await expect(page.getByRole('listitem').filter({ hasText: mine })).toHaveCount(0);
  await expect(row).toBeVisible();
});

test('filters by project', async ({ page }) => {
  const id = unique();
  const inProject = `Deploys with Fly ${id}`;
  const aboutMe = `Likes green tea ${id}`;
  await page.request.post('/api/remember', { data: { content: inProject, project: `acme-${id}` } });
  await page.request.post('/api/remember', { data: { content: aboutMe } });
  await page.reload();
  await expect(page.getByRole('listitem').filter({ hasText: inProject })).toContainText(`acme-${id}`);

  await page.getByLabel('Project').selectOption(`acme-${id}`);
  await expect(page.getByText(inProject)).toBeVisible();
  await expect(page.getByText(aboutMe)).toHaveCount(0);

  await page.getByLabel('Project').selectOption('About you');
  await expect(page.getByText(aboutMe)).toBeVisible();
  await expect(page.getByText(inProject)).toHaveCount(0);
});

test('chat explains how to set up Ollama', async ({ page }) => {
  await page.getByRole('button', { name: 'Chat' }).click();
  await expect(page.getByText('Chat runs on a model in Ollama')).toBeVisible();
  await expect(page.getByText('ollama pull qwen2.5:3b')).toBeVisible();
});

test('opens a tab from the address', async ({ page }) => {
  await page.goto('/?view=activity');
  await expect(page.getByRole('button', { name: 'Activity' })).toHaveAttribute('aria-current', 'page');
});

test('no horizontal scrolling', async ({ page }) => {
  expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)).toBe(false);
});

test('page loads nothing from the internet', async ({ page }) => {
  const external: string[] = [];
  page.on('request', (r) => {
    if (!r.url().startsWith('http://127.0.0.1')) external.push(r.url());
  });
  await page.reload();
  await page.waitForLoadState('networkidle');
  expect(external).toEqual([]);
});
