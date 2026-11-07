export const PROJECT_ID = 'gid://axonhub/Project/1';
export const user = {
  id: 'gid://axonhub/User/1', email: 'my@example.com', firstName: 'UI', lastName: 'Fixture',
  isOwner: true, preferLanguage: 'en', scopes: ['*'], roles: [],
  projects: [{ projectID: PROJECT_ID, isOwner: true, scopes: ['*'], roles: [] }],
};
export const channel = {
  id: 'gid://axonhub/Channel/101', type: 'openai_multiprotocol', name: 'UI multi-endpoint',
  createdAt: '2026-11-06T00:00:00Z', updatedAt: '2026-11-06T00:00:00Z',
  baseURL: 'https://relay.example.test/v1', status: 'enabled',
  credentials: { apiKeys: ['ui-fixture-key-a', 'ui-fixture-key-b'] },
  supportedModels: ['gpt-4o'], manualModels: ['gpt-4o'], defaultTestModel: 'gpt-4o', tags: ['ui-fixture'], orderingWeight: 0,
  policies: { stream: 'unlimited' }, allModelEntries: [{ requestModel: 'gpt-4o', actualModel: 'gpt-4o', source: 'direct' }],
  endpoints: [{ apiFormat: 'openai/responses', baseURL: 'https://responses.example.test/v1', path: '/responses' }],
  defaultEndpoints: [{ apiFormat: 'openai/chat_completions' }, { apiFormat: 'openai/responses' }, { apiFormat: 'anthropic/messages' }],
  settings: {
    modelApiFormatPolicies: [{ model: 'gpt-4o', allow: ['openai/responses'], exclude: [] }],
    hammerRetry: { retryDelayMs: 500, maxRetries: 4 }, reasoningFilter: true,
    maxSingleChannelRetries: 2,
    responseTimeout: { mode: 'CUSTOM', streamFirstEventTimeoutSeconds: 12, nonStreamResponseTimeoutSeconds: 45 },
    apiKeyStrategy: 'round_robin_success', apiKeyRoundRobinSwitchAfter: 3,
  },
};
export const legacyCodexChannel = {
  ...channel, type: 'codex', name: 'UI legacy WS', baseURL: 'wss://codex.example.test/v1',
  endpoints: [{ apiFormat: 'openai/responses', transport: 'websocket' }],
  defaultEndpoints: [{ apiFormat: 'openai/responses-ws' }],
  settings: {
    ...channel.settings,
    modelApiFormatPolicies: [
      { model: 'gpt-4o', allow: ['openai/responses'], exclude: [] },
      { model: 'deny-model', allow: [], exclude: ['openai/responses'] },
    ],
  },
};
export const profile = {
  name: 'Linked exclude', templateID: 51, templateName: 'Shared exclusion', modelMappings: [],
  channelIDs: [101], channelIDsMatchMode: 'exclude', channelTags: [], channelTagsMatchMode: 'any',
  modelIDs: [], loadBalanceStrategy: 'default', traceStickyMode: 'default',
};
export const pageInfo = { hasNextPage: false, hasPreviousPage: false, startCursor: null, endCursor: null };

export function channelForScenario(scenario: string | null) {
  if (scenario === 'codex-endpoints' || scenario === 'legacy-policy') return legacyCodexChannel;
  if (scenario === 'non-codex-ws-endpoints') return {
    ...channel, type: 'openai_responses', name: 'Legacy Responses WS', baseURL: 'wss://api.openai.com/v1#', endpoints: [],
    defaultEndpoints: [{ apiFormat: 'openai/responses-ws' }],
  };
  if (scenario === 'channel-duplicate-gcp') return {
    ...channel, type: 'anthropic_gcp', name: 'GCP source', baseURL: 'https://us-east5-aiplatform.googleapis.com',
    credentials: { gcp: { region: 'us-east5', projectID: 'ui-project', jsonData: '{"type":"service_account"}' } },
    defaultEndpoints: [{ apiFormat: 'anthropic/messages' }], endpoints: [], settings: {},
  };
  return channel;
}
