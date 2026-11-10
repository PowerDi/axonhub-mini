import { expect, test } from '@playwright/test';
import { channel, PROJECT_ID } from './fixtures';
import { openFixture } from './helpers';

test('multi-endpoint channel can add a separate Responses WebSocket URL without losing its HTTP URL', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'endpoints');
  await page.getByRole('dialog').getByTestId('endpoint-format-select').click();
  await page.getByRole('option', { name: 'openai/responses-ws', exact: true }).click();
  await page.getByRole('dialog').locator('#endpoint-base-url').fill('wss://ws.example.test/v1');
  await page.getByRole('dialog').locator('#endpoint-path').fill('/custom/responses');
  await page.getByRole('dialog').getByTestId('endpoint-add-button').click();
  await page.getByRole('dialog').getByTestId('endpoints-save-button').click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  const saved = operations.find((op) => op.operationName === 'UpdateChannel');
  expect(saved?.variables.input.endpoints).toEqual(expect.arrayContaining([
    expect.objectContaining({ apiFormat: 'openai/responses', baseURL: 'https://responses.example.test/v1' }),
    expect.objectContaining({ apiFormat: 'openai/responses-ws', baseURL: 'wss://ws.example.test/v1', path: '/custom/responses' }),
  ]));
  expect(saved?.variables.input.settings.hammerRetry).toEqual(channel.settings.hammerRetry);
  expect(errors).toEqual([]);
});

test('legacy Codex WS save normalizes both Allow and Exclude atomically', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'codex-endpoints');
  await expect(page.getByRole('dialog').getByTestId('codex-endpoints-websocket-checkbox')).toBeChecked();
  await page.getByRole('dialog').getByTestId('endpoints-save-button').click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  const saved = operations.find((op) => op.operationName === 'UpdateChannel')!;
  expect(saved.variables.input.baseURL).toBe('https://codex.example.test/v1');
  expect(saved.variables.input.settings.modelApiFormatPolicies).toEqual([
    { model: 'gpt-4o', allow: ['openai/responses-ws'], exclude: [] },
    { model: 'deny-model', allow: [], exclude: ['openai/responses-ws'] },
  ]);
  expect(errors).toEqual([]);
});

test('model policy edit uses settings PATCH and preserves legacy WebSocket identity', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'legacy-policy');
  await expect(page.getByRole('dialog').getByTestId('model-policy-gpt-4o')).toContainText('openai/responses-ws');
  await expect(page.getByRole('dialog').getByTestId('model-policy-deny-model')).toContainText('openai/responses-ws');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  const saved = operations.find((op) => op.operationName === 'UpdateChannelSettings')!;
  expect(Object.keys(saved.variables.input)).toEqual(['modelApiFormatPolicies']);
  expect(saved.variables.input.modelApiFormatPolicies[0].allow).toEqual(['openai/responses-ws']);
  expect(operations.some((op) => op.operationName === 'UpdateChannel')).toBe(false);
  expect(errors).toEqual([]);
});

test('channel editor recalls and saves beta12 timeout/key settings without dropping mini settings', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'channel-edit');
  await expect(page.getByRole('dialog').getByTestId('response-timeout-stream-first-event')).toHaveValue('12');
  await expect(page.getByRole('dialog').getByTestId('response-timeout-non-stream')).toHaveValue('45');
  await page.getByRole('dialog').getByTestId('response-timeout-non-stream').fill('60');
  await page.getByRole('dialog').getByTestId('channel-submit-button').click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  const saved = operations.find((op) => op.operationName === 'UpdateChannel')!;
  expect(saved.variables.input.type).toBe('openai_multiprotocol');
  expect(saved.variables.input.settings.responseTimeout.nonStreamResponseTimeoutSeconds).toBe(60);
  expect(saved.variables.input.settings.apiKeyStrategy).toBe('round_robin_success');
  expect(saved.variables.input.settings.apiKeyRoundRobinSwitchAfter).toBe(3);
  expect(saved.variables.input.settings.reasoningFilter).toBe(true);
  expect(saved.variables.input.settings.modelApiFormatPolicies).toEqual(channel.settings.modelApiFormatPolicies);
  expect(saved.variables.input.settings.hammerRetry.maxRetries).toBe(4);
  expect(errors).toEqual([]);
});

test('profile manager recalls Exclude and preserves template linkage on submit', async ({ page }) => {
  const { errors } = await openFixture(page, 'profiles');
  await expect(page.getByRole('dialog').getByTestId('profile-channel-match-mode')).toContainText('Exclude selected');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  const saved = JSON.parse(await page.getByTestId('profile-submitted').textContent() ?? 'null');
  expect(saved.profiles[0]).toMatchObject({ channelIDsMatchMode: 'exclude', channelIDs: [101], templateID: 51, templateName: 'Shared exclusion' });
  expect(errors).toEqual([]);
});

test('new API key template offers Exclude and sends an explicitly project-scoped payload', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'template');
  await page.getByRole('textbox', { name: 'Template Name', exact: true }).fill('UI exclusion template');
  await page.getByRole('dialog').getByTestId('template-channel-match-mode').click();
  await page.getByRole('option', { name: 'Exclude selected', exact: true }).click();
  await page.getByRole('dialog').getByPlaceholder('Excluded Channels', { exact: true }).fill(channel.name);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('button', { name: `Remove ${channel.name} tag`, exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Create Template', exact: true }).click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  const saved = operations.find((op) => op.operationName === 'CreateApiKeyProfileTemplate')!;
  expect(saved.projectHeader).toBe(PROJECT_ID);
  expect(saved.variables.input.projectID).toBe(PROJECT_ID);
  expect(saved.variables.profile.channelIDsMatchMode).toBe('exclude');
  expect(saved.variables.profile.channelIDs).toEqual([101]);
  expect(errors).toEqual([]);
});

