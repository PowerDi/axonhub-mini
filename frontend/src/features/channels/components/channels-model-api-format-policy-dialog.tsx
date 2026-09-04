import { useEffect, useMemo, useState, useCallback } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { AlertTriangle, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { useUpdateChannel } from '../data/channels';
import { Channel, ModelAPIFormatPolicy } from '../data/schema';
import { mergeChannelSettingsForUpdate } from '../utils/merge';
interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

// api formats the policy editor can reference; the backend validates the
// full SupportedAPIFormats set, this list keeps the picker to the
// protocol-level formats policies are meant for.
const POLICY_API_FORMATS = [
  'openai/chat_completions',
  'openai/completions',
  'openai/responses',
  'openai/responses_compact',
  'anthropic/messages',
  'gemini/contents',
  'ollama/chat',
] as const;

const createPolicyFormSchema = () =>
  z.object({
    modelApiFormatPolicies: z
      .array(
        z
          .object({
            model: z.string().min(1, 'Model is required'),
            exclude: z.array(z.string()).optional().nullable(),
            allow: z.array(z.string()).optional().nullable(),
          })
          .refine(
            (policy) => !policy.allow || policy.allow.length === 0 || !policy.exclude || policy.exclude.length === 0,
            {
              message: 'A policy can use either Allow or Exclude, not both',
            }
          )
      )
      .refine(
        (policies) => {
          const models = policies.map((p) => p.model);
          return new Set(models).size === models.length;
        },
        {
          message: 'Each model can only have one policy',
        }
      ),
  });

export function ChannelsModelApiFormatPolicyDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannel = useUpdateChannel();

  const policyFormSchema = useMemo(() => createPolicyFormSchema(), []);

  const form = useForm<z.infer<typeof policyFormSchema>>({
    resolver: zodResolver(policyFormSchema),
    defaultValues: {
      modelApiFormatPolicies: currentRow.settings?.modelApiFormatPolicies || [],
    },
  });

  const policies = form.watch('modelApiFormatPolicies') || [];

  const [modelSearch, setModelSearch] = useState('');
  const [error, setError] = useState<string | null>(null);

  // Models the user can pick from: the channel's supported model list.
  const channelModels = useMemo(() => currentRow.supportedModels || [], [currentRow.supportedModels]);

  // Models already carrying a policy are excluded from the picker —
  // the schema enforces one policy per model.
  const selectableModels = useMemo(() => {
    const policyModels = new Set(policies.map((p) => p.model));
    return channelModels.filter((model) => !policyModels.has(model));
  }, [channelModels, policies]);

  const filteredSelectableModels = useMemo(() => {
    const query = modelSearch.trim().toLowerCase();
    if (!query) return selectableModels;
    return selectableModels.filter((model) => model.toLowerCase().includes(query));
  }, [modelSearch, selectableModels]);

  useEffect(() => {
    if (open) {
      form.reset({
        modelApiFormatPolicies: currentRow.settings?.modelApiFormatPolicies || [],
      });
      setModelSearch('');
      setError(null);
    }
  }, [open, currentRow, form]);

  // Models affected by each policy are highlighted when their filtered
  // endpoint set becomes empty on this channel.
  const resolvedApiFormats = useMemo(() => {
    const formats = new Set<string>();
    (currentRow.defaultEndpoints || []).forEach((ep) => formats.add(ep.apiFormat));
    (currentRow.endpoints || []).forEach((ep) => formats.add(ep.apiFormat));
    return formats;
  }, [currentRow.defaultEndpoints, currentRow.endpoints]);

  // Per-model policies that leave no usable endpoint on this channel.
  const policyFiltersAllEndpoints = useCallback(
    (policy: ModelAPIFormatPolicy): boolean => {
      const allowed = (format: string): boolean => {
        if (policy.allow && policy.allow.length > 0) {
          return policy.allow.includes(format);
        }
        return !(policy.exclude || []).includes(format);
      };
      if (resolvedApiFormats.size === 0) {
        return false;
      }
      return Array.from(resolvedApiFormats).every((format) => !allowed(format));
    },
    [resolvedApiFormats]
  );

  const addPolicy = useCallback(
    (model: string) => {
      setError(null);
      if (!model) {
        return;
      }

      if (policies.some((p) => p.model === model)) {
        setError(t('channels.dialogs.modelApiFormatPolicy.duplicateError'));
        return;
      }

      const next = [...policies, { model, exclude: [], allow: [] }];
      form.setValue('modelApiFormatPolicies', next, {
        shouldValidate: true,
        shouldDirty: true,
      });
      setModelSearch('');
    },
    [policies, form, t]
  );

  const removePolicy = useCallback(
    (model: string) => {
      form.setValue(
        'modelApiFormatPolicies',
        policies.filter((p) => p.model !== model),
        { shouldValidate: true, shouldDirty: true }
      );
    },
    [policies, form]
  );

  const updatePolicy = useCallback(
    (model: string, patch: Partial<ModelAPIFormatPolicy>) => {
      form.setValue(
        'modelApiFormatPolicies',
        policies.map((p) => (p.model === model ? { ...p, ...patch } : p)),
        { shouldValidate: true, shouldDirty: true }
      );
    },
    [policies, form]
  );

  const onSubmit = async (values: z.infer<typeof policyFormSchema>) => {
    // An allow list plus an exclude list on the same policy is contradictory:
    // allow wins on the backend, so drop the exclude entries before saving.
    const cleaned = (values.modelApiFormatPolicies || []).map((policy) => {
      if (policy.allow && policy.allow.length > 0) {
        return { ...policy, exclude: [] };
      }
      return policy;
    });

    // Zero-path warning: policies that filter every endpoint on this channel
    // are the dangerous case the backend rejects when no other channel serves
    // the model — surface it before the round-trip.
    const starved = cleaned.filter(policyFiltersAllEndpoints);
    if (starved.length > 0) {
      toast.warning(
        t('channels.dialogs.modelApiFormatPolicy.zeroEndpointWarning', {
          models: starved.map((p) => p.model).join(', '),
        })
      );
    }

    try {
      const nextSettings = mergeChannelSettingsForUpdate(currentRow.settings, {
        modelApiFormatPolicies: cleaned,
      });

      await updateChannel.mutateAsync({
        id: currentRow.id,
        input: {
          settings: nextSettings,
        },
      });
      onOpenChange(false);
    } catch (_error) {
      // error handled by hook
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-[800px]'>
        <DialogHeader>
          <DialogTitle>{t('channels.dialogs.modelApiFormatPolicy.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.modelApiFormatPolicy.description', { name: currentRow.name })}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)}>
            <Card>
              <CardHeader>
                <CardTitle className='text-lg'>{t('channels.dialogs.modelApiFormatPolicy.title')}</CardTitle>
                <CardDescription>{t('channels.dialogs.modelApiFormatPolicy.hint')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                {/* Add new policy: pick models from the channel's model list */}
                <div className='space-y-2'>
                  <label className='text-muted-foreground text-xs font-medium'>
                    {t('channels.dialogs.modelApiFormatPolicy.modelSelection.title')}
                  </label>
                  <div className='relative'>
                    <Input
                      placeholder={t('channels.dialogs.modelApiFormatPolicy.modelSelection.searchPlaceholder')}
                      value={modelSearch}
                      onChange={(e) => setModelSearch(e.target.value)}
                      className='h-9'
                    />
                  </div>
                  {selectableModels.length === 0 ? (
                    <p className='text-muted-foreground py-2 text-center text-xs'>
                      {channelModels.length === 0
                        ? t('channels.dialogs.modelApiFormatPolicy.modelSelection.noModels')
                        : t('channels.dialogs.modelApiFormatPolicy.modelSelection.empty', { query: modelSearch })}
                    </p>
                  ) : filteredSelectableModels.length === 0 ? (
                    <p className='text-muted-foreground py-2 text-center text-xs'>
                      {t('channels.dialogs.modelApiFormatPolicy.modelSelection.empty', { query: modelSearch })}
                    </p>
                  ) : (
                    <div className='max-h-44 overflow-y-auto rounded-md border p-1'>
                      {filteredSelectableModels.map((model) => (
                        <label
                          key={model}
                          className='hover:bg-accent flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5'
                        >
                          <Checkbox
                            checked={false}
                            onCheckedChange={() => addPolicy(model)}
                            aria-label={t('channels.dialogs.modelApiFormatPolicy.addButton', { model })}
                          />
                          <span className='font-mono text-xs break-all'>{model}</span>
                        </label>
                      ))}
                    </div>
                  )}
                </div>

                {error && <p className='text-destructive text-sm'>{error}</p>}
                {form.formState.errors.modelApiFormatPolicies?.message && (
                  <p className='text-destructive text-sm'>{form.formState.errors.modelApiFormatPolicies.message.toString()}</p>
                )}

                {policies.length === 0 ? (
                  <p className='text-muted-foreground py-4 text-center text-sm'>
                    {t('channels.dialogs.modelApiFormatPolicy.noPolicies')}
                  </p>
                ) : (
                  <div className='space-y-2'>
                    {policies.map((policy) => {
                      const starved = policyFiltersAllEndpoints(policy);
                      const inChannelModels = channelModels.includes(policy.model);
                      return (
                        <div key={policy.model} className='rounded-lg border p-3'>
                          <div className='flex items-center justify-between pb-2'>
                            <div className='flex min-w-0 flex-wrap items-center gap-2'>
                              <Badge variant='outline' className='font-mono text-xs'>
                                {policy.model}
                              </Badge>
                              {!inChannelModels && (
                                <Badge variant='outline' className='text-muted-foreground shrink-0 text-xs'>
                                  {t('channels.dialogs.modelApiFormatPolicy.notInList')}
                                </Badge>
                              )}
                            </div>
                            <Button
                              type='button'
                              variant='ghost'
                              size='sm'
                              onClick={() => removePolicy(policy.model)}
                              className='text-destructive hover:text-destructive'
                            >
                              <X size={16} />
                            </Button>
                          </div>

                          <div className='space-y-2'>
                            <FormField
                              control={form.control}
                              name='modelApiFormatPolicies'
                              render={() => (
                                <FormItem>
                                  <FormControl>
                                    <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
                                      <div className='space-y-1'>
                                        <label className='text-muted-foreground text-xs font-medium'>
                                          {t('channels.dialogs.modelApiFormatPolicy.excludeLabel')}
                                        </label>
                                        <TagsAutocompleteInput
                                          value={policy.exclude || []}
                                          onChange={(values: string[]) => updatePolicy(policy.model, { exclude: values })}
                                          placeholder={t('channels.dialogs.modelApiFormatPolicy.excludePlaceholder')}
                                          suggestions={[...POLICY_API_FORMATS]}
                                          className='h-auto min-h-9 py-1'
                                        />
                                      </div>
                                      <div className='space-y-1'>
                                        <label className='text-muted-foreground text-xs font-medium'>
                                          {t('channels.dialogs.modelApiFormatPolicy.allowLabel')}
                                        </label>
                                        <TagsAutocompleteInput
                                          value={policy.allow || []}
                                          onChange={(values: string[]) => updatePolicy(policy.model, { allow: values })}
                                          placeholder={t('channels.dialogs.modelApiFormatPolicy.allowPlaceholder')}
                                          suggestions={[...POLICY_API_FORMATS]}
                                          className='h-auto min-h-9 py-1'
                                        />
                                      </div>
                                    </div>
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                            <p className='text-muted-foreground text-xs'>
                              {t('channels.dialogs.modelApiFormatPolicy.allowOverridesHint')}
                            </p>
                            {starved && (
                              <div className='text-(--warning-soft-fg) bg-warning/10 flex items-center gap-2 rounded-md px-3 py-2 text-sm'>
                                <AlertTriangle className='h-4 w-4 shrink-0' />
                                <span>
                                  {t('channels.dialogs.modelApiFormatPolicy.zeroEndpointWarning', {
                                    models: policy.model,
                                  })}
                                </span>
                              </div>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>

            <DialogFooter className='mt-6'>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={updateChannel.isPending}>
                {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
