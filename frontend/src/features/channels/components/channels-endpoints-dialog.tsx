'use client';

import { useState, useMemo, useCallback, useEffect, useRef } from 'react';
import { Google } from '@lobehub/icons';
import { AlertCircle, Loader2, Plus, Radar, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useDetectChannelEndpoints, useUpdateChannelSettings } from '../data/channels';
import {
  buildCodexResponsesEndpoints,
  getCodexResponsesWebSocketState,
  getCodexResponsesWebSocketURL,
  hasCodexResponsesWebSocketOverrideReplacement,
  getConfigurableApiFormatsForChannelType,
  isResponsesFamilyFormat,
  toHttpBaseURL,
  translateLegacyCodexModelPolicies,
} from '../data/protocol-options';
import { RELAY_PROTOCOL_LABEL_KEYS, getChannelRelayProtocols, isRelayProtocol, type RelayProtocol } from '../data/relay-protocols';
import {
  Channel,
  ChannelEndpoint,
  DetectedChannelEndpoint,
  channelEndpointSchema,
  configurableChannelEndpointApiFormats,
  configurableChannelEndpointApiFormatSchema,
} from '../data/schema';
import { ChatProtocolIcon, MessagesProtocolIcon, ResponsesProtocolIcon } from './endpoint-protocol-icons';

const RELAY_PROTOCOL_ICONS: Record<RelayProtocol, React.ComponentType<{ size?: number | string; className?: string }>> = {
  'openai/chat_completions': ChatProtocolIcon,
  'openai/responses': ResponsesProtocolIcon,
  'anthropic/messages': MessagesProtocolIcon,
  'gemini/contents': Google,
};

const RELAY_PROTOCOL_ACTIVE_CLASSES: Record<RelayProtocol, string> = {
  'openai/chat_completions': 'border-sky-500/30 bg-sky-500/15 text-sky-600 dark:text-sky-400',
  'openai/responses': 'border-violet-500/30 bg-violet-500/15 text-violet-600 dark:text-violet-400',
  'anthropic/messages': 'border-amber-500/30 bg-amber-500/15 text-amber-600 dark:text-amber-400',
  'gemini/contents': 'border-emerald-500/30 bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
};

const DETECTION_REASON_KEYS: Record<string, string> = {
  supported: 'channels.endpoints.detect.reasons.supported',
  not_found: 'channels.endpoints.detect.reasons.notFound',
  auth_error: 'channels.endpoints.detect.reasons.authError',
  rate_limited: 'channels.endpoints.detect.reasons.rateLimited',
  server_error: 'channels.endpoints.detect.reasons.serverError',
  unreachable: 'channels.endpoints.detect.reasons.unreachable',
  invalid_request: 'channels.endpoints.detect.reasons.invalidRequest',
};

function DetectionResultCard({ result }: { result: DetectedChannelEndpoint }) {
  const { t } = useTranslation();

  if (!isRelayProtocol(result.apiFormat)) {
    return null;
  }

  const Icon = RELAY_PROTOCOL_ICONS[result.apiFormat];
  const label = t(RELAY_PROTOCOL_LABEL_KEYS[result.apiFormat]);
  const reasonKey = DETECTION_REASON_KEYS[result.reason] ?? DETECTION_REASON_KEYS.unreachable;

  return (
    <div
      className={cn(
        'flex min-w-0 items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs',
        result.supported ? RELAY_PROTOCOL_ACTIVE_CLASSES[result.apiFormat] : 'border-border/60 bg-muted/40 text-muted-foreground'
      )}
    >
      <Icon size={14} />
      <span className='truncate font-medium'>{label}</span>
      <span className='text-muted-foreground shrink-0'>
        {result.supported ? t('channels.endpoints.detect.supported') : t(reasonKey)}
        {!result.supported && result.statusCode ? ` (${result.statusCode})` : ''}
      </span>
    </div>
  );
}

