import { test, expect } from '@playwright/test'
import { adminAuthFile } from './auth-state'
import { expectAuthenticated, signInAsAdmin, waitForGraphQLOperation } from './auth.utils'

test('initialize system and save a verified admin session', async ({ page }) => {
  const email = process.env.AXONHUB_ADMIN_EMAIL || 'my@example.com'
  const password = process.env.AXONHUB_ADMIN_PASSWORD || 'pwd123456'

  await page.goto('/', { waitUntil: 'domcontentloaded' })
  await expect(page).toHaveURL(/\/(initialization|sign-in)(?:[/?#]|$)/, { timeout: 20000 })

  if (new URL(page.url()).pathname === '/initialization') {
    await page.getByRole('textbox', { name: /Owner First Name/i }).fill('Admin')
    await page.getByRole('textbox', { name: /Owner Last Name/i }).fill('User')
    await page.getByRole('textbox', { name: /Owner Email/i }).fill(email)
    await page.getByLabel(/Owner Password/i).fill(password)
    await page.getByRole('textbox', { name: /Brand Name/i }).fill('AxonHub')
    await page.getByRole('button', { name: /Initialize System|初始化系统/i }).click()
    await expect(page).toHaveURL(/\/sign-in(?:[/?#]|$)/, { timeout: 15000 })
  }

  const [data] = await Promise.all([
    waitForGraphQLOperation(page, 'OnboardingInfo'),
    signInAsAdmin(page, { email, password }),
  ])
  let info = data.onboardingInfo
  if (!info?.onboarded) {
    const [updated] = await Promise.all([
      waitForGraphQLOperation(page, 'OnboardingInfo'),
      waitForGraphQLOperation(page, 'CompleteOnboarding'),
      page.getByTestId('onboarding-skip-tour').click(),
    ])
    info = updated.onboardingInfo
    await expect(page.getByTestId('onboarding-skip-tour')).toBeHidden()
  }
  if (!info?.autoDisableChannel?.onboarded) {
    await Promise.all([
      waitForGraphQLOperation(page, 'CompleteAutoDisableChannelOnboarding'),
      page.getByTestId('auto-disable-onboarding-skip-tour').click(),
    ])
    await expect(page.getByTestId('auto-disable-onboarding-skip-tour')).toBeHidden()
  }

  if (!info?.systemModelSetting?.onboarded) {
    await page.goto('/models')
    await expect(page.locator('#driver-popover-content')).toBeVisible()
    await Promise.all([
      waitForGraphQLOperation(page, 'CompleteSystemModelSettingOnboarding'),
      page.locator('[data-settings-button]').click(),
    ])
    await expect(page.locator('#driver-popover-content')).toBeHidden()
    await page.keyboard.press('Escape')
    await page.goto('/')
  }

  // Reload before saving: an in-memory login alone is insufficient for reuse.
  await page.reload({ waitUntil: 'domcontentloaded' })
  await expectAuthenticated(page)
  await page.goto('/channels')
  await expect(page.getByTestId('add-channel-button')).toBeVisible()
  await page.context().storageState({ path: adminAuthFile })
})
