import { expect, test, type Page } from '@playwright/test';
import { openFixture } from './helpers';

async function choosePalette(page: Page, palette: 'Blue' | 'Mini') {
  await page.getByRole('button', { name: 'Toggle theme', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Color scheme', exact: true }).hover();
  await page.getByRole('menuitem', { name: palette, exact: true }).click();
  await expect(page.locator('html')).toHaveClass(new RegExp(`\\b${palette.toLowerCase()}\\b`));
}

for (const palette of ['Blue', 'Mini'] as const) {
  test(`${palette} keeps native plugin actions, live model preview and Mini channel fields`, async ({ page }) => {
    const { operations, errors } = await openFixture(page, 'native-ui');
    await choosePalette(page, palette);
    await expect(page.getByRole('button', { name: 'Fixture plugin tool', exact: true })).toBeVisible();
    await page.getByTestId('open-alias-settings').click();
    await expect(page.getByText('Model preview', { exact: true })).toBeVisible();
    await page.getByPlaceholder('e.g., deepseek', { exact: true }).fill('A');
    await expect(page.getByTestId('model-alias-preview-scroll').getByText('A/gpt-4o', { exact: true })).toBeVisible();
    expect(operations.some((op) => op.operationName === 'PreviewChannelModelEntries' && op.variables.input.settings.extraModelPrefix === 'A')).toBe(true);
    expect(operations.some((op) => op.operationName.startsWith('Update'))).toBe(false);
    await page.screenshot({ path: `../.agent/summary/native-ui-${palette.toLowerCase()}.png`, fullPage: true });
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
    await page.getByTestId('open-channel-settings').click();
    await expect(page.getByTestId('channel-hammer-retry-switch')).toBeChecked();
    await expect(page.getByTestId('response-timeout-mode')).toBeVisible();
    expect(errors).toEqual([]);
  });
}

test('Mini palette persists and switching back restores native variables without replacing the page', async ({ page }) => {
  const { errors } = await openFixture(page, 'native-ui');
  await choosePalette(page, 'Mini');
  const miniPrimary = await page.locator('html').evaluate((root) => getComputedStyle(root).getPropertyValue('--primary'));
  await page.reload();
  await expect(page.locator('html')).toHaveClass(/\bmini\b/);
  await expect(page.getByRole('button', { name: 'Fixture plugin tool', exact: true })).toBeVisible();
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(page.locator('html')).toHaveClass(/\bdark\b/);
  await choosePalette(page, 'Blue');
  await expect(page.locator('html')).not.toHaveClass(/\bmini\b/);
  const nativePrimary = await page.locator('html').evaluate((root) => getComputedStyle(root).getPropertyValue('--primary'));
  expect(nativePrimary).not.toBe(miniPrimary);
  await expect(page.getByTestId('open-alias-settings')).toBeVisible();
  expect(errors).toEqual([]);
});