interface Props {
  channel: Channel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const ALLOWED_API_FORMATS = configurableChannelEndpointApiFormatSchema.options;
const OPENAI_RESPONSES_WEBSOCKET = 'openai/responses-ws';
const RECOGNIZED_API_FORMATS: readonly string[] = [...ALLOWED_API_FORMATS, OPENAI_RESPONSES_WEBSOCKET];

function EndpointTable({
  endpoints,
  readOnly,
  hideBaseURL,
  children,
}: {
  endpoints: ChannelEndpoint[];
  readOnly?: boolean;
  hideBaseURL?: boolean;
  children?: (ep: ChannelEndpoint, index: number) => React.ReactNode;
}) {
  const { t } = useTranslation();
  const gridCols = hideBaseURL ? 'grid-cols-[1fr_1fr_auto]' : 'grid-cols-[1fr_1fr_1fr_auto]';
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className={`bg-muted/50 text-muted-foreground ${gridCols} gap-2 border-b px-3 py-2 text-xs font-medium`}>
        <span>{t('channels.endpoints.apiFormat')}</span>
        {!hideBaseURL && <span>{t('channels.endpoints.baseURL')}</span>}
        <span>{t('channels.endpoints.path')}</span>
        <span className='w-8' />
      </div>
      <div className='divide-y'>
        {endpoints.map((ep, index) => (
          <div
            key={`${ep.apiFormat}-${index}`}
            className={`hover:bg-muted/30 ${gridCols} items-center gap-2 px-3 py-2.5 text-sm transition-colors`}
          >
            <div className='flex items-center gap-2'>
              <Badge variant='secondary' className='w-fit font-mono text-xs'>
                {ep.apiFormat}
              </Badge>
              {readOnly && index === 0 && (
                <Badge variant='outline' className='text-[10px]'>
                  {t('channels.endpoints.primaryBadge')}
                </Badge>
              )}
              {!readOnly && !RECOGNIZED_API_FORMATS.includes(ep.apiFormat) && (
                <Badge variant='destructive' className='text-[10px]'>
                  {t('channels.endpoints.unsupportedBadge')}
                </Badge>
              )}
            </div>
            {!hideBaseURL && <span className='text-muted-foreground truncate font-mono text-xs'>{ep.baseURL || '-'}</span>}
            <span className='text-muted-foreground truncate font-mono text-xs'>{ep.path || '-'}</span>
            {readOnly ? (
              <span className='text-muted-foreground text-right text-[10px]'>{t('channels.endpoints.readOnly')}</span>
            ) : (
              children?.(ep, index)
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export function ChannelsEndpointsDialog({ channel, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const updateChannelSettings = useUpdateChannelSettings();
  const detectEndpoints = useDetectChannelEndpoints();
  const wasOpenRef = useRef(false);

  const isCodexChannel = channel.type === 'codex';
  const defaultEndpoints = useMemo(() => {
    const defaults = channel.defaultEndpoints ?? [];
    return isCodexChannel ? defaults.filter((ep) => !isResponsesFamilyFormat(ep.apiFormat)) : defaults;
  }, [channel.defaultEndpoints, isCodexChannel]);
  const initialCodexResponses = useMemo(() => getCodexResponsesWebSocketState(channel), [channel]);
  const [codexResponsesWebSocket, setCodexResponsesWebSocket] = useState(initialCodexResponses.enabled);
  const modelPolicies = useMemo(
    () =>
      (isCodexChannel && translateLegacyCodexModelPolicies(channel, channel.settings?.modelApiFormatPolicies)) ||
      channel.settings?.modelApiFormatPolicies ||
      [],
    [channel, isCodexChannel]
  );

  const [endpoints, setEndpoints] = useState<ChannelEndpoint[]>(() => channel.endpoints ?? []);
  const [newApiFormat, setNewApiFormat] = useState('');
  const [newPath, setNewPath] = useState('');
  const [newBaseURL, setNewBaseURL] = useState('');
  const [blockedEndpointRemoval, setBlockedEndpointRemoval] = useState<{ apiFormat: string; models: string[] } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [detectionResults, setDetectionResults] = useState<DetectedChannelEndpoint[] | null>(null);

  useEffect(() => {
    const justOpened = open && !wasOpenRef.current;
    wasOpenRef.current = open;
    if (!justOpened) {
      return;
    }

    setEndpoints(channel.endpoints ?? []);
    setCodexResponsesWebSocket(initialCodexResponses.enabled);
    setNewApiFormat('');
    setNewPath('');
    setNewBaseURL('');
    setBlockedEndpointRemoval(null);
    setError(null);
    setDetectionResults(null);
  }, [open, channel.endpoints, channel.settings, initialCodexResponses.enabled]);

  const usedApiFormats = useMemo(() => new Set(endpoints.map((ep) => ep.apiFormat)), [endpoints]);

  const availableApiFormats = useMemo(
    () =>
      getConfigurableApiFormatsForChannelType(channel.type, configurableChannelEndpointApiFormats).filter((f) => !usedApiFormats.has(f)),
    [usedApiFormats, channel.type]
  );

  const savedEndpoints = useMemo(
    () => (isCodexChannel ? buildCodexResponsesEndpoints(endpoints, codexResponsesWebSocket) : endpoints),
    [endpoints, codexResponsesWebSocket, isCodexChannel]
  );
  const codexWebSocketURL =
    isCodexChannel && codexResponsesWebSocket ? getCodexResponsesWebSocketURL(toHttpBaseURL(channel.baseURL), endpoints) : undefined;

  const handleAddEndpoint = useCallback(() => {
    setError(null);
    if (!newApiFormat) return;

    if (usedApiFormats.has(newApiFormat)) {
      setError(t('channels.endpoints.duplicateError'));
      return;
    }

    if (!ALLOWED_API_FORMATS.includes(newApiFormat as (typeof ALLOWED_API_FORMATS)[number])) {
      setError(t('channels.endpoints.invalidApiFormat', 'Unsupported API format'));
      return;
    }

    const parsed = channelEndpointSchema.safeParse({
      apiFormat: newApiFormat,
      path: newPath || undefined,
      baseURL: newBaseURL || undefined,
    });
    if (!parsed.success) {
      const firstIssue = parsed.error.issues[0];
      if (firstIssue?.path[0] === 'baseURL') {
        setError(t('channels.endpoints.invalidBaseURL'));
      } else {
        setError(firstIssue?.message || 'Invalid endpoint');
      }
      return;
    }

    setEndpoints((prev) => [...prev, parsed.data]);
    setNewApiFormat('');
    setNewPath('');
    setNewBaseURL('');
  }, [newApiFormat, newPath, newBaseURL, usedApiFormats, t]);

  const handleRemoveEndpoint = useCallback((apiFormat: string) => {
    setEndpoints((prev) => prev.filter((ep) => ep.apiFormat !== apiFormat));
    setBlockedEndpointRemoval(null);
    setError(null);
  }, []);

  // Per-model API format policies reference api_formats, so removing an endpoint
  // can leave a model with no usable endpoint on this channel. Report those
  // models instead of silently starving them.
  const modelsStarvedByRemoving = useCallback(
    (apiFormat: string): string[] => {
      const policies = modelPolicies;
      const remaining = new Set(
        [...defaultEndpoints, ...savedEndpoints.filter((ep) => ep.apiFormat !== apiFormat)].map((ep) => ep.apiFormat)
      );

      const starved: string[] = [];
      for (const policy of policies) {
        const allows = (format: string): boolean => {
          if (policy.allow && policy.allow.length > 0) {
            return policy.allow.includes(format);
          }
          return !(policy.exclude ?? []).includes(format);
        };
        if (!Array.from(remaining).some(allows)) {
          starved.push(policy.model);
        }
      }

      return starved;
    },
    [modelPolicies, defaultEndpoints, savedEndpoints]
  );

  const handleRequestRemoveEndpoint = useCallback(
    (apiFormat: string) => {
      const starved = modelsStarvedByRemoving(apiFormat);
      if (starved.length > 0) {
        setBlockedEndpointRemoval({ apiFormat, models: starved });
        return;
      }
      handleRemoveEndpoint(apiFormat);
    },
    [handleRemoveEndpoint, modelsStarvedByRemoving]
  );

  const handleCodexWebSocketChange = useCallback(
    (checked: boolean) => {
      const models = checked ? [] : modelsStarvedByRemoving(OPENAI_RESPONSES_WEBSOCKET);
      if (models.length > 0) {
        setBlockedEndpointRemoval({ apiFormat: OPENAI_RESPONSES_WEBSOCKET, models });
        return;
      }
      setCodexResponsesWebSocket(checked);
    },
    [modelsStarvedByRemoving]
  );

  const handleDetectEndpoints = useCallback(async () => {
    setError(null);

    try {
      const results = await detectEndpoints.mutateAsync({ channelID: channel.id });
      setDetectionResults(results);

      // Add the supported relay protocols that the channel does not expose yet.
      // Default endpoints are built-in and read-only, so only the override list
      // is updated. The user still has to save to persist the change.
      const available = getChannelRelayProtocols(defaultEndpoints, endpoints);
      const additions = results
        .filter((result) => result.supported && isRelayProtocol(result.apiFormat) && !available.has(result.apiFormat))
        .map((result) => channelEndpointSchema.parse({ apiFormat: result.apiFormat }));

      if (additions.length > 0) {
        setEndpoints((prev) => {
          const existing = new Set(prev.map((ep) => ep.apiFormat));
          return [...prev, ...additions.filter((ep) => !existing.has(ep.apiFormat))];
        });
      }
    } catch {
      // error handled by the hook
    }
  }, [channel.id, defaultEndpoints, endpoints, detectEndpoints]);

  const handleSave = useCallback(async () => {
    setError(null);

    const apiFormats = savedEndpoints.map((ep) => ep.apiFormat);
    const duplicates = apiFormats.filter((f, i) => apiFormats.indexOf(f) !== i);
    if (duplicates.length > 0) {
      setError(t('channels.endpoints.duplicateError'));
      return;
    }

    const invalidApiFormat = apiFormats.find((format) => !RECOGNIZED_API_FORMATS.includes(format));
    if (invalidApiFormat) {
      setError(t('channels.endpoints.invalidApiFormat', 'Unsupported API format'));
      return;
    }

    // Reject only starvation. A denylist may name a removed endpoint without
    // invalidating the policy; unrelated allowed endpoints must remain usable.
    const savedFormats = new Set([...defaultEndpoints.map((ep) => ep.apiFormat), ...apiFormats]);
    const invalidPolicy = modelPolicies.find(
      (policy) =>
        !Array.from(savedFormats).some((format) =>
          policy.allow?.length ? policy.allow.includes(format) : !(policy.exclude ?? []).includes(format)
        )
    );
    if (invalidPolicy) {
      setError(t('channels.endpoints.policyStarvationWarning', { models: invalidPolicy.model }));
      return;
    }
    const baseURL = isCodexChannel ? toHttpBaseURL(channel.baseURL) : channel.baseURL;

    try {
      await updateChannelSettings.mutateAsync({
        id: channel.id,
        input: {
          ...(baseURL !== channel.baseURL ? { baseURL } : {}),
          endpoints: savedEndpoints.map((ep) => ({
            apiFormat: ep.apiFormat,
            path: ep.path || undefined,
            baseURL: ep.baseURL || undefined,
            transport: ep.transport || undefined,
          })),
        },
        patch: isCodexChannel ? { modelApiFormatPolicies: modelPolicies } : {},
      });

      toast.success(t('channels.messages.updateSuccess'));
      onOpenChange(false);
    } catch {
      // error handled by hook
    }
  }, [channel, defaultEndpoints, savedEndpoints, modelPolicies, isCodexChannel, onOpenChange, updateChannelSettings, t]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && newApiFormat) {
        e.preventDefault();
        handleAddEndpoint();
      }
    },
    [newApiFormat, handleAddEndpoint]
  );

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='flex h-[90vh] max-h-[700px] w-full max-w-full flex-col overflow-hidden sm:max-w-4xl'>
          <DialogHeader className='shrink-0'>
            <DialogTitle>{t('channels.endpoints.title')}</DialogTitle>
            <DialogDescription>{channel.name}</DialogDescription>
          </DialogHeader>

          <div className='min-h-0 flex-1 space-y-6 overflow-y-auto py-4'>
            {isCodexChannel && (
              <div className='space-y-3'>
                <label className='text-sm font-medium'>{t('channels.dialogs.fields.apiFormat.label')}</label>
                <div className='space-y-1 rounded-lg border px-3 py-2.5'>
                  <div className='flex min-h-8 items-center justify-between gap-4'>
                    <span className='text-sm'>{t('channels.dialogs.fields.apiFormat.formats.openai/responses')}</span>
                    <label className='flex cursor-pointer items-center gap-2 text-sm'>
                      <Checkbox
                        checked={codexResponsesWebSocket}
                        onCheckedChange={(checked) => handleCodexWebSocketChange(checked === true)}
                        data-testid='codex-endpoints-websocket-checkbox'
                      />
                      <span>{t('channels.dialogs.fields.apiFormat.codexWebSocket.label')}</span>
                    </label>
                  </div>
                  {codexWebSocketURL && (
                    <p className='text-muted-foreground text-xs break-all' data-testid='codex-endpoints-websocket-url'>
                      {t('channels.dialogs.fields.apiFormat.codexWebSocket.derivedURL', { url: codexWebSocketURL })}
                    </p>
                  )}
                  {codexResponsesWebSocket &&
                    (initialCodexResponses.webSocketOnly || hasCodexResponsesWebSocketOverrideReplacement(channel)) && (
                      <p className='text-xs text-amber-600 dark:text-amber-400' data-testid='codex-endpoints-websocket-upgrade-note'>
                        {t(
                          initialCodexResponses.webSocketOnly
                            ? 'channels.dialogs.fields.apiFormat.codexWebSocket.legacyUpgrade'
                            : 'channels.dialogs.fields.apiFormat.codexWebSocket.overrideReplacement'
                        )}
                      </p>
                    )}
                </div>
              </div>
            )}

            {/* Default endpoints */}
            <div className='space-y-3'>
              <div className='flex items-center justify-between'>
                <label className='text-sm font-medium'>{t('channels.endpoints.defaultEndpoints', 'Default endpoints')}</label>
                {defaultEndpoints.length > 0 && (
                  <span className='text-muted-foreground text-xs'>
                    {t('channels.endpoints.resolvedCount', { count: defaultEndpoints.length })}
                  </span>
                )}
              </div>
              {defaultEndpoints.length === 0 ? (
                <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm'>
                  {t('channels.endpoints.noDefaultEndpoints', 'No default endpoints resolved for this channel type.')}
                </div>
              ) : (
                <EndpointTable endpoints={defaultEndpoints} readOnly hideBaseURL />
              )}
            </div>

            {/* Current configured endpoints */}
            <div className='space-y-3'>
              <div className='flex items-center justify-between gap-2'>
                <label className='text-sm font-medium'>{t('channels.endpoints.currentEndpoints')}</label>
                <div className='flex items-center gap-2'>
                  {savedEndpoints.length > 0 && (
                    <span className='text-muted-foreground text-xs'>
                      {t('channels.endpoints.configuredCount', { count: savedEndpoints.length })}
                    </span>
                  )}
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='h-8'
                    onClick={handleDetectEndpoints}
                    disabled={detectEndpoints.isPending}
                  >
                    {detectEndpoints.isPending ? (
                      <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                    ) : (
                      <Radar className='mr-1.5 h-3.5 w-3.5' />
                    )}
                    {t(detectEndpoints.isPending ? 'channels.endpoints.detect.detecting' : 'channels.endpoints.detect.button')}
                  </Button>
                </div>
              </div>
              <p className='text-muted-foreground text-xs'>{t('channels.endpoints.detect.description')}</p>
              {savedEndpoints.length === 0 ? (
                <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm'>
                  {t('channels.endpoints.noOverridesHint', 'No custom endpoint overrides configured.')}
                </div>
              ) : (
                <EndpointTable endpoints={savedEndpoints}>
                  {(ep) =>
                    isCodexChannel && isResponsesFamilyFormat(ep.apiFormat) ? (
                      <span className='text-muted-foreground text-right text-[10px]'>{t('channels.endpoints.readOnly')}</span>
                    ) : (
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='hover:text-destructive hover:bg-destructive/10 h-7 w-7 p-0'
                        onClick={() => handleRequestRemoveEndpoint(ep.apiFormat)}
                        aria-label={t('channels.endpoints.removeEndpoint', { apiFormat: ep.apiFormat })}
                      >
                        <X className='h-3.5 w-3.5' />
                      </Button>
                    )
                  }
                </EndpointTable>
              )}
              {detectionResults && (
                <div className='bg-muted/20 space-y-2 rounded-lg border p-3'>
                  <p className='text-muted-foreground text-xs'>{t('channels.endpoints.detect.resultTitle')}</p>
                  <div className='flex flex-wrap gap-2'>
                    {detectionResults.map((result) => (
                      <DetectionResultCard key={result.apiFormat} result={result} />
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* Add new endpoint */}
            <div className='space-y-3'>
              <label className='text-sm font-medium'>{t('channels.endpoints.addEndpoint')}</label>
              <div className='grid items-start gap-3 md:grid-cols-[minmax(10rem,1fr)_minmax(14rem,1.2fr)_minmax(14rem,1.2fr)_auto]'>
                <div className='min-w-0 space-y-1'>
                  <label htmlFor='endpoint-api-format' className='text-muted-foreground block text-xs'>
                    {t('channels.endpoints.apiFormat')}
                  </label>
                  <Select value={newApiFormat} onValueChange={setNewApiFormat}>
                    <SelectTrigger id='endpoint-api-format' className='w-full'>
                      <SelectValue placeholder={t('channels.endpoints.apiFormat')} />
                    </SelectTrigger>
                    <SelectContent>
                      {availableApiFormats.length === 0 ? (
                        <div className='text-muted-foreground px-2 py-4 text-center text-sm'>{t('channels.endpoints.allFormatsUsed')}</div>
                      ) : (
                        availableApiFormats.map((format) => (
                          <SelectItem key={format} value={format}>
                            {format}
                          </SelectItem>
                        ))
                      )}
                    </SelectContent>
                  </Select>
                </div>
                <div className='min-w-0 space-y-1'>
                  <label htmlFor='endpoint-base-url' className='text-muted-foreground block text-xs'>
                    {t('channels.endpoints.baseURL')}
                  </label>
                  <Input
                    id='endpoint-base-url'
                    placeholder={newApiFormat ? t('channels.endpoints.baseURLPlaceholder') : t('channels.endpoints.selectFormatFirst')}
                    value={newBaseURL}
                    onChange={(e) => setNewBaseURL(e.target.value)}
                    onKeyDown={handleKeyDown}
                    disabled={!newApiFormat}
                    aria-describedby='endpoint-base-url-hint'
                    className='disabled:opacity-50'
                  />
                  <p id='endpoint-base-url-hint' className='text-muted-foreground text-[11px] leading-4 break-words'>
                    {t('channels.endpoints.baseURLHint')}
                  </p>
                </div>
                <div className='min-w-0 space-y-1'>
                  <label htmlFor='endpoint-path' className='text-muted-foreground block text-xs'>
                    {t('channels.endpoints.path')}
                  </label>
                  <Input
                    id='endpoint-path'
                    placeholder={newApiFormat ? t('channels.endpoints.pathPlaceholder') : t('channels.endpoints.selectFormatFirst')}
                    value={newPath}
                    onChange={(e) => setNewPath(e.target.value)}
                    onKeyDown={handleKeyDown}
                    disabled={!newApiFormat}
                    aria-describedby='endpoint-path-hint'
                    className='disabled:opacity-50'
                  />
                  <p id='endpoint-path-hint' className='text-muted-foreground text-[11px] leading-4 break-words'>
                    {t('channels.endpoints.pathHint')}
                  </p>
                </div>
                <Button
                  type='button'
                  variant='default'
                  size='icon'
                  onClick={handleAddEndpoint}
                  disabled={!newApiFormat}
                  className='shrink-0 justify-self-end md:mt-5 md:justify-self-auto'
                  aria-label={t('channels.endpoints.addEndpoint')}
                >
                  <Plus className='h-4 w-4' />
                </Button>
              </div>
            </div>

            {error && (
              <div className='text-destructive bg-destructive/10 flex items-center gap-2 rounded-md px-3 py-2 text-sm'>
                <AlertCircle className='h-4 w-4 shrink-0' />
                <span>{error}</span>
              </div>
            )}
          </div>

          <DialogFooter className='shrink-0 border-t pt-4'>
            <Button variant='outline' onClick={() => onOpenChange(false)}>
              {t('common.buttons.cancel')}
            </Button>
            <Button onClick={handleSave} disabled={updateChannelSettings.isPending}>
              {updateChannelSettings.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <AlertDialog
        open={blockedEndpointRemoval !== null}
        onOpenChange={(isOpen) => {
          if (!isOpen) setBlockedEndpointRemoval(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader className='text-left'>
            <AlertDialogTitle>{t('channels.endpoints.removeEndpointBlockedTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('channels.endpoints.policyStarvationWarning', { models: blockedEndpointRemoval?.models.join(', ') ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {blockedEndpointRemoval && blockedEndpointRemoval.models.length > 0 && (
            <div className='flex flex-wrap gap-1 rounded-md border px-3 py-2'>
              {blockedEndpointRemoval.models.map((model) => (
                <Badge key={model} variant='secondary' className='font-mono text-xs'>
                  {model}
                </Badge>
              ))}
            </div>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.buttons.close')}</AlertDialogCancel>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
