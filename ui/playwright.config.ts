import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { defineConfig, devices } from '@playwright/test';

const port = 11499;
// A throwaway memory folder so tests never touch real memories.
const home = mkdtempSync(join(tmpdir(), 'woolly-ui-test-'));

export default defineConfig({
  testDir: './e2e',
  workers: 1, // tests share one memory folder
  reporter: 'list',
  use: { baseURL: `http://127.0.0.1:${port}` },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
  ],
  webServer: {
    command: `npm run build && cd .. && go run ./cmd/woolly serve --addr 127.0.0.1:${port}`,
    url: `http://127.0.0.1:${port}/api/version`,
    env: { WOOLLY_HOME: home, WOOLLY_NO_EMBED: '1' },
    reuseExistingServer: false,
    timeout: 180_000,
  },
});
