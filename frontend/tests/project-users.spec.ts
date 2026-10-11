import { randomUUID } from 'node:crypto'
import { test as base, expect, type Page } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

type Membership = { name: string; email: string; outsiderEmail: string }
const test = base.extend<{ membership: Membership }>({
  membership: async ({ page }, use) => {
    await gotoAndEnsureAuth(page, '/')
    const token = await page.evaluate(() => localStorage.getItem('axonhub_access_token'))
    async function graphql(query: string, variables: Record<string, unknown>) {
      const response = await page.request.post('/admin/graphql', {
        headers: { Authorization: `Bearer ${token}` }, data: { query, variables },
      })
      expect(response.ok()).toBe(true)
      const body = await response.json()
      expect(body.errors).toBeUndefined()
      return body.data
    }
    const name = `pw-member-${randomUUID().slice(0, 8)}`
    const email = `${name}@example.com`
    const outsiderEmail = `${name}-new@example.com`
    const project = await graphql('mutation($input: CreateProjectInput!) { createProject(input: $input) { id } }', { input: { name } })
    const projectId = project.createProject.id
    const users: string[] = []
    try {
      for (const address of [email, outsiderEmail]) {
        const user = await graphql('mutation($input: CreateUserInput!) { createUser(input: $input) { id } }', {
          input: { email: address, firstName: name, lastName: 'Member', password: 'pwd123456' },
        })
        users.push(user.createUser.id)
      }
      await graphql('mutation($input: AddUserToProjectInput!) { addUserToProject(input: $input) { id } }', {
        input: { projectId, userId: users[0], isOwner: false, scopes: [] },
      })
      await page.reload()
      await page.getByRole('button', { name: 'Default', exact: true }).click()
      await page.getByRole('menuitem', { name, exact: true }).click()
      await gotoAndEnsureAuth(page, '/project/users')
      await expect(page.getByTestId('users-table').getByRole('row').filter({ hasText: email })).toBeVisible()
      await use({ name, email, outsiderEmail })
    } finally {
      await graphql('mutation($id: ID!) { deleteProject(id: $id) }', { id: projectId })
      for (const id of users) await graphql('mutation($id: ID!) { deleteUser(id: $id) }', { id })
    }
  },
})

async function assignPermissions(page: Page) {
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('checkbox', { name: 'Developer', exact: true }).check()
  await dialog.getByRole('combobox').last().click()
  await page.getByRole('option', { name: /^read_users\b/ }).click()
  await page.keyboard.press('Escape')
}

test.describe('Project members', () => {
  test.setTimeout(60000)

  test('adds a member with a role and project scopes', async ({ page, membership }) => {
    await page.getByRole('button', { name: 'Add User', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('combobox').first().click()
    await page.getByPlaceholder(/Search users/i).fill(membership.outsiderEmail)
    await page.getByRole('option', { name: new RegExp(membership.outsiderEmail) }).click()
    await assignPermissions(page)
    await Promise.all([
      waitForGraphQLOperation(page, 'AddUserToProject'),
      dialog.getByRole('button', { name: 'Add to Project', exact: true }).click(),
    ])
    await expect(dialog).toBeHidden()
    await page.reload()
    const row = page.getByTestId('users-table').getByRole('row').filter({ hasText: membership.outsiderEmail })
    await expect(row).toContainText('Developer')
    await expect(row).toContainText('read_users')
  })

  test('saves edited roles and scopes across a reload', async ({ page, membership }) => {
    const row = page.getByTestId('users-table').getByRole('row').filter({ hasText: membership.email })
    await row.getByRole('button').last().click()
    await page.getByRole('menuitem', { name: 'Edit', exact: true }).click()
    await assignPermissions(page)
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateProjectUser'),
      page.getByRole('dialog').getByRole('button', { name: 'Save changes', exact: true }).click(),
    ])
    await expect(page.getByRole('dialog')).toBeHidden()
    await page.reload()
    await expect(row).toContainText('Developer')
    await expect(row).toContainText('read_users')
  })

  test('requires name confirmation and removes only the selected member', async ({ page, membership }) => {
    const row = page.getByTestId('users-table').getByRole('row').filter({ hasText: membership.email })
    await row.getByRole('button').last().click()
    await page.getByRole('menuitem', { name: /Remove/i }).click()
    const dialog = page.getByRole('alertdialog')
    const remove = dialog.getByRole('button', { name: 'Remove', exact: true })
    await expect(remove).toBeDisabled()
    await dialog.getByTestId('remove-confirmation-input').fill(`${membership.name} Member`)
    await Promise.all([waitForGraphQLOperation(page, 'RemoveUserFromProject'), remove.click()])
    await expect(dialog).toBeHidden()
    await page.reload()
    await expect(row).toBeHidden()
    await expect(page.getByTestId('users-table').getByRole('row').filter({ hasText: 'my@example.com' })).toBeVisible()
  })

  test('filters members by email and name, shows no matches, and resets', async ({ page, membership }) => {
    const table = page.getByTestId('users-table')
    const search = page.getByPlaceholder('Filter users...', { exact: true })
    const owner = table.getByRole('row').filter({ hasText: 'my@example.com' })
    const member = table.getByRole('row').filter({ hasText: membership.email })
    await search.fill(membership.email)
    await expect(member).toBeVisible()
    await expect(owner).toBeHidden()
    await search.fill(membership.name)
    await expect(member).toBeVisible()
    await expect(owner).toBeHidden()
    await search.fill('no-such-project-member')
    await expect(member).toBeHidden()
    await expect(table.getByText(/No (results|data)/i)).toBeVisible()
    await search.clear()
    await expect(owner).toBeVisible()
    await expect(member).toBeVisible()
  })
})