test('Intl currency formatting still uses configured currency and digits after i18next migration', async ({ page }) => {
  await openFixture(page, 'endpoints');
  await expect(page.getByTestId('currency-format')).toHaveText('$1,234.50');
});


test('endpoint editor refuses to remove the last allowed WebSocket endpoint', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'codex-endpoints');
  await page.getByRole('dialog').getByTestId('codex-endpoints-websocket-checkbox').click();
  await expect(page.getByRole('alertdialog')).toContainText('gpt-4o');
  await expect(page.getByRole('dialog', { includeHidden: true }).getByTestId('codex-endpoints-websocket-checkbox')).toBeChecked();
  expect(operations.some((op) => op.operationName.startsWith('Update'))).toBe(false);
  expect(errors).toEqual([]);
});

test('profile schemas preserve template linkage and normalize only legacy empty matching modes', async ({ page }) => {
  const { errors } = await openFixture(page, 'schemas');
  const results = JSON.parse(await page.getByTestId('schema-regression').textContent() ?? 'null');
  for (const result of results) {
    expect(result.invalidAccepted).toBe(false);
    expect(result.normalized.map((row: { profiles: { channelIDsMatchMode: string }[] }) => row.profiles[0].channelIDsMatchMode)).toEqual(['include', 'include', 'include', 'include', 'exclude']);
    for (const row of result.normalized) expect(row.profiles[0]).toMatchObject({ templateID: 51, templateName: 'Shared exclusion', channelTagsMatchMode: 'any' });
  }
  expect(errors).toEqual([]);
});

test('analytics charts retain custom legends after the Recharts API update', async ({ page }) => {
  const { errors } = await openFixture(page, 'analytics');
  await expect(page.getByTestId('trend-chart').locator('.recharts-legend-item')).toHaveCount(5);
  await expect(page.getByTestId('pie-charts').locator('.recharts-legend-item').first()).toContainText('Fixture channel');
  expect(errors).toEqual([]);
});

test('non-Codex legacy WSS policy does not block an unrelated endpoint save', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'non-codex-ws-endpoints');
  await page.getByRole('dialog').getByTestId('endpoint-format-select').click();
  await page.getByRole('option', { name: 'openai/chat_completions', exact: true }).click();
  await page.getByRole('dialog').getByTestId('endpoint-add-button').click();
  await page.getByRole('dialog').getByTestId('endpoints-save-button').click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  const saved = operations.find((op) => op.operationName === 'UpdateChannel')!;
  expect(saved.variables.input.baseURL).toBeUndefined(); // the existing WSS base URL is untouched
  expect(saved.variables.input.settings.modelApiFormatPolicies).toEqual([{ model: 'gpt-4o', allow: ['openai/responses-ws'], exclude: [] }]);
  expect(errors).toEqual([]);
});

test('duplicating a GCP channel submits instead of silently failing derived-type validation', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'channel-duplicate-gcp');
  await page.getByRole('dialog').getByTestId('channel-name-input').fill('GCP copy');
  await page.getByRole('dialog').getByTestId('channel-submit-button').click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  const saved = operations.find((op) => op.operationName === 'DuplicateChannel');
  expect(saved?.variables.input.name).toBe('GCP copy');
  expect(errors).toEqual([]);
});

test('policy settings PATCH shows a successful update toast', async ({ page }) => {
  const { operations, errors } = await openFixture(page, 'policy');
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  expect(operations.some((op) => op.operationName === 'UpdateChannelSettings')).toBe(true);
  await expect(page.locator('[data-sonner-toast]')).toContainText('Channel updated successfully');
  expect(errors).toEqual([]);
});

test('analytics detail headers align with throughput, TTFT and cost cells', async ({ page }) => {
  await openFixture(page, 'analytics');
  const details = page.getByTestId('dimension-details');
  await expect(details.locator('th')).toHaveCount(11);
  await expect(details.locator('tbody tr').first().locator('td')).toHaveCount(11);
  await expect(details.locator('th').nth(8)).toContainText('tok/s');
  await expect(details.locator('th').nth(9)).toContainText('TTFT');
});

test('default channel table fits desktop content and still scrolls on mobile', async ({ page }) => {
  const { errors } = await openFixture(page, 'channels-table');
  const table = page.getByTestId('channels-table');
  const scroller = table.locator('..');
  const dimensions = () => scroller.evaluate((element) => ({ width: element.clientWidth, scroll: element.scrollWidth }));
  const desktop = await dimensions();
  expect(desktop.scroll).toBeLessThanOrEqual(desktop.width + 1);
  const action = page.getByTestId('row-actions');
  const actionBounds = await action.boundingBox();
  expect(actionBounds!.x + actionBounds!.width).toBeLessThanOrEqual(1440);
  await action.click();
  await expect(page.getByRole('menu')).toBeVisible();
  await page.keyboard.press('Escape');
  await page.setViewportSize({ width: 390, height: 844 });
  const mobile = await dimensions();
  expect(mobile.scroll).toBeGreaterThan(mobile.width);
  await scroller.evaluate((element) => { element.scrollLeft = element.scrollWidth; });
  const mobileAction = await action.boundingBox();
  expect(mobileAction!.x + mobileAction!.width).toBeLessThanOrEqual(390);
  expect(errors).toEqual([]);
});
