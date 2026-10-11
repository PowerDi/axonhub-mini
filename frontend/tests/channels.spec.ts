import { createServer } from 'node:http'
import { test, expect, type Page } from '@playwright/test'
import { adminGraphQL, gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

async function createFixtureChannel(page: Page, name: string, baseURL = 'http://127.0.0.1:1/v1') {
  const data = await adminGraphQL(page, 'mutation($input: CreateChannelInput!) { createChannel(input: $input) { id } }', {
    input: { name, type: 'openai', baseURL, credentials: { apiKeys: ['e2e-test-key'] }, supportedModels: ['gpt-4o'], defaultTestModel: 'gpt-4o' },
  })
  return data.createChannel.id as string
}

test.describe('Admin Channels Management', () => {
  test.beforeEach(async ({ page }) => {
    // Increase timeout for authentication
    test.setTimeout(60000)
    await gotoAndEnsureAuth(page, '/channels')

    // Wait for page to fully load and channels table to appear
    await page.waitForTimeout(2000)
    
    // Wait for the channels table to be visible (indicates page is loaded)
    const channelsTable = page.locator('[data-testid="channels-table"]')
    await channelsTable.waitFor({ state: 'visible', timeout: 15000 })
  })

  test('can create, edit, and archive a channel', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-Channel ${uniqueSuffix}`
    const baseURL = `https://api.test-${uniqueSuffix}.example.com`

    // Step 1: Create a new channel
    // Wait for the add button to appear (requires write_channels permission)
    const createButton = page.getByTestId('add-channel-button')
    await expect(createButton).toBeVisible({ timeout: 10000 })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()
    await expect(createDialog).toContainText(/创建|Create/i)

    // Fill in channel details
    await createDialog.getByTestId('channel-name-input').fill(name)

    // Select provider (OpenAI) - use data-testid for reliable selection
    const openaiProviderRadio = createDialog.getByTestId('provider-openai')
    await openaiProviderRadio.click()

    // Fill in base URL
    await createDialog.getByTestId('channel-base-url-input').fill(baseURL)

    // Fill in API Key
    const apiKeyInput = createDialog.getByTestId('channel-api-key-input')
    await apiKeyInput.fill('  sk-test-key-' + uniqueSuffix + '  ')
    await createDialog.getByTestId('channel-name-input').click()
    await expect(createDialog.getByTestId('channel-name-input')).toBeFocused()

    // Add at least one supported model (required to enable Create button)
    // Wait for Quick Add Models section to appear and click on gpt-4o badge
    const modelBadge = createDialog.getByTestId('quick-model-gpt-4o')
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    // Wait for selection state to update
    await page.waitForTimeout(300)

    // Click "Add Selected" button to add the selected models
    const addSelectedButton = createDialog.getByTestId('add-selected-models-button')
    await expect(addSelectedButton).toBeEnabled({ timeout: 5000 })
    await addSelectedButton.click()
    await page.waitForTimeout(500)

    // Select Default Test Model (required field)
    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
    if ((await defaultTestModelSelect.count()) > 0) {
      await defaultTestModelSelect.click()
      // Select the first available option (gpt-4o)
      const firstOption = page.getByRole('option').first()
      await firstOption.click()
      await page.waitForTimeout(300)
    }

    // Submit the form
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByTestId('channel-submit-button').click(),
    ])

    // Wait for dialog to close
    await expect(createDialog).not.toBeVisible({ timeout: 10000 })

    // Verify channel appears in the table
    await page.waitForTimeout(1000)
    const channelsTable = page.locator('[data-testid="channels-table"]')

    // New channels are created with 'disabled' status by default.
    // If an active status filter excludes disabled channels (e.g. "Enabled" is pre-selected),
    // clear the filter so the new channel is visible.
    const statusBtn = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()
    const statusBtnText = await statusBtn.textContent()
    if (statusBtnText && /Enabled|启用/i.test(statusBtnText) && !/Disabled|禁用/i.test(statusBtnText)) {
      await statusBtn.click()
      await page.waitForTimeout(500)
      const disabledOpt = page
        .getByRole('option', { name: /Disabled|禁用/i })
        .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))
      if ((await disabledOpt.count()) > 0) {
        await disabledOpt.first().click()
        await page.waitForTimeout(500)
      }
      await page.keyboard.press('Escape')
      await page.waitForTimeout(500)
    }

    const channelRow = channelsTable.locator('tbody tr').filter({ hasText: name })
    await expect(channelRow).toBeVisible()
    const statusSwitch = channelRow.locator('[data-testid="channel-status-switch"]')
    await expect(statusSwitch).toBeVisible()
    await expect(statusSwitch).not.toBeChecked()

    // Step 2: Edit the channel (click the dedicated edit button in the actions cell)
    const editButton = channelRow.locator('td:last-child button').first()
    await editButton.click()

    const editDialog = page.getByRole('dialog', { name: /编辑|Edit Channel/i })
    await expect(editDialog).toBeVisible()
    await expect(editDialog).toContainText(/编辑|Edit Channel/i)

    // Update the name
    const updatedName = `${name} - Updated`
    const nameInput = editDialog.getByTestId('channel-name-input')
    await nameInput.clear()
    await nameInput.fill(updatedName)

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      editDialog.getByRole('button', { name: /Edit|编辑|保存|Save|更新|Update/i }).click(),
    ])

    // Wait for dialog to close
    await expect(editDialog).not.toBeVisible({ timeout: 10000 })

    // Wait for table to update
    await page.waitForTimeout(1000)

    // Re-locate the channel row with updated name
    const updatedChannelRow = channelsTable.locator('tbody tr').filter({ hasText: updatedName })
    await expect(updatedChannelRow).toBeVisible()
    await expect(updatedChannelRow).toContainText(updatedName)

    // Step 3: Archive the channel
    const archiveActionsTrigger = updatedChannelRow.locator('[data-testid="row-actions"]')
    await archiveActionsTrigger.click()
    const archiveMenu = page.getByRole('menu')
    await expect(archiveMenu).toBeVisible()
    await archiveMenu.getByRole('menuitem', { name: /归档|Archive/i }).click()

    const archiveDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(archiveDialog).toBeVisible()
    await expect(archiveDialog).toContainText(/归档|Archive/i)

    // Wait for dialog to stabilize
    await page.waitForTimeout(500)

    // Click the confirm button - it's the last button (first is Cancel)
    const archiveButton = archiveDialog.getByRole('button', { name: /归档|Archive/i }).last()
    await Promise.all([waitForGraphQLOperation(page, 'UpdateChannelStatus'), archiveButton.click()])

    // Wait for dialog to close before proceeding
    await expect(archiveDialog).not.toBeVisible({ timeout: 10000 })

    // Wait for table to update (archived channels are hidden by default)
    await page.waitForTimeout(1000)

    // Archived channels are excluded from the default view, so we need to apply the status filter
    // Click on Status filter button (in the toolbar, not the table header)
    // The filter uses a Popover, not a DropdownMenu
    const statusFilterButton = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()
    await statusFilterButton.click()

    // Wait for popover to open
    await page.waitForTimeout(500)

    // Select Archived filter - it's a CommandItem, not a menuitemcheckbox
    // Use a more flexible selector
    const archivedFilter = page
      .getByRole('option', { name: /Archived|已归档/i })
      .or(page.locator('[role="option"]').filter({ hasText: /Archived|已归档/i }))
    await expect(archivedFilter).toBeVisible({ timeout: 5000 })
    await archivedFilter.click()

    // Wait for filter to apply
    await page.waitForTimeout(1000)

    // Now verify the archived channel appears - switch should be disabled for archived channels
    const archivedChannelRow = channelsTable.locator('tbody tr').filter({ hasText: updatedName })
    await expect(archivedChannelRow).toBeVisible()
    const archivedSwitch = archivedChannelRow.locator('[data-testid="channel-status-switch"]')
    await expect(archivedSwitch).toBeDisabled()

    // Archived channels have their switch disabled — no Enable action in the menu.
    // The test verifies archive was successful (switch disabled) above.
  })

  test('preserves a third-party Codex base URL when creating a channel', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-CodexChannel-${uniqueSuffix}`
    const baseURL = `https://codex-${uniqueSuffix}.example.com/v1`

    await page.getByTestId('add-channel-button').click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByTestId('provider-codex').click()
    await createDialog.getByRole('tab', { name: /Third[- ]party|第三方/i }).click()
    await createDialog.getByTestId('channel-name-input').fill(name)
    await createDialog.getByTestId('channel-base-url-input').fill(baseURL)
    await createDialog.getByTestId('channel-api-key-input').fill(`sk-test-key-${uniqueSuffix}`)

    const modelBadge = createDialog.getByTestId('quick-model-gpt-5.2')
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    await createDialog.getByTestId('add-selected-models-button').click()

    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
    await expect(defaultTestModelSelect).toBeVisible({ timeout: 5000 })
    await defaultTestModelSelect.click()
    await page.getByRole('option', { name: 'gpt-5.2' }).click()

    const createRequest = page.waitForRequest((request) => {
      if (!request.url().includes('/admin/graphql') || request.method() !== 'POST') return false
      return request.postDataJSON()?.operationName === 'CreateChannel'
    })

    await Promise.all([
      createRequest,
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByTestId('channel-submit-button').click(),
    ])

    const payload = (await createRequest).postDataJSON() as { variables: { input: { baseURL: string } } }
    expect(payload.variables.input.baseURL).toBe(baseURL)
  })

  test('can test a channel', async ({ page }) => {
    const name = `pw-channel-test-${Date.now()}`
    const provider = createServer(async (request, response) => {
      let body = ''
      for await (const chunk of request) body += chunk
      const input = JSON.parse(body)
      const common = { id: 'chatcmpl-test', created: Math.floor(Date.now() / 1000), model: input.model }
      if (input.stream) {
        response.writeHead(200, { 'Content-Type': 'text/event-stream' })
        response.end(`data: ${JSON.stringify({ ...common, object: 'chat.completion.chunk', choices: [{ index: 0, delta: { role: 'assistant', content: 'Channel test succeeded' }, finish_reason: null }] })}\n\ndata: ${JSON.stringify({ ...common, object: 'chat.completion.chunk', choices: [{ index: 0, delta: {}, finish_reason: 'stop' }], usage: { prompt_tokens: 3, completion_tokens: 4, total_tokens: 7 } })}\n\ndata: [DONE]\n\n`)
      } else {
        response.writeHead(200, { 'Content-Type': 'application/json' })
        response.end(JSON.stringify({ ...common, object: 'chat.completion', choices: [{ index: 0, message: { role: 'assistant', content: 'Channel test succeeded' }, finish_reason: 'stop' }], usage: { prompt_tokens: 3, completion_tokens: 4, total_tokens: 7 } }))
      }
    })
    await new Promise<void>((resolve) => provider.listen(0, '127.0.0.1', resolve))
    let id: string | undefined
    try {
      id = await createFixtureChannel(page, name, `http://127.0.0.1:${(provider.address() as { port: number }).port}/v1`)
      await page.reload()
      await page.getByPlaceholder('Search by name', { exact: true }).fill(name)
      const row = page.getByTestId('channels-table').getByRole('row').filter({ hasText: name })
      const [result] = await Promise.all([
        waitForGraphQLOperation(page, 'TestChannel'),
        row.locator('td:last-child button').nth(1).click(),
      ])
      expect(result.testChannel.success, result.testChannel.error).toBe(true)
      expect(result.testChannel.message).toContain('Channel test succeeded')
    } finally {
      await new Promise<void>((resolve, reject) => provider.close((error) => error ? reject(error) : resolve()))
      if (id) await adminGraphQL(page, 'mutation($id: ID!) { deleteChannel(id: $id) }', { id })
    }
  })

  test('can search channels by name', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const searchTerm = `pw-test-SearchChannel${uniqueSuffix}`

    // Create a channel with a unique name for searching
    const createButton = page.getByTestId('add-channel-button')
    await expect(createButton).toBeVisible({ timeout: 10000 })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByTestId('channel-name-input').fill(searchTerm)

    // Select provider - use data-testid for reliable selection
    const openaiProviderRadio = createDialog.getByTestId('provider-openai')
    await openaiProviderRadio.click()

    await createDialog.getByTestId('channel-base-url-input').fill('https://api.openai.com/v1')
    await createDialog.getByTestId('channel-api-key-input').fill('sk-test-key-' + uniqueSuffix)

    // Add at least one supported model (required to enable Create button)
    // Wait for model badge to appear and click it
    const modelBadge = createDialog.getByTestId('quick-model-gpt-4o')
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    await page.waitForTimeout(300)
    
    const addSelectedButton = createDialog.getByTestId('add-selected-models-button')
    await expect(addSelectedButton).toBeEnabled({ timeout: 5000 })
    await addSelectedButton.click()
    await page.waitForTimeout(500)

    // Select Default Test Model (required field)
    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
    await expect(defaultTestModelSelect).toBeVisible({ timeout: 5000 })
    await defaultTestModelSelect.click()
    const firstOption = page.getByRole('option').first()
    await firstOption.click()
    await page.waitForTimeout(300)

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByTestId('channel-submit-button').click(),
    ])

    // Wait for dialog to close
    await expect(createDialog).not.toBeVisible({ timeout: 10000 })

    // Wait for the table to update
    await page.waitForTimeout(1000)

    // Newly created channels are disabled by default.
    // If an active status filter excludes disabled channels, include them.
    const statusBtn = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()
    const statusBtnText = await statusBtn.textContent()
    if (statusBtnText && /Enabled|启用/i.test(statusBtnText) && !/Disabled|禁用/i.test(statusBtnText)) {
      await statusBtn.click()
      await page.waitForTimeout(500)
      const disabledOpt = page
        .getByRole('option', { name: /Disabled|禁用/i })
        .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))
      if ((await disabledOpt.count()) > 0) {
        await disabledOpt.first().click()
        await page.waitForTimeout(500)
      }
      await page.keyboard.press('Escape')
      await page.waitForTimeout(500)
    }

    // Use the search filter
    const searchInput = page
      .locator('input[placeholder*="搜索"], input[placeholder*="Search"], input[type="search"]')
      .first()
    await searchInput.fill(searchTerm)

    // Wait for debounce and API call
    await page.waitForTimeout(1000)

    // Verify the searched channel appears
    const searchedRow = page.locator('tbody tr').filter({ hasText: searchTerm })
    await expect(searchedRow).toBeVisible()
  })

  test('can filter channels by type', async ({ page }) => {
    const name = `pw-provider-filter-${Date.now()}`
    const id = await createFixtureChannel(page, name)
    try {
      await page.reload()
      await page.getByTestId('channel-provider-openai').click()
      await page.getByPlaceholder('Search by name', { exact: true }).fill(name)
      const row = page.getByTestId('channels-table').getByRole('row').filter({ hasText: name })
      await expect(row).toBeVisible()
      await expect(row).toContainText('OpenAI')
      await expect(page.getByTestId('channel-provider-openai')).toHaveAttribute('aria-pressed', 'true')
    } finally {
      await adminGraphQL(page, 'mutation($id: ID!) { deleteChannel(id: $id) }', { id })
    }
  })

  test('can filter channels by status', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(1000)

    // Look for status filter button/dropdown
    // The button text may include the selected filter value (e.g. "Status Enabled"),
    // so we match on the base text "Status" or "状态"
    const statusFilterButton = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()

    const statusFilterCount = await statusFilterButton.count()
    if (statusFilterCount === 0) {
      test.skip()
      return
    }

    await statusFilterButton.click()

    // Wait for filter menu
    await page.waitForTimeout(500)

    // "Enabled" may already be pre-selected as the default filter, so clicking it
    // would deselect it instead of applying it. Use "Disabled" to test filtering,
    // which is not pre-selected.
    const disabledFilter = page
      .getByRole('option', { name: /Disabled|禁用/i })
      .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))

    const disabledFilterCount = await disabledFilter.count()
    if (disabledFilterCount > 0) {
      await disabledFilter.first().click()

      // Wait for filter to apply
      await page.waitForTimeout(1000)

      // Verify filtered results - disabled channels have unchecked switches
      const rows = page.locator('tbody tr')
      const rowCount = await rows.count()

      // Skip assertion if no disabled channels exist after filtering
      if (rowCount === 0) {
        return
      }

      const firstRow = rows.first()
      const statusSwitch = firstRow.locator('[data-testid="channel-status-switch"]')
      await expect(statusSwitch).toBeVisible({ timeout: 5000 })
      await expect(statusSwitch).not.toBeChecked({ timeout: 5000 })
    }
  })

  test('validates required fields when creating a channel', async ({ page }) => {
    // Wait for the page to be ready
    await page.waitForTimeout(1000)

    const createButton = page.getByTestId('add-channel-button')

    // Check if button exists (user may not have permission)
    const buttonCount = await createButton.count()
    if (buttonCount === 0) {
      test.skip()
      return
    }

    await expect(createButton).toBeVisible()
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()

    // Verify that the Create button is disabled when required fields are empty
    const submitButton = createDialog.getByTestId('channel-submit-button')
    await expect(submitButton).toBeDisabled()

    // Fill in name but leave other required fields empty
    const nameInput = createDialog.getByTestId('channel-name-input')
    await nameInput.fill('Test Channel')

    // Button should still be disabled (missing type, base URL, API key, and models)
    await expect(submitButton).toBeDisabled()

    // Verify validation message for supported models
    await expect(createDialog).toContainText(/Please add at least one supported model|请至少添加一个支持的模型/i)
  })

  test('can navigate between pages', async ({ page }) => {
    const prefix = `pw-channel-pages-${Date.now()}`
    const ids: string[] = []
    try {
      for (let i = 0; i < 11; i++) ids.push(await createFixtureChannel(page, `${prefix}-${i}`))
      await page.reload()
      await page.getByPlaceholder('Search by name', { exact: true }).fill(prefix)
      await page.getByTestId('channels-pagination').getByRole('combobox').click()
      await page.getByRole('option', { name: '10', exact: true }).click()
      const rows = page.getByTestId('channels-table').locator('tbody tr')
      await expect(rows).toHaveCount(10)
      const firstPage = await rows.first().innerText()
      await page.getByRole('button', { name: 'Go to next page', exact: true }).click()
      await expect(rows).toHaveCount(1)
      await expect.poll(() => rows.first().innerText()).not.toBe(firstPage)
      await page.getByRole('button', { name: 'Go to previous page', exact: true }).click()
      await expect(rows).toHaveCount(10)
      await expect.poll(() => rows.first().innerText()).toBe(firstPage)
    } finally {
      if (!page.isClosed()) for (const id of ids) await adminGraphQL(page, 'mutation($id: ID!) { deleteChannel(id: $id) }', { id })
    }
  })

  test('can open model mapping dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for model mapping option - uses i18n key channels.dialogs.settings.modelMapping.title
    const modelMappingOption = menu.getByRole('menuitem', {
      name: /模型映射|Model Mapping|模型别名|Model Alias/i,
    })
    const modelMappingCount = await modelMappingOption.count()

    if (modelMappingCount > 0) {
      await modelMappingOption.click()

      // Verify model mapping dialog opens
      const modelMappingDialog = page.getByRole('dialog')
      await expect(modelMappingDialog).toBeVisible()
      await expect(modelMappingDialog).toContainText(/模型映射|Model Mapping|模型别名|Model Alias/i)
    }
  })

  test('can open override parameters dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for override settings option - uses i18n key channels.dialogs.settings.overrides.action
    const overrideParametersOption = menu.getByRole('menuitem', {
      name: /Overrides|覆盖设置/i,
    })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount > 0) {
      await overrideParametersOption.click()

      // Verify override settings dialog opens
      const overrideParametersDialog = page.getByRole('dialog')
      await expect(overrideParametersDialog).toBeVisible()
      await expect(overrideParametersDialog).toContainText(/Override Settings|覆盖配置|覆盖设置/i)
    }
  })

  test('can bulk import channels', async ({ page }) => {
    // Look for bulk import button
    const bulkImportButton = page.getByRole('button', { name: /Bulk Import|批量导入/i })

    const bulkImportCount = await bulkImportButton.count()
    if (bulkImportCount === 0) {
      test.skip()
      return
    }

    await bulkImportButton.click()

    // Verify bulk import dialog opens
    const bulkImportDialog = page.getByRole('dialog')
    await expect(bulkImportDialog).toBeVisible()
    await expect(bulkImportDialog).toContainText(/Bulk Import|批量导入/i)

    // Close the dialog - use .first() to avoid strict mode violation
    const closeButton = bulkImportDialog.getByRole('button', { name: /取消|Cancel/i }).first()
    if ((await closeButton.count()) > 0) {
      await closeButton.click()
    } else {
      await page.keyboard.press('Escape')
    }
  })

  test('can configure model mappings in model mapping dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    const uniqueSuffix = Date.now().toString().slice(-6)
    const aliasName = `pw-alias-${uniqueSuffix}`

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for model mapping option
    const modelMappingOption = menu.getByRole('menuitem', { name: /模型映射|Model Mapping|模型别名|Model Alias/i })
    const modelMappingCount = await modelMappingOption.count()

    if (modelMappingCount === 0) {
      test.skip()
      return
    }

    await modelMappingOption.click()

    // Verify model mapping dialog opens
    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()
    await expect(settingsDialog).toContainText(/模型映射|Model Mapping|模型别名|Model Alias/i)

    // Look for model mapping section
    const mappingSection = settingsDialog.getByRole('heading', {
      name: /Model Mapping|模型映射|Model Alias|模型别名/i,
    })
    const mappingSectionCount = await mappingSection.count()

    if (mappingSectionCount === 0) {
      test.skip()
      return
    }

    // Add a model mapping
    const originalInput = settingsDialog.getByPlaceholder(/Original Model Name|原模型名称|Alias Name|别名/i)
    
    // Fill original model name
    await originalInput.fill(aliasName)
    await page.waitForTimeout(500)

    // Find and click the target model select (it's a Select component)
    const targetSelectTrigger = settingsDialog.locator('[role="combobox"]').last()
    await targetSelectTrigger.click()
    
    // Wait for dropdown to open
    await page.waitForTimeout(500)
    
    // Select first available option
    const firstOption = page.getByRole('option').first()
    await firstOption.click()
    await page.waitForTimeout(500)

    // Click add button
    const addButton = settingsDialog.getByTestId('add-model-mapping-button')
    await addButton.click()
    
    // Wait for the mapping to be added
    await page.waitForTimeout(1000)

    // Verify mapping appears in the list - look for the text in a border container
    const mappingContainer = settingsDialog.locator('.rounded-lg.border').filter({ hasText: aliasName })
    await expect(mappingContainer).toBeVisible()

    // Save the settings
    const saveButton = settingsDialog.getByRole('button', { name: /保存|Save/i })
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      saveButton.click(),
    ])

    // Wait for dialog to close
    await expect(settingsDialog).not.toBeVisible({ timeout: 10000 })

    // Re-open model mapping dialog to verify the mapping was saved
    await actionsTrigger.click()
    const reopenMenu = page.getByRole('menu')
    await expect(reopenMenu).toBeVisible({ timeout: 5000 })
    await modelMappingOption.click()

    const reopenedDialog = page.getByRole('dialog')
    await expect(reopenedDialog).toBeVisible({ timeout: 5000 })

    // Verify the mapping still exists
    await expect(reopenedDialog).toContainText(aliasName)

    // Close the dialog via keyboard to avoid flaky button detach
    await page.keyboard.press('Escape')
    await expect(reopenedDialog).not.toBeVisible()
  })

  test('can filter channels by tags', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const tagName = `pw-tag-${uniqueSuffix}`

    // Create a channel with a specific tag
    const createButton = page.getByTestId('add-channel-button')
    await expect(createButton).toBeVisible({ timeout: 10000 })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByTestId('channel-name-input').fill(`Channel-${tagName}`)

    // Select provider - use data-testid for reliable selection
    const openaiProviderRadio = createDialog.getByTestId('provider-openai')
    await openaiProviderRadio.click()

    await createDialog.getByTestId('channel-base-url-input').fill('https://api.openai.com/v1')
    await createDialog.getByTestId('channel-api-key-input').fill('sk-test-' + uniqueSuffix)

    // Add model - wait for badge to be visible then click
    const modelBadge = createDialog.getByTestId('quick-model-gpt-4o')
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    await page.waitForTimeout(300)
    
    const addSelectedButton = createDialog.getByTestId('add-selected-models-button')
    await expect(addSelectedButton).toBeEnabled({ timeout: 5000 })
    await addSelectedButton.click()
    await page.waitForTimeout(500)

    // Select Default Test Model
    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
    await expect(defaultTestModelSelect).toBeVisible({ timeout: 5000 })
    await defaultTestModelSelect.click()
    const firstOption = page.getByRole('option').first()
    await firstOption.click()
    await page.waitForTimeout(300)

    await createDialog.getByPlaceholder('Add tags...').fill(tagName)
    await createDialog.getByPlaceholder('Add tags...').press('Enter')
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByTestId('channel-submit-button').click(),
    ])

    await expect(createDialog).not.toBeVisible({ timeout: 10000 })
    await page.waitForTimeout(1000)

    // Newly created channels are disabled by default.
    // If an active status filter excludes disabled channels, include them.
    const statusBtn = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()
    const statusBtnText = await statusBtn.textContent()
    if (statusBtnText && /Enabled|启用/i.test(statusBtnText) && !/Disabled|禁用/i.test(statusBtnText)) {
      await statusBtn.click()
      await page.waitForTimeout(500)
      const disabledOpt = page
        .getByRole('option', { name: /Disabled|禁用/i })
        .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))
      if ((await disabledOpt.count()) > 0) {
        await disabledOpt.first().click()
        await page.waitForTimeout(500)
      }
      await page.keyboard.press('Escape')
      await page.waitForTimeout(500)
    }

    // Look for tags filter button
    const tagsFilterButton = page
      .locator('button')
      .filter({ hasText: /Tags|标签/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()

    await tagsFilterButton.click()
    await page.getByRole('option', { name: new RegExp(tagName) }).click()
    await page.keyboard.press('Escape')
    const filteredRows = page.getByTestId('channels-table').locator('tbody tr')
    await expect(filteredRows).toHaveCount(1)
    await expect(filteredRows.first()).toContainText(`Channel-${tagName}`)
  })
})
