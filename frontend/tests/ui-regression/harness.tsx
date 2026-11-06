import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ChannelsActionDialog } from '@/features/channels/components/channels-action-dialog';
import { ChannelsEndpointsDialog } from '@/features/channels/components/channels-endpoints-dialog';
import { ChannelsModelApiFormatPolicyDialog } from '@/features/channels/components/channels-model-api-format-policy-dialog';
import ChannelsProvider from '@/features/channels/context/channels-context';
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
import { DimensionPieCharts } from '@/features/analytics/components/dimension-pie-charts';
import { channel, legacyCodexChannel, profile, PROJECT_ID, user } from './fixtures';

useAuthStore.getState().auth.setUser(user);
useAuthStore.getState().auth.setAccessToken('ui-fixture-token');
useProjectStore.getState().setSelectedProjectId(PROJECT_ID);
await i18n.changeLanguage('en');
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false }, mutations: { retry: false } } });
const scenario = new URLSearchParams(location.search).get('case');
const currentChannel = channelSchema.parse(scenario === 'codex-endpoints' || scenario === 'legacy-policy' ? legacyCodexChannel : channel);

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

function Harness() {
  const [open, setOpen] = useState(true);
  const [submitted, setSubmitted] = useState<unknown>(null);
  return <>
    <div data-testid='harness-ready'>Isolated UI fixture</div>
    <button data-testid='reopen-dialog' onClick={() => setOpen(true)}>Reopen</button>
    <output data-testid='dialog-open'>{String(open)}</output>
    <output data-testid='profile-submitted'>{JSON.stringify(submitted)}</output>
    <output data-testid='currency-format'>{i18n.t('currencies.format', { val: 1234.5, currencyCode: 'USD', currency: 'USD', locale: 'en-US', minimumFractionDigits: 2, maximumFractionDigits: 2 })}</output>
    {(scenario === 'endpoints' || scenario === 'codex-endpoints') && <ChannelsEndpointsDialog channel={currentChannel} open={open} onOpenChange={setOpen} />}
    {(scenario === 'policy' || scenario === 'legacy-policy') && <ChannelsModelApiFormatPolicyDialog currentRow={currentChannel} open={open} onOpenChange={setOpen} />}
    {scenario === 'channel-edit' && <ChannelsActionDialog currentRow={currentChannel} open={open} onOpenChange={setOpen} />}
    {scenario === 'profiles' && <ApiKeyProfilesDialog open={open} onOpenChange={setOpen} initialData={{ activeProfile: profile.name, profiles: [apiKeyProfileSchema.parse(profile)] }} onSubmit={(value) => { setSubmitted(value); setOpen(false); }} />}
    {scenario === 'template' && <ApiKeyCreateTemplateDialog open={open} onOpenChange={setOpen} />}
    {scenario === 'schemas' && <output data-testid='schema-regression'>{JSON.stringify(schemaResults)}</output>}
    {scenario === 'analytics' && <div className='p-6 space-y-6'>
      <div data-testid='trend-chart'><CombinedTrendChart data={daily} isLoading={false} currencyCode='USD' /></div>
      <div data-testid='pie-charts'><DimensionPieCharts channelStats={dimensions} modelStats={dimensions} apiKeyStats={dimensions} userStats={dimensions} isLoading={false} currencyCode='USD' /></div>
    </div>}
    <Toaster />
  </>;
}

createRoot(document.getElementById('root')!).render(
  <ThemeProvider><QueryClientProvider client={queryClient}><ChannelsProvider><ApiKeysProvider><Harness /></ApiKeysProvider></ChannelsProvider></QueryClientProvider></ThemeProvider>
);
