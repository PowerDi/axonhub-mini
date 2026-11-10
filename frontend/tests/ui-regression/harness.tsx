import { AuthenticatedLayout } from '@/authenticated-layout';
import { FontProvider } from '@/context/font-context';
import { ChannelsModelMappingDialog } from '@/features/channels/components/channels-model-mapping-dialog';
import { Button } from '@/components/ui/button';
import { createRootRoute, createRouter, createMemoryHistory, RouterProvider } from '@tanstack/react-router';
import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ChannelsActionDialog } from '@/features/channels/components/channels-action-dialog';
import { ChannelsEndpointsDialog } from '@/features/channels/components/channels-endpoints-dialog';
import { ChannelsModelApiFormatPolicyDialog } from '@/features/channels/components/channels-model-api-format-policy-dialog';
import ChannelsProvider from '@/features/channels/context/channels-context';
import { ChannelsTable } from '@/features/channels/components/channels-table';
import { createColumns } from '@/features/channels/components/channels-columns';
import { DEFAULT_CHANNEL_COLUMN_VISIBILITY } from '@/features/channels/data/channels';
import { channelSchema } from '@/features/channels/data/schema';
import { ApiKeyProfilesDialog } from '@/features/apikeys/components/apikeys-profiles-dialog';
import { ApiKeyCreateTemplateDialog } from '@/features/apikeys/components/apikeys-create-template-dialog';
import { ApiKeysProvider } from '@/features/apikeys/context/apikeys-context';
import { apiKeyProfileSchema, updateApiKeyProfilesInputSchema, updateApiKeyProfilesInputSchemaFactory } from '@/features/apikeys/data/schema';
import { useAuthStore } from '@/stores/authStore';
import { useProjectStore } from '@/stores/projectStore';
import { ThemeProvider } from '@/context/theme-context';
import { Toaster } from '@/components/ui/sonner';
import i18n from '@/lib/i18n';
import '@/index.css';
import { CombinedTrendChart } from '@/features/analytics/components/combined-trend-chart';
import { DimensionDetailTable } from '@/features/analytics/components/dimension-detail-table';
import { DimensionPieCharts } from '@/features/analytics/components/dimension-pie-charts';
import { channelForScenario, profile, PROJECT_ID, user } from './fixtures';

useAuthStore.getState().auth.setUser(user);
useAuthStore.getState().auth.setAccessToken('ui-fixture-token');
useProjectStore.getState().setSelectedProjectId(PROJECT_ID);
await i18n.changeLanguage('en');
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false }, mutations: { retry: false } } });
const scenario = new URLSearchParams(location.search).get('case');
const currentChannel = channelSchema.parse(channelForScenario(scenario));

const schemaInputs = [null, '', undefined, 'include', 'exclude'].map((channelIDsMatchMode) => ({
  activeProfile: profile.name, profiles: [{ ...profile, channelIDsMatchMode, channelTagsMatchMode: null }],
}));
const schemas = [updateApiKeyProfilesInputSchema, updateApiKeyProfilesInputSchemaFactory((key) => key)];
const schemaResults = schemas.map((schema) => ({
  normalized: schemaInputs.map((input) => schema.parse(input)),
  invalidAccepted: schema.safeParse({ activeProfile: profile.name, profiles: [{ ...profile, channelIDsMatchMode: 'typo' }] }).success,
}));
const daily = [{ date: '2026-11-06', inputTokens: 120, cachedInputTokens: 20, uncachedInputTokens: 100, outputTokens: 40, totalTokens: 160, requestCount: 3, cost: 1.25 }];
const dimensions = [{ id: 'fixture', name: 'Fixture channel', requestCount: 3, inputTokens: 120, cachedInputTokens: 20, outputTokens: 40, totalTokens: 160, cost: 1.25, tokensPerSecond: null, ttftMs: null }];

function ChannelTableFixture() {
  const [visibility, setVisibility] = useState(DEFAULT_CHANNEL_COLUMN_VISIBILITY);
  const noop = () => {};
  return <div className='p-4 md:ml-[232px]' data-testid='channel-page-content'>
    <ChannelsTable columns={createColumns(i18n.t)} data={[currentChannel]} pageSize={10} totalCount={1}
      nameFilter='' typeFilter={[]} statusFilter={[]} tagFilter='' modelFilter='' sorting={[]}
      onSortingChange={noop} onNextPage={noop} onPreviousPage={noop} onPageSizeChange={noop}
      onNameFilterChange={noop} onTypeFilterChange={noop} onStatusFilterChange={noop} onTagFilterChange={noop}
      onModelFilterChange={noop} columnVisibility={visibility} onColumnVisibilityChange={setVisibility} />
  </div>;
}

