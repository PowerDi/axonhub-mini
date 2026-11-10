import { expect, type Page } from '@playwright/test';
import { channel, channelForScenario, pageInfo, user } from './fixtures';

type Operation = { operationName: string; query: string; variables: Record<string, any>; projectHeader?: string };

export async function openFixture(page: Page, scenario: string) {
  const operations: Operation[] = [];
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  const snapshot = structuredClone(channelForScenario(scenario));
  await page.route('**/admin/graphql', async (route) => {
    const payload = route.request().postDataJSON() as Operation;
    // Channel operations include a fragment before the operation definition.
    const op = { ...payload, operationName: payload.operationName ?? /\b(?:query|mutation)\s+(\w+)/.exec(payload.query)?.[1] ?? 'unnamed' };
    operations.push({ ...op, projectHeader: route.request().headers()['x-project-id'] });
    let data: Record<string, unknown>;
    switch (op.operationName) {
      case 'SystemGeneralSettings': data = { systemGeneralSettings: { currencyCode: 'USD', timezone: 'UTC' } }; break;
      case 'PluginUIEntries': data = { pluginUIEntries: {
        pages: [{ pluginID: 'example.fixture', key: 'notes', entry: 'ui/notes.html', title: 'Fixture plugin page' }],
        actions: [{ pluginID: 'example.fixture', key: 'notes', slot: 'app.header.actions', title: 'Fixture plugin tool', icon: 'puzzle', revision: '1', effect: { type: 'dialog', entry: 'ui/notes.html' } }],
      } }; break;
      case 'BrandSettings': data = { brandSettings: { brandName: 'AxonHub', brandLogo: null, title: 'AxonHub' } }; break;
      case 'SystemVersion': data = { systemVersion: { version: 'v1.0.0-rc2', commit: 'fixture', buildTime: '', goVersion: '', platform: '', uptime: 1 } }; break;
      case 'CheckForUpdate': data = { checkForUpdate: { currentVersion: 'v1.0.0-rc2', latestVersion: 'v1.0.0-rc2', hasUpdate: false, releaseUrl: '' } }; break;
      case 'OnboardingInfo': data = { onboardingInfo: { onboarded: true, systemModelSetting: { onboarded: true }, autoDisableChannel: { onboarded: true } } }; break;
      case 'MyProjects': data = { myProjects: [] }; break;
      case 'ProviderQuotaStatuses': data = { queryChannels: { edges: [], pageInfo, totalCount: 0 } }; break;
      case 'PreviewChannelModelEntries': {
        const { supportedModels, settings } = op.variables.input;
        data = { previewChannelModelEntries: supportedModels.flatMap((model: string) => [
          ...(settings.hideOriginalModels ? [] : [{ requestModel: model, actualModel: model, source: 'direct' }]),
          ...(settings.extraModelPrefix ? [{ requestModel: `${settings.extraModelPrefix}/${model}`, actualModel: model, source: 'prefix' }] : []),
        ]) };
        break;
      }
      case 'Me': data = { me: user }; break;
      case 'Models': data = { queryModels: [{ id: 'gpt-4o', status: 'enabled' }] }; break;
      case 'GetAllChannelSummarys': data = { allChannelSummarys: [channel] }; break;
      case 'ApiKeyProfileTemplates': data = { apiKeyProfileTemplates: { edges: [], pageInfo, totalCount: 0 } }; break;
      case 'AllChannelTags': data = { allChannelTags: ['ui-fixture'] }; break;
      case 'ProxyPresets': data = { proxyPresets: [] }; break;
      case 'QuotaRoutingSettings': data = { quotaRoutingSettings: { defaultMode: 'IGNORE_QUOTA' } }; break;
      case 'UserAgentPassThroughSettings': data = { userAgentPassThroughSettings: { enabled: false } }; break;
      case 'PassThroughSettings': data = { passThroughSettings: { enabled: false } }; break;
      case 'RetryPolicy': data = { retryPolicy: { enabled: true, maxChannelRetries: 2, maxSingleChannelRetries: 1, retryDelayMs: 0, streamFirstEventTimeoutSeconds: 10, nonStreamResponseTimeoutSeconds: 30 } }; break;
      case 'QueryChannelNames':
      case 'QueryChannels': data = { queryChannels: { edges: [{ node: snapshot, cursor: '101' }], pageInfo, totalCount: 1 } }; break;
      case 'DuplicateChannel': data = { duplicateChannel: { ...snapshot, ...op.variables.input } }; break;
      case 'UpdateChannel': data = { updateChannel: { ...snapshot, ...op.variables.input } }; break;
      case 'UpdateChannelSettings': data = { updateChannelSettings: { ...snapshot, settings: { ...snapshot.settings, ...op.variables.input } } }; break;
      case 'CreateApiKeyProfileTemplate': data = { createApiKeyProfileTemplate: { id: 'gid://axonhub/APIKeyProfileTemplate/88', ...op.variables.input, profile: op.variables.profile } }; break;
      default: errors.push(`Unexpected GraphQL operation: ${op.operationName}`); data = {};
    }
    await route.fulfill({ json: { data } });
  });
  await page.goto(`/tests/ui-regression/index.html?case=${scenario}`);
  await expect(page.getByTestId('harness-ready')).toBeVisible();
  if (scenario !== 'schemas' && scenario !== 'analytics' && scenario !== 'channels-table' && scenario !== 'native-ui') await expect(page.getByRole('dialog')).toBeVisible();
  return { operations, errors };
}
