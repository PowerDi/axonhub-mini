import type { ApiFormat, ChannelType } from './schema';

type ProtocolEndpoint = {
  readonly apiFormat?: string | null;
};

export function getAvailableProtocolFormats(
  defaultEndpoints: readonly ProtocolEndpoint[],
  endpoints: readonly ProtocolEndpoint[]
): string[] {
  const formats = new Set<string>();

  for (const endpoint of [...defaultEndpoints, ...endpoints]) {
    if (typeof endpoint.apiFormat === 'string' && endpoint.apiFormat.length > 0) {
      formats.add(endpoint.apiFormat);
    }
  }

  return Array.from(formats);
}

type ProviderConfig = {
  readonly channelTypes: readonly ChannelType[];
};

type ChannelConfig = {
  readonly apiFormat?: ApiFormat | null;
};

export type ProtocolConfigs = {
  readonly providerConfigs: Readonly<Record<string, ProviderConfig>>;
  readonly channelConfigs: Readonly<Record<string, ChannelConfig>>;
};

export function getApiFormatsForProvider(provider: string, configs: ProtocolConfigs): ApiFormat[] {
  const providerConfig = configs.providerConfigs[provider];
  if (!providerConfig) return [];

  const formats: ApiFormat[] = [];
  for (const channelType of providerConfig.channelTypes) {
    const apiFormat = configs.channelConfigs[channelType]?.apiFormat;
    if (apiFormat && !formats.includes(apiFormat)) {
      formats.push(apiFormat);
    }
  }

  // Native video is a ZenMux channel type, so it is only offered when the
  // provider actually includes the ZenMux channel type in its config.
  if (providerConfig.channelTypes.includes('zenmux') && !formats.includes('zenmux/video')) {
    formats.push('zenmux/video');
  }

  return formats;
}

// Every channel type that belongs to the ZenMux family. The backend accepts the
// native video api_format on all of them, so the endpoints dialog must offer it
// everywhere in the family and nowhere else.
const ZENMUX_CHANNEL_TYPES: readonly ChannelType[] = [
  'zenmux',
  'zenmux_responses',
  'zenmux_anthropic',
  'zenmux_gemini',
  'zenmux_video',
];

/**
 * Custom endpoint formats the endpoints dialog may offer for a channel.
 * Native video is only offered inside the ZenMux family; the SystemOne endpoint is
 * exclusive to the TypeSafe channel type, and OpenAI Decisions is opt-in for the
 * OpenAI channel only (the backend still accepts manual configuration elsewhere).
 * Other custom endpoint formats remain available everywhere.
 */
export function getConfigurableApiFormatsForChannelType(
  channelType: ChannelType,
  configurableFormats: readonly string[]
): string[] {
  if (channelType === 'typesafe') {
    return ['typesafe/systemone'];
  }

  const filtered = configurableFormats
    .filter((format) => format !== 'typesafe/systemone')
    .filter((format) => channelType === 'openai' || format !== 'openai/decisions');

  if (ZENMUX_CHANNEL_TYPES.includes(channelType)) {
    return [...filtered];
  }

  return filtered.filter((format) => format !== 'zenmux/video');
}

export function getChannelTypeForApiFormat(provider: string, apiFormat: ApiFormat, configs: ProtocolConfigs): ChannelType | undefined {
  const providerConfig = configs.providerConfigs[provider];
  if (!providerConfig) return undefined;

  // Native video is a ZenMux channel type, so it is only mappable back to a
  // channel type when the provider config actually includes the ZenMux
  // channel type.
  if (apiFormat === 'zenmux/video') {
    return providerConfig.channelTypes.includes('zenmux') ? 'zenmux' : undefined;
  }

  for (const channelType of providerConfig.channelTypes) {
    if (configs.channelConfigs[channelType]?.apiFormat === apiFormat) {
      return channelType;
    }
  }

  return undefined;
}

