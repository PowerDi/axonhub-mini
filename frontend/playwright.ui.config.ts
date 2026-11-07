import { defineConfig, devices } from '@playwright/test';

// Isolated browser/component acceptance tests. No real credentials or backend
// data are used; this server never restarts the managed development server.
export default defineConfig({
  testDir: './tests/ui-regression',
  testMatch: '**/*.spec.ts',
  fullyParallel: false,
  workers: 1,
  timeout: 60000,
  expect: { timeout: 15000 },
  reporter: 'list',
  outputDir: 'test-results/ui-regression',
  use: {
    ...devices['Desktop Chrome'],
    viewport: { width: 1440, height: 1000 },
    baseURL: 'http://127.0.0.1:9530',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: 'pnpm exec vite --config vite.ui.config.ts --host 127.0.0.1 --port 9530 --strictPort',
    url: 'http://127.0.0.1:9530/tests/ui-regression/index.html',
    reuseExistingServer: false,
    timeout: 120000,
  },
});
