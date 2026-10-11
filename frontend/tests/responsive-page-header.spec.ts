import { test, expect, type Page } from '@playwright/test'
import { gotoAndEnsureAuth } from './auth.utils'

const header = (page: Page) => page.locator('#content header')
const main = (page: Page) => page.locator('#content main')

async function openPage(page: Page, route: string, width = 375, height = 667) {
  await page.setViewportSize({ width, height })
  await gotoAndEnsureAuth(page, route)
  await expect(header(page).getByRole('heading')).toBeVisible()
  await expect(main(page)).toBeVisible()
}

async function expectInFlow(page: Page) {
  const geometry = await page.locator('#content').evaluate((content) => {
    const heading = content.querySelector('header')!.getBoundingClientRect()
    const body = content.querySelector('main')!.getBoundingClientRect()
    return { bottom: heading.bottom, top: body.top, width: document.documentElement.scrollWidth, viewport: innerWidth }
  })
  expect(geometry.bottom).toBeLessThanOrEqual(geometry.top + 1)
  expect(geometry.width).toBeLessThanOrEqual(geometry.viewport + 1)
}

async function expectActionsInside(page: Page) {
  for (const button of await header(page).getByRole('button').all()) {
    const box = await button.boundingBox()
    expect(box).not.toBeNull()
    expect(box!.x).toBeGreaterThanOrEqual(0)
    expect(box!.x + box!.width).toBeLessThanOrEqual(page.viewportSize()!.width)
  }
  await expectInFlow(page)
}

test.describe('Responsive page layout', () => {
  test('project description and create action fit the page', async ({ page }) => {
    await openPage(page, '/projects', 768, 800)
    await expect(header(page).locator('p')).toBeVisible()
    await expectActionsInside(page)
  })

  test('long storage descriptions remain readable above the body on a phone', async ({ page }) => {
    await openPage(page, '/data-storages')
    for (const paragraph of await header(page).locator('p').all()) {
      await expect(paragraph).toBeVisible()
      expect(await paragraph.evaluate((el) => el.scrollHeight <= el.clientHeight + 1)).toBe(true)
    }
    await expectActionsInside(page)
  })

  test('project actions stay inside the page while resizing', async ({ page }) => {
    await openPage(page, '/projects')
    for (const width of [375, 768, 1440]) {
      await page.setViewportSize({ width, height: 800 })
      await expect.poll(() => header(page).evaluate((el) => el.getBoundingClientRect().width)).toBeGreaterThan(0)
      await expectActionsInside(page)
    }
    await header(page).getByRole('button', { name: /Create Project|创建项目/i }).click()
    await expect(page.getByRole('dialog')).toBeVisible()
  })

  test('storage creation is fully visible and clickable on a phone', async ({ page }) => {
    await openPage(page, '/data-storages')
    await expectActionsInside(page)
    await header(page).getByRole('button', { name: /Create Payload Storage|创建载荷存储/i }).click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('dialog')).toBeHidden()
  })

  test('Tab reveals every channel and model action in the horizontal scroller', async ({ page }) => {
    for (const [path, id] of [['/channels', 'channel-actions-scroller'], ['/models', 'model-actions-scroller']]) {
      await openPage(page, path)
      const scroller = page.getByTestId(id)
      const buttons = scroller.getByRole('button')
      await expect(buttons.first()).toBeVisible()
      const count = await buttons.count()
      expect(count).toBeGreaterThan(1)
      expect(await scroller.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true)
      await buttons.first().focus()
      for (let i = 0; i < count; i++) {
        await expect(buttons.nth(i)).toBeFocused()
        const position = await buttons.nth(i).evaluate((el) => {
          const button = el.getBoundingClientRect()
          const clip = el.parentElement!.getBoundingClientRect()
          return { left: button.left - clip.left, right: clip.right - button.right }
        })
        expect(position.left).toBeGreaterThanOrEqual(3)
        expect(position.right).toBeGreaterThanOrEqual(3)
        if (i + 1 < count) await page.keyboard.press('Tab')
      }
      await page.keyboard.press('Shift+Tab')
      await expect(buttons.nth(count - 2)).toBeFocused()
      await expectInFlow(page)
    }
  })

  test('short viewports allow scrolling to pagination without squeezing away the table', async ({ page }) => {
    for (const [path, id] of [['/data-storages', 'data-storages'], ['/api-keys', 'api-keys']]) {
      await openPage(page, path, 375, 320)
      const size = page.getByTestId(`${id}-pagination`).getByRole('combobox')
      await size.scrollIntoViewIfNeeded()
      await size.click()
      await expect(page.getByRole('option', { name: '10', exact: true })).toBeVisible()
      await page.keyboard.press('Escape')
      const box = (await size.boundingBox())!
      expect(box.y + box.height).toBeLessThanOrEqual(321)
      expect(await page.getByTestId(`${id}-table-scroll`).evaluate((el) => el.getBoundingClientRect().height)).toBeGreaterThan(24)
      await expectInFlow(page)
    }
  })

  test('sidebar collapse and expansion resize the page without clipping actions', async ({ page }) => {
    await openPage(page, '/data-storages', 1270, 800)
    const sidebar = page.locator('[data-slot="sidebar"][data-state]')
    await expect(sidebar).toHaveAttribute('data-state', 'expanded')
    const original = (await header(page).boundingBox())!.width
    await page.locator('[data-sidebar="trigger"]').click()
    await expect(sidebar).toHaveAttribute('data-state', 'collapsed')
    await expect.poll(async () => (await header(page).boundingBox())!.width).toBeGreaterThan(original + 10)
    await expectActionsInside(page)
    await page.locator('[data-sidebar="trigger"]').click()
    await expect(sidebar).toHaveAttribute('data-state', 'expanded')
    await expect.poll(async () => Math.abs((await header(page).boundingBox())!.width - original)).toBeLessThan(1)
    await expectActionsInside(page)
  })

  test('storage settings remain reachable from a phone', async ({ page }) => {
    await openPage(page, '/data-storages')
    await expectActionsInside(page)
    await header(page).getByRole('button', { name: /Storage Settings|存储设置/i }).click()
    await expect(page).toHaveURL(/\/system\?tab=storage$/)
    await expect(page.getByRole('tab', { name: /Storage|存储/i })).toHaveAttribute('data-state', 'active')
  })
})
