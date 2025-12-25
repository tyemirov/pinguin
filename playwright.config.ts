import { defineConfig, devices } from '@playwright/test';
import path from 'path';

const testServerUrl = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:4174';
const devServerPath = path.join(process.cwd(), 'tests', 'support', 'devServer.js');

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 60 * 1000,
  expect: {
    timeout: 5 * 1000,
  },
  fullyParallel: false,
  globalSetup: './tests/e2e/global-setup.js',
  reporter: [['list']],
  workers: 1,
  use: {
    baseURL: testServerUrl,
    headless: true,
    actionTimeout: 5 * 1000,
    trace: 'on-first-retry',
  },
  webServer: {
    command: `node ${JSON.stringify(devServerPath)}`,
    url: testServerUrl,
    reuseExistingServer: true,
    stdout: 'pipe',
    stderr: 'pipe',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
