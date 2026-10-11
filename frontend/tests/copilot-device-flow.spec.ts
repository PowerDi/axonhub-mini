import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

const START = '**/admin/copilot/oauth/start'
const POLL = '**/admin/copilot/oauth/poll'
const startResult = {
  session_id: 'test-session-id',
  user_code: 'ABCD-EFGH',
  verification_uri: 'https://github.com/login/device',
  expires_in: 900,
  interval: 1,
}

// OAuth responses are simulated; login, channel storage and reload use the real backend.
test.describe('GitHub Copilot Device Flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.route(START, (route) => route.fulfill({ json: startResult }))
    await page.route(POLL, (route) => route.fulfill({ json: { status: 'pending' } }))
    await gotoAndEnsureAuth(page, '/channels')
    await page.getByTestId('add-channel-button').click()
    await page.getByTestId('provider-github_copilot').click()
  })

  test('shows an enabled connect button initially', async ({ page }) => {
    await expect(page.getByRole('button', { name: 'Connect to GitHub Copilot' })).toBeEnabled()
  })

  test('shows loading until the start response arrives', async ({ page }) => {
    await page.route(START, async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 1000))
      await route.fulfill({ json: startResult })
    })
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    await expect(dialog.getByText('Starting...')).toBeVisible()
    await expect(dialog.getByText(startResult.user_code)).toBeVisible()
  })

  test('displays the code, verification URL and waiting instructions', async ({ page }) => {
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    await expect(dialog.getByText(startResult.user_code)).toBeVisible()
    await expect(dialog.getByText(startResult.verification_uri)).toBeVisible()
    await expect(dialog.getByRole('button', { name: 'Open GitHub' })).toBeEnabled()
    await expect(dialog.getByText(/Waiting for you to authorize/i)).toBeVisible()
  })

  test('shows a start error and allows retry', async ({ page }) => {
    await page.route(START, (route) => route.fulfill({ status: 500, json: { error: 'Failed to start device flow' } }), { times: 1 })
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    await expect(dialog.getByText('Failed to start device flow', { exact: true })).toBeVisible()
    await dialog.getByRole('button', { name: 'Retry', exact: true }).click()
    await dialog.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    await expect(dialog.getByText(startResult.user_code)).toBeVisible()
  })

  test('can reset a pending authorization', async ({ page }) => {
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    await expect(dialog.getByText(startResult.user_code)).toBeVisible()
    await dialog.getByRole('button', { name: 'Retry', exact: true }).click()
    await expect(dialog.getByText(startResult.user_code)).toBeHidden()
    await expect(dialog.getByRole('button', { name: 'Connect to GitHub Copilot' })).toBeEnabled()
  })

  test('uses system authentication without inheriting project context', async ({ page }) => {
    const requestPromise = page.waitForRequest('**/admin/copilot/oauth/start')
    await page.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    const request = await requestPromise
    expect(request.headers().authorization).toMatch(/^Bearer .+/)
    expect(request.headers()['x-project-id']).toBeUndefined()
    await expect(page.getByRole('dialog').getByText(startResult.user_code)).toBeVisible()
  })

  test('stores authorized credentials and shows them as connected after reopening', async ({ page }) => {
    test.setTimeout(60000)
    let polls = 0
    await page.route(POLL, (route) => {
      expect(route.request().postDataJSON()).toEqual({ session_id: startResult.session_id })
      return route.fulfill({ json: ++polls >= 2 ? { access_token: 'test-copilot-token', token_type: 'Bearer' } : { status: 'pending' } })
    })
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    await expect(dialog.getByText(/Connected successfully|credentials imported/i).first()).toBeVisible({ timeout: 10000 })
    expect(polls).toBeGreaterThanOrEqual(2)

    const name = `pw-copilot-${Date.now()}`
    await dialog.getByTestId('channel-name-input').fill(name)
    await dialog.getByPlaceholder('Model ID', { exact: true }).fill('gpt-4o')
    await dialog.getByPlaceholder('Model ID', { exact: true }).locator('..').getByRole('button', { name: 'Add', exact: true }).click()
    await dialog.getByTestId('default-test-model-select').click()
    await page.getByRole('option', { name: 'gpt-4o', exact: true }).click()
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      dialog.getByTestId('channel-submit-button').click(),
    ])
    await expect(dialog).toBeHidden()
    await page.reload()
    const row = page.getByTestId('channels-table').getByRole('row').filter({ hasText: name })
    await row.locator('td:last-child button').first().click()
    await expect(dialog.getByText(/Already connected to GitHub Copilot/i)).toBeVisible()
    await expect(dialog.getByRole('button', { name: /Re-authenticate/i })).toBeEnabled()
  })

  test('backs off polling after slow_down', async ({ page }) => {
    const polls: number[] = []
    await page.route(POLL, (route) => {
      polls.push(Date.now())
      return route.fulfill({ json: { status: polls.length === 1 ? 'slow_down' : 'pending' } })
    })
    await page.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    await expect.poll(() => polls.length, { timeout: 10000 }).toBeGreaterThanOrEqual(2)
    expect(polls[1] - polls[0]).toBeGreaterThanOrEqual(1900)
    await expect(page.getByRole('dialog').getByText(/Waiting for you to authorize/i)).toBeVisible()
  })

  test('shows an expired session error with a retry action', async ({ page }) => {
    await page.route(POLL, (route) => route.fulfill({ status: 400, json: { error: 'expired_token' } }))
    await page.getByRole('button', { name: 'Connect to GitHub Copilot' }).click()
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByText('expired_token', { exact: true })).toBeVisible()
    await expect(dialog.getByRole('button', { name: 'Retry', exact: true })).toBeEnabled()
  })
})
