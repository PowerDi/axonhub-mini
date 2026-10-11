import { expect, type Page } from '@playwright/test'

export interface AdminCredentials {
  email: string
  password: string
}

const defaultCredentials: AdminCredentials = {
  email: process.env.AXONHUB_ADMIN_EMAIL || 'my@example.com',
  password: process.env.AXONHUB_ADMIN_PASSWORD || 'pwd123456',
}

export async function expectAuthenticated(page: Page) {
  // AuthGuard only mounts the layout after the backend validates the user.
  await expect(page.locator('#content')).toBeVisible({ timeout: 15000 })
  await expect.poll(() => page.evaluate(() => !!localStorage.getItem('axonhub_access_token'))).toBe(true)
  await expect(page).not.toHaveURL(/\/(sign-in|initialization|500)(?:[/?#]|$)/)
}

export async function signInAsAdmin(page: Page, credentials: AdminCredentials = defaultCredentials) {
  await page.getByTestId('sign-in-email').fill(credentials.email)
  await page.getByTestId('sign-in-password').fill(credentials.password)

  const [response] = await Promise.all([
    page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/admin/auth/signin' && response.request().method() === 'POST'
    ),
    page.getByTestId('sign-in-submit').click(),
  ])
  expect(response.status(), 'The real sign-in request must succeed').toBe(200)
  await expectAuthenticated(page)
}

export async function ensureSignedIn(page: Page) {
  const signIn = page.getByTestId('sign-in-email')
  // Wait for either the rendered login form or the authenticated layout.
  // isVisible({ timeout }) does not wait, and a token alone may be expired.
  await expect(signIn.or(page.locator('#content')).first()).toBeVisible({ timeout: 20000 })
  if (await signIn.isVisible()) await signInAsAdmin(page)
  await expectAuthenticated(page)
}

export async function gotoAndEnsureAuth(page: Page, path: string) {
  await page.goto(path, { waitUntil: 'domcontentloaded' })
  await ensureSignedIn(page)
  const target = new URL(path, page.url())
  if (page.url() !== target.href) {
    await page.goto(path, { waitUntil: 'domcontentloaded' })
  }
  await expectAuthenticated(page)
  await expect(page).toHaveURL(target.href)
}

export async function waitForGraphQLOperation(page: Page, operationName: string) {
  const lowerCamel = operationName.charAt(0).toLowerCase() + operationName.slice(1)
  const operation = new RegExp(`\\b(${operationName}|${lowerCamel})\\b`)
  const response = await page.waitForResponse((response) => {
    if (new URL(response.url()).pathname !== '/admin/graphql') return false
    const body = response.request().postData()
    return !!body && operation.test(body)
  })
  expect(response.ok(), `${operationName} HTTP response`).toBe(true)
  const body = await response.json()
  expect(body.errors, `${operationName} GraphQL errors`).toBeUndefined()
  expect(body.data, `${operationName} GraphQL data`).toBeTruthy()
  return body.data
}

// Prepare isolated test data through the real admin API.
export async function adminGraphQL(page: Page, query: string, variables: Record<string, unknown>) {
  const token = await page.evaluate(() => localStorage.getItem('axonhub_access_token'))
  expect(token).toBeTruthy()
  const response = await page.request.post('/admin/graphql', {
    headers: { Authorization: `Bearer ${token}` }, data: { query, variables },
  })
  expect(response.ok()).toBe(true)
  const body = await response.json()
  expect(body.errors).toBeUndefined()
  return body.data
}
