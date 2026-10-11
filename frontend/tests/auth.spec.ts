import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, expectAuthenticated } from './auth.utils'

test('reuses the setup session without another sign-in', async ({ page }) => {
  const signIns: string[] = []
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === '/admin/auth/signin') signIns.push(request.url())
  })
  await gotoAndEnsureAuth(page, '/channels')
  await expect(page.getByTestId('add-channel-button')).toBeVisible()
  expect(signIns).toHaveLength(0)
})

test.describe('Fresh browser authentication', () => {
  test.use({ storageState: { cookies: [], origins: [] } })

  test('logs in after a delayed auth redirect and preserves the requested page', async ({ page }) => {
    test.setTimeout(60000)
    // Delay the real status response beyond the old helper's fixed 2s sleep.
    await page.route('**/admin/system/status', async (route) => {
      const response = await route.fetch()
      await new Promise((resolve) => setTimeout(resolve, 3000))
      await route.fulfill({ response })
    }, { times: 1 })

    await gotoAndEnsureAuth(page, '/channels')
    await expect(page.getByTestId('add-channel-button')).toBeVisible()
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expectAuthenticated(page)
    await expect(page).toHaveURL(/\/channels$/)
  })
})