function NativeUIFixture() {
  const [panel, setPanel] = useState<'aliases' | 'channel' | null>(null);
  return <AuthenticatedLayout>
    <main className='p-6'>
      <h1>Native interface acceptance</h1>
      <div className='flex gap-3 py-4'>
        <Button data-testid='open-alias-settings' onClick={() => setPanel('aliases')}>Model aliases</Button>
        <Button data-testid='open-channel-settings' onClick={() => setPanel('channel')}>Channel settings</Button>
      </div>
      <ChannelsModelMappingDialog currentRow={currentChannel} open={panel === 'aliases'} onOpenChange={(open) => { if (!open) setPanel(null); }} />
      <ChannelsActionDialog currentRow={currentChannel} open={panel === 'channel'} onOpenChange={(open) => { if (!open) setPanel(null); }} />
    </main>
  </AuthenticatedLayout>;
}

function Harness() {
  const [open, setOpen] = useState(true);
  const [submitted, setSubmitted] = useState<unknown>(null);
  return <>
    <div data-testid='harness-ready'>Isolated UI fixture</div>
    <button data-testid='reopen-dialog' onClick={() => setOpen(true)}>Reopen</button>
    <output data-testid='dialog-open'>{String(open)}</output>
    <output data-testid='profile-submitted'>{JSON.stringify(submitted)}</output>
    <output data-testid='currency-format'>{i18n.t('currencies.format', { val: 1234.5, currencyCode: 'USD', currency: 'USD', locale: 'en-US', minimumFractionDigits: 2, maximumFractionDigits: 2 })}</output>
    {(scenario === 'endpoints' || scenario === 'codex-endpoints' || scenario === 'non-codex-ws-endpoints') && <ChannelsEndpointsDialog channel={currentChannel} open={open} onOpenChange={setOpen} />}
    {(scenario === 'policy' || scenario === 'legacy-policy') && <ChannelsModelApiFormatPolicyDialog currentRow={currentChannel} open={open} onOpenChange={setOpen} />}
    {scenario === 'channel-duplicate-gcp' && <ChannelsActionDialog duplicateFromRow={currentChannel} open={open} onOpenChange={setOpen} />}
    {scenario === 'channel-edit' && <ChannelsActionDialog currentRow={currentChannel} open={open} onOpenChange={setOpen} />}
    {scenario === 'profiles' && <ApiKeyProfilesDialog open={open} onOpenChange={setOpen} initialData={{ activeProfile: profile.name, profiles: [apiKeyProfileSchema.parse(profile)] }} onSubmit={(value) => { setSubmitted(value); setOpen(false); }} />}
    {scenario === 'template' && <ApiKeyCreateTemplateDialog open={open} onOpenChange={setOpen} />}
    {scenario === 'native-ui' && <NativeUIFixture />}
    {scenario === 'channels-table' && <ChannelTableFixture />}
    {scenario === 'schemas' && <output data-testid='schema-regression'>{JSON.stringify(schemaResults)}</output>}
    {scenario === 'analytics' && <div className='p-6 space-y-6'>
      <div data-testid='trend-chart'><CombinedTrendChart data={daily} isLoading={false} currencyCode='USD' /></div>
      <div data-testid='dimension-details'><DimensionDetailTable channelStats={dimensions} modelStats={dimensions} apiKeyStats={dimensions} userStats={dimensions} isLoading={false} /></div>
      <div data-testid='pie-charts'><DimensionPieCharts channelStats={dimensions} modelStats={dimensions} apiKeyStats={dimensions} userStats={dimensions} isLoading={false} currencyCode='USD' /></div>
    </div>}
    <Toaster />
  </>;
}

const router = createRouter({ routeTree: createRootRoute({ component: Harness }), history: createMemoryHistory({ initialEntries: ['/'] }) });

createRoot(document.getElementById('root')!).render(
  <ThemeProvider><FontProvider><QueryClientProvider client={queryClient}><ChannelsProvider><ApiKeysProvider><RouterProvider router={router} /></ApiKeysProvider></ChannelsProvider></QueryClientProvider></FontProvider></ThemeProvider>
);
