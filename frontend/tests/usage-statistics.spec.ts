import { createServer } from 'node:http'
import { test, expect } from '@playwright/test'
import { extractNumberID } from '../src/lib/utils'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

// Only the external provider is simulated. Project/key creation, gateway routing,
// usage persistence, GraphQL aggregation and the page all use the real backend.
test('usage statistics reflect a real gateway request and stay project-scoped', async ({ page }) => {
  test.setTimeout(60000)
  await gotoAndEnsureAuth(page, '/')
  const token = await page.evaluate(() => localStorage.getItem('axonhub_access_token'))
  async function graphql(query: string, variables: Record<string, unknown> = {}, projectID?: string) {
    const response = await page.request.post('/admin/graphql', {
      headers: { Authorization: `Bearer ${token}`, ...(projectID ? { 'X-Project-ID': projectID } : {}) },
      data: { query, variables },
    })
    expect(response.ok()).toBe(true)
    const body = await response.json()
    expect(body.errors).toBeUndefined()
    return body.data
  }

  const name = `pw-usage-${Date.now()}`
  const model = `${name}-model`
  const received: { path?: string; model: string }[] = []
  const provider = createServer(async (request, response) => {
    let body = ''
    for await (const chunk of request) body += chunk
    received.push({ path: request.url, model: JSON.parse(body).model })
    response.writeHead(200, { 'Content-Type': 'application/json' })
    response.end(JSON.stringify({
      id: 'chatcmpl-e2e', object: 'chat.completion', created: Math.floor(Date.now() / 1000), model,
      choices: [{ index: 0, message: { role: 'assistant', content: 'E2E gateway response' }, finish_reason: 'stop' }],
      usage: { prompt_tokens: 11, completion_tokens: 7, total_tokens: 18 },
    }))
  })
  await new Promise<void>((resolve) => provider.listen(0, '127.0.0.1', resolve))
  const port = (provider.address() as { port: number }).port
  let projectID: string | undefined
  let emptyProjectID: string | undefined
  let channelID: string | undefined
  let modelID: string | undefined
  try {
    const project = await graphql('mutation($input: CreateProjectInput!) { createProject(input: $input) { id } }', { input: { name } })
    projectID = project.createProject.id
    const empty = await graphql('mutation($input: CreateProjectInput!) { createProject(input: $input) { id } }', { input: { name: `${name}-empty` } })
    emptyProjectID = empty.createProject.id
    const channel = await graphql('mutation($input: CreateChannelInput!) { createChannel(input: $input) { id } }', {
      input: { name, type: 'openai', baseURL: `http://127.0.0.1:${port}/v1`, supportedModels: [model], defaultTestModel: model, credentials: { apiKeys: ['e2e-provider-key'] } },
    })
    channelID = channel.createChannel.id
    await graphql('mutation($id: ID!) { updateChannelStatus(id: $id, status: enabled) { id } }', { id: channelID })
    const configuredModel = await graphql('mutation($input: CreateModelInput!) { createModel(input: $input) { id } }', {
      input: { developer: 'openai', modelID: model, name, icon: 'OpenAI', group: 'e2e', modelCard: {},
        settings: { associations: [{ type: 'channel_model', channelModel: { channelId: Number(extractNumberID(channelID!)), modelId: model } }] } },
    })
    modelID = configuredModel.createModel.id
    await graphql('mutation($id: ID!) { updateModel(id: $id, input: { status: enabled }) { id } }', { id: modelID })
    const key = await graphql('mutation($input: CreateAPIKeyInput!) { createAPIKey(input: $input) { key } }', { input: { name, type: 'personal', projectID } }, projectID)

    // Channel activation refreshes the routing snapshot asynchronously.
    await expect.poll(async () => {
      const response = await page.request.get('/v1/models', { headers: { Authorization: `Bearer ${key.createAPIKey.key}` } })
      expect(response.ok()).toBe(true)
      return (await response.json()).data.map((item: { id: string }) => item.id)
    }, { timeout: 15000 }).toContain(model)

    const completion = await page.request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${key.createAPIKey.key}` },
      data: { model, messages: [{ role: 'user', content: 'test usage accounting' }], stream: false },
    })
    expect(completion.status(), await completion.text()).toBe(200)
    expect((await completion.json()).choices[0].message.content).toBe('E2E gateway response')
    expect(received).toEqual([{ path: '/v1/chat/completions', model }])

    await expect.poll(async () => {
      const result = await graphql('query { usageStatsByUser { requestCount totalTokens } }', {}, projectID)
      return result.usageStatsByUser
    }, { timeout: 15000 }).toEqual([{ requestCount: 1, totalTokens: 18 }])

    await page.reload()
    await page.getByRole('button', { name: 'Default', exact: true }).click()
    await page.getByRole('menuitem', { name, exact: true }).click()
    await expect(page.getByRole('button', { name, exact: true })).toBeVisible()
    const [stats] = await Promise.all([
      waitForGraphQLOperation(page, 'GetUsageStatsByUser'),
      page.getByRole('link', { name: 'Usage Statistics', exact: true }).click(),
    ])
    await expect(page).toHaveURL(/\/project\/usage-stats$/)
    await expect(page.getByRole('heading', { name: 'Usage Statistics' })).toBeVisible()
    expect(stats.usageStatsByUser).toHaveLength(1)
    const row = page.getByRole('row').filter({ hasText: stats.usageStatsByUser[0].userName })
    await expect(row.getByRole('cell').nth(2)).toHaveText('1')
    await expect(row.getByRole('cell').nth(3)).toHaveText('18')

    await page.getByRole('searchbox').fill('no-such-usage-user')
    await expect(page.getByText('No results.', { exact: true })).toBeVisible()
    await expect(row).toBeHidden()
    await page.getByRole('searchbox').clear()
    await expect(row).toBeVisible()

    // Reload in another project: aggregate data must not leak across the boundary.
    await page.getByRole('button', { name, exact: true }).click()
    await page.getByRole('menuitem', { name: `${name}-empty`, exact: true }).click()
    await expect(page.getByText('No user usage data available', { exact: true })).toBeVisible()
    await expect(page.getByRole('table')).toBeHidden()
  } finally {
    await new Promise<void>((resolve, reject) => provider.close((error) => error ? reject(error) : resolve()))
    if (modelID) await graphql('mutation($id: ID!) { deleteModel(id: $id) }', { id: modelID })
    if (channelID) await graphql('mutation($id: ID!) { deleteChannel(id: $id) }', { id: channelID })
    for (const id of [projectID, emptyProjectID]) {
      if (id) await graphql('mutation($id: ID!) { deleteProject(id: $id) }', { id })
    }
  }
})
