import { randomBytes } from 'node:crypto';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { defineConfig, devices } from '@playwright/test';

const port = 11499;
// A throwaway memory folder so tests never touch real memories. Playwright
// loads this file again in each worker, so create the folder and token once
// and pass them on through the environment.
if (!process.env.WOOLLY_TEST_HOME) {
  const home = mkdtempSync(join(tmpdir(), 'woolly-ui-test-'));
  const token = randomBytes(32).toString('hex');
  // Woolly reuses an existing token file, so the tests know the token up front.
  writeFileSync(join(home, 'token'), token + '\n', { mode: 0o600 });
  process.env.WOOLLY_TEST_HOME = home;
  process.env.WOOLLY_TEST_TOKEN = token;
}
const home = process.env.WOOLLY_TEST_HOME;
const token = process.env.WOOLLY_TEST_TOKEN;

export default defineConfig({
  testDir: './e2e',
  workers: 1, // tests share one memory folder
  reporter: 'list',
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
  ],
  webServer: {
    command: `npm run build && cd .. && go run ./cmd/woolly serve --addr 127.0.0.1:${port}`,
    url: `http://127.0.0.1:${port}/api/version`,
    // No Ollama, so the chat shows its setup steps.
    env: { WOOLLY_HOME: home, WOOLLY_NO_EMBED: '1', OLLAMA_HOST: '127.0.0.1:9' },
    reuseExistingServer: false,
    timeout: 180_000,
  },
});
