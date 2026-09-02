import { useCallback, useEffect, useMemo, useState } from 'react';
import { IconAlertCircle, IconChevronDown, IconSearch } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { extractNumberIDAsNumber } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useModels } from '../context/models-context';
import { MAX_ASSOCIATIONS } from '../data/constants';
import { buildProviderModelIndex, compareModelIds, deriveModelMetadata, normalizeModelKey } from '../data/model-metadata';
import {
  type ImportUnassociatedModelItemInput,
  useImportUnassociatedModels,
  useQueryAllModels,
  useQueryUnassociatedChannels,
} from '../data/models';
import { useDevelopersData } from '../data/providers';
import { type Model } from '../data/schema';

/**
 * Which Model a selected upstream model is imported into: either a brand new
 * Model identified by its model ID, or an existing one identified by its numeric ID.
 */
type ImportTarget = { kind: 'new'; modelId: string } | { kind: 'existing'; id: number };

/** One entry of the built-in model library, as offered in the target dropdown. */
interface BuiltinModelOption {
  modelId: string;
  developer: string;
  name: string;
}

interface SelectedSource {
  /** `channelId::upstreamModelId` — unique per row in the left-hand list. */
  key: string;
  channelId: number;
  channelName: string;
  upstreamModelId: string;
  target: ImportTarget;
}

function sourceKey(channelId: number, upstreamModelId: string) {
  return `${channelId}::${upstreamModelId}`;
}

function targetKey(target: ImportTarget) {
  return target.kind === 'new' ? `new:${target.modelId}` : `existing:${target.id}`;
}

/** Upstream IDs carry a routing prefix the Model itself should not inherit. */
function stripVendorPrefix(upstreamModelId: string) {
  return upstreamModelId.slice(upstreamModelId.lastIndexOf('/') + 1);
}

/**
 * How many built-in library entries a search renders. The library holds several
 * hundred models; past a screenful the list stops helping and only makes the
 * popover harder to read, so narrowing the query is the way through.
 */
const BUILTIN_RESULT_LIMIT = 20;

/**
 * Whether a target option survives the popover's search box.
 *
 * cmdk's own filtering is turned off in that popover because model IDs need
 * separator-insensitive matching — `gpt4o` should find `gpt-4o` — and running two
 * filters in series would hide entries this one accepts.
 */
function matchesTargetSearch(search: string, ...fields: string[]) {
  const raw = search.trim().toLowerCase();
  if (!raw) return true;

  const key = normalizeModelKey(search);

  return fields.some((field) => field.toLowerCase().includes(raw) || (!!key && normalizeModelKey(field).includes(key)));
}

function countAssociations(model?: Model) {
  return model?.settings?.associations?.length || 0;
}

/**
 * Priority the next appended association gets, mirroring what the backend
 * computes so the preview shows the priorities the import will actually write.
 */
function nextPriorityFor(model?: Model) {
  const associations = model?.settings?.associations || [];
  if (associations.length === 0) return 0;

  return Math.max(...associations.map((association) => association.priority ?? 0)) + 1;
}

export function ModelsUnassociatedDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useModels();
  const { data, refetch, isLoading, isFetching } = useQueryUnassociatedChannels();
  const { data: modelsData } = useQueryAllModels({});
  const { data: developersData } = useDevelopersData();
  const importModels = useImportUnassociatedModels();

  const [searchQuery, setSearchQuery] = useState('');
  const debouncedSearchQuery = useDebounce(searchQuery, 300);
  // Ordered: the position of a source in this list becomes its association
  // priority, so the user's pick order is the failover order.
  const [selected, setSelected] = useState<SelectedSource[]>([]);

  const isOpen = open === 'unassociated';

  useEffect(() => {
    if (isOpen) {
      refetch();
      setSearchQuery('');
      setSelected([]);
    }
  }, [isOpen, refetch]);

  const handleClose = useCallback(() => {
    setOpen(null);
  }, [setOpen]);

  const providerIndex = useMemo(() => buildProviderModelIndex(developersData), [developersData]);

  const existingModels = useMemo(() => modelsData?.edges.map((edge) => edge.node) || [], [modelsData]);

  const existingModelsById = useMemo(() => {
    const map = new Map<number, Model>();
    existingModels.forEach((model) => map.set(extractNumberIDAsNumber(model.id), model));

    return map;
  }, [existingModels]);

  /**
   * Which Model owns each model ID. Model IDs are globally unique — archived
   * Models keep theirs — so this decides whether a source can become a new Model
   * at all, or only be appended to the Model already holding its ID.
   */
  const modelsByModelID = useMemo(() => {
    const map = new Map<string, Model>();
    existingModels.forEach((model) => map.set(model.modelID, model));

    return map;
  }, [existingModels]);

  /**
   * The built-in model library flattened for the target dropdown.
   *
   * Picking an entry here is what earns a new Model real metadata: the import
   * derives metadata from the *target* model ID against this same index, so a
   * library ID matches by construction, while a raw upstream ID often does not
   * and falls back to placeholders.
   */
  const builtinOptions = useMemo(() => {
    const options: BuiltinModelOption[] = [];
    providerIndex.forEach(({ developer, model }) => {
      options.push({
        modelId: stripVendorPrefix(model.id),
        developer,
        name: model.display_name || model.name || model.id,
      });
    });

    return options;
  }, [providerIndex]);

  const filteredData = useMemo(() => {
    if (!data || !debouncedSearchQuery.trim()) return data;

    const query = debouncedSearchQuery.toLowerCase();

    return data
      .map((info) => ({
        ...info,
        models: info.models.filter((model) => model.toLowerCase().includes(query)),
      }))
      .filter((info) => info.models.length > 0);
  }, [data, debouncedSearchQuery]);

  const selectedByKey = useMemo(() => {
    const map = new Map<string, SelectedSource>();
    selected.forEach((source) => map.set(source.key, source));

    return map;
  }, [selected]);

  /**
   * How many associations each existing Model would hold after the import. The
   * dropdown greys out targets that are already at the cap, so the user cannot
   * build an item the backend would reject.
   */
  const pendingAppendCounts = useMemo(() => {
    const counts = new Map<number, number>();
    selected.forEach((source) => {
      if (source.target.kind === 'existing') {
        counts.set(source.target.id, (counts.get(source.target.id) || 0) + 1);
      }
    });

    return counts;
  }, [selected]);

  /** Model IDs of the new Models this import would create, in pick order. */
  const pendingNewModelIds = useMemo(() => {
    const ids: string[] = [];
    selected.forEach((source) => {
      if (source.target.kind === 'new' && !ids.includes(source.target.modelId)) {
        ids.push(source.target.modelId);
      }
    });

    return ids;
  }, [selected]);

  const toggleSource = useCallback(
    (channelId: number, channelName: string, upstreamModelId: string) => {
      const key = sourceKey(channelId, upstreamModelId);
      setSelected((prev) => {
        const existing = prev.find((source) => source.key === key);
        if (existing) {
          return prev.filter((source) => source.key !== key);
        }

        // A source whose model ID is already taken can only be appended to the
        // Model holding it — creating a second Model with that ID is impossible.
        // Otherwise default to a new Model rather than preselecting a merge: a
        // name that merely looks similar (glm-5.2 vs glm-5.2-think) is a
        // different model, so merging is the user's call.
        const newModelId = stripVendorPrefix(upstreamModelId);
        const owner = modelsByModelID.get(newModelId);

        return [
          ...prev,
          {
            key,
            channelId,
            channelName,
            upstreamModelId,
            target: owner
              ? { kind: 'existing', id: extractNumberIDAsNumber(owner.id) }
              : { kind: 'new', modelId: newModelId },
          },
        ];
      });
    },
    [modelsByModelID]
  );

  const setTarget = useCallback((key: string, target: ImportTarget) => {
    setSelected((prev) => prev.map((source) => (source.key === key ? { ...source, target } : source)));
  }, []);

  /** Groups the selection by target Model — one import item per group. */
  const groups = useMemo(() => {
    const byTarget = new Map<string, { target: ImportTarget; sources: SelectedSource[] }>();
    selected.forEach((source) => {
      const key = targetKey(source.target);
      const group = byTarget.get(key);
      if (group) {
        group.sources.push(source);
      } else {
        byTarget.set(key, { target: source.target, sources: [source] });
      }
    });

    return Array.from(byTarget.values());
  }, [selected]);

  const createCount = groups.filter((group) => group.target.kind === 'new').length;
  const appendCount = groups.filter((group) => group.target.kind === 'existing').length;

  /**
   * Targets the import would push past the association cap. The dropdown greys
   * out Models that are already full, but a default target is assigned without
   * going through the dropdown, so the overflow is caught here as well.
   */
  const overCapacityTargets = useMemo(() => {
    const keys = new Set<string>();
    groups.forEach((group) => {
      const held = group.target.kind === 'existing' ? countAssociations(existingModelsById.get(group.target.id)) : 0;
      if (held + group.sources.length > MAX_ASSOCIATIONS) {
        keys.add(targetKey(group.target));
      }
    });

    return keys;
  }, [groups, existingModelsById]);

  const handleImport = useCallback(async () => {
    const items: ImportUnassociatedModelItemInput[] = groups.map((group) => {
      const sources = group.sources.map((source) => ({
        channelId: source.channelId,
        upstreamModelId: source.upstreamModelId,
      }));

      if (group.target.kind === 'existing') {
        return { targetModelId: group.target.id, sources };
      }

      // Metadata comes from the target model ID rather than from any single
      // source: several upstream names can feed one new Model, and the target ID
      // is what the Model is actually called.
      const metadata = deriveModelMetadata(group.target.modelId, providerIndex);

      return {
        metadata: {
          modelId: group.target.modelId,
          developer: metadata.developer,
          type: metadata.type,
          name: metadata.name,
          icon: metadata.icon,
          group: metadata.group,
          modelCard: metadata.modelCard,
        },
        sources,
      };
    });

    try {
      await importModels.mutateAsync(items);
      handleClose();
    } catch {
      // Surfaced by the mutation's error handler; the dialog stays open so the
      // user can adjust the selection.
    }
  }, [groups, providerIndex, importModels, handleClose]);

  const totalUnassociated = data?.reduce((sum, info) => sum + info.models.length, 0) || 0;

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='flex max-h-[90vh] flex-col overflow-hidden sm:max-w-6xl'>
        <DialogHeader className='flex-shrink-0'>
          <DialogTitle>{t('models.unassociated.title')}</DialogTitle>
          <DialogDescription>{t('models.unassociated.description')}</DialogDescription>
        </DialogHeader>

        {isLoading || isFetching ? (
          <div className='flex items-center justify-center py-8'>
            <div className='text-muted-foreground text-sm'>{t('common.loading')}</div>
          </div>
        ) : data && data.length > 0 ? (
          <>
            <div className='flex flex-shrink-0 items-center gap-2 rounded-lg border border-warning/40 bg-warning/10 p-3 dark:border-warning/40'>
              <IconAlertCircle className='h-5 w-5 shrink-0 text-(--warning-soft-fg) dark:text-(--warning-soft-fg)' />
              <div className='text-sm text-(--warning-soft-fg) dark:text-(--warning-soft-fg)'>
                {t('models.unassociated.summary', {
                  channelCount: data.length,
                  modelCount: totalUnassociated,
                })}
              </div>
            </div>

            <div className='flex min-h-0 flex-1 flex-col gap-6 sm:flex-row'>
              {/* Left Side - Selection */}
              <div className='flex min-h-0 flex-1 flex-col sm:flex-[2]'>
                <div className='relative flex-shrink-0 pb-3'>
                  <IconSearch className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
                  <Input
                    placeholder={t('models.unassociated.searchPlaceholder')}
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className='pl-9'
                  />
                </div>

                <ScrollArea className='min-h-0 flex-1 rounded-md border p-4'>
                  {!filteredData || filteredData.length === 0 ? (
                    <p className='text-muted-foreground py-8 text-center text-sm'>
                      {debouncedSearchQuery.trim()
                        ? t('models.unassociated.noSearchResults')
                        : t('models.unassociated.noUnassociated')}
                    </p>
                  ) : (
                    <div className='space-y-4'>
                      {filteredData.map((info) => {
                        const channelId = extractNumberIDAsNumber(info.channel.id);

                        return (
                          <div key={info.channel.id} className='space-y-2'>
                            <div className='flex items-center gap-2'>
                              <span className='text-sm font-medium'>{info.channel.name}</span>
                              <Badge variant='outline' className='text-xs'>
                                {info.channel.type}
                              </Badge>
                            </div>

                            <div className='space-y-1.5'>
                              {info.models.map((upstreamModelId) => {
                                const key = sourceKey(channelId, upstreamModelId);
                                const selection = selectedByKey.get(key);

                                return (
                                  <div
                                    key={key}
                                    className='flex flex-wrap items-center gap-2 rounded-md border px-3 py-2'
                                  >
                                    <Checkbox
                                      id={key}
                                      checked={!!selection}
                                      onCheckedChange={() => toggleSource(channelId, info.channel.name, upstreamModelId)}
                                    />
                                    <label htmlFor={key} className='min-w-0 flex-1 cursor-pointer font-mono text-xs break-all'>
                                      {upstreamModelId}
                                    </label>

                                    {selection && (
                                      <TargetSelect
                                        upstreamModelId={upstreamModelId}
                                        target={selection.target}
                                        existingModels={existingModels}
                                        existingModelsById={existingModelsById}
                                        modelsByModelID={modelsByModelID}
                                        builtinOptions={builtinOptions}
                                        pendingAppendCounts={pendingAppendCounts}
                                        pendingNewModelIds={pendingNewModelIds}
                                        onChange={(target) => setTarget(key, target)}
                                      />
                                    )}
                                  </div>
                                );
                              })}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </ScrollArea>
              </div>

              {/* Right Side - Preview */}
              <div className='flex min-h-0 flex-1 flex-col border-t pt-4 sm:border-t-0 sm:border-l sm:pt-0 sm:pl-6'>
                <div className='shrink-0 space-y-1 pb-3'>
                  <h3 className='text-sm font-semibold'>{t('models.unassociated.preview')}</h3>
                  <p className='text-muted-foreground text-xs'>{t('models.unassociated.previewDescription')}</p>
                </div>

                <div className='min-h-0 flex-1 overflow-y-auto'>
                  {groups.length === 0 ? (
                    <p className='text-muted-foreground py-8 text-center text-sm'>{t('models.unassociated.previewEmpty')}</p>
                  ) : (
                    <div className='space-y-3'>
                      {groups.map((group) => {
                        const target =
                          group.target.kind === 'existing' ? existingModelsById.get(group.target.id) : undefined;
                        const basePriority = group.target.kind === 'existing' ? nextPriorityFor(target) : 0;
                        const metadata =
                          group.target.kind === 'new' ? deriveModelMetadata(group.target.modelId, providerIndex) : undefined;

                        return (
                          <div key={targetKey(group.target)} className='space-y-2 rounded-md border p-3'>
                            <div className='flex flex-wrap items-center gap-2'>
                              <span className='font-mono text-xs font-medium break-all'>
                                {group.target.kind === 'new' ? group.target.modelId : target?.modelID}
                              </span>
                              {group.target.kind === 'new' ? (
                                <Badge className='text-xs'>{t('models.unassociated.newBadge')}</Badge>
                              ) : (
                                <Badge variant='secondary' className='text-xs'>
                                  {t('models.unassociated.appendBadge')}
                                </Badge>
                              )}
                              {metadata && !metadata.matched && (
                                <Badge variant='outline' className='text-xs text-(--warning-soft-fg) dark:text-(--warning-soft-fg)'>
                                  {t('models.unassociated.needsReview')}
                                </Badge>
                              )}
                              {target?.status === 'archived' && (
                                <Badge variant='outline' className='text-xs text-(--warning-soft-fg) dark:text-(--warning-soft-fg)'>
                                  {t('models.unassociated.archived')}
                                </Badge>
                              )}
                              {overCapacityTargets.has(targetKey(group.target)) && (
                                <Badge variant='destructive' className='text-xs'>
                                  {t('models.unassociated.overCapacity', { max: MAX_ASSOCIATIONS })}
                                </Badge>
                              )}
                            </div>

                            <div className='space-y-1'>
                              {group.sources.map((source, index) => (
                                <div key={source.key} className='flex flex-wrap items-center gap-1.5 text-xs'>
                                  <Badge variant='outline' className='shrink-0 font-mono text-[10px]'>
                                    P{Math.min(basePriority + index, MAX_ASSOCIATIONS)}
                                  </Badge>
                                  <span className='text-muted-foreground'>{source.channelName}</span>
                                  <span className='text-muted-foreground'>→</span>
                                  <span className='font-mono break-all'>{source.upstreamModelId}</span>
                                </div>
                              ))}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </>
        ) : (
          <div className='flex flex-col items-center justify-center py-8 text-center'>
            <div className='text-muted-foreground text-sm'>{t('models.unassociated.noUnassociated')}</div>
          </div>
        )}

        <div className='flex flex-shrink-0 flex-col gap-2 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground text-sm'>
            {selected.length > 0
              ? t('models.unassociated.footerSummary', {
                  selected: selected.length,
                  created: createCount,
                  appended: appendCount,
                })
              : null}
          </div>
          <div className='flex justify-end gap-2'>
            <Button variant='outline' onClick={handleClose}>
              {t('common.buttons.cancel')}
            </Button>
            <Button
              onClick={handleImport}
              disabled={selected.length === 0 || overCapacityTargets.size > 0 || importModels.isPending}
            >
              {t('models.unassociated.importButton', { count: selected.length })}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

interface TargetSelectProps {
  upstreamModelId: string;
  target: ImportTarget;
  existingModels: Model[];
  existingModelsById: Map<number, Model>;
  modelsByModelID: Map<string, Model>;
  builtinOptions: BuiltinModelOption[];
  pendingAppendCounts: Map<number, number>;
  pendingNewModelIds: string[];
  onChange: (target: ImportTarget) => void;
}

/**
 * Picks the Model a source is imported into. Existing Models that already hold
 * the maximum number of associations are shown but disabled, and archived ones
 * are badged rather than hidden — an archived Model still owns its model ID, so
 * appending to it is often what the user wants, but only they can decide that.
 */
function TargetSelect({
  upstreamModelId,
  target,
  existingModels,
  existingModelsById,
  modelsByModelID,
  builtinOptions,
  pendingAppendCounts,
  pendingNewModelIds,
  onChange,
}: TargetSelectProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  // Held here rather than left to cmdk because the built-in library group is
  // filtered by hand: the whole library is only searched once there is a query.
  const [search, setSearch] = useState('');

  const ownNewModelId = stripVendorPrefix(upstreamModelId);

  // Model IDs already owned by a Model are dropped: the backend rejects the whole
  // import if any item tries to create a duplicate, so offering one would be a
  // guaranteed failure. Appending to the owner is offered below instead.
  const newOptions = useMemo(() => {
    const ids = [ownNewModelId, ...pendingNewModelIds.filter((id) => id !== ownNewModelId)];

    return ids.filter((id) => !modelsByModelID.has(id) && matchesTargetSearch(search, id));
  }, [modelsByModelID, ownNewModelId, pendingNewModelIds, search]);

  /**
   * Built-in library entries offered for this row.
   *
   * The library holds hundreds of models, so the full list is never rendered:
   * with no search text only the entries resembling the upstream ID are shown —
   * that is the handful the user actually wants — and typing searches the whole
   * library, capped so the popover stays scrollable. IDs already owned by a Model
   * are dropped for the same reason as in `newOptions`.
   */
  const builtinMatches = useMemo(() => {
    const available = builtinOptions.filter((option) => !modelsByModelID.has(option.modelId));

    if (!search.trim()) {
      // A short upstream ID can prefix-match a lot of the library, so the
      // suggestions are capped too, exact matches first.
      return available
        .map((option) => ({ option, similarity: compareModelIds(upstreamModelId, option.modelId) }))
        .filter(({ similarity }) => similarity !== 'none')
        .sort((a, b) => (a.similarity === b.similarity ? 0 : a.similarity === 'exact' ? -1 : 1))
        .slice(0, BUILTIN_RESULT_LIMIT)
        .map(({ option }) => option);
    }

    return available.filter((option) => matchesTargetSearch(search, option.modelId, option.name)).slice(0, BUILTIN_RESULT_LIMIT);
  }, [builtinOptions, modelsByModelID, search, upstreamModelId]);

  // Similar IDs float to the top so the likely target is the first thing read,
  // without being selected for the user.
  const sortedExisting = useMemo(() => {
    const scored = existingModels
      .filter((model) => matchesTargetSearch(search, model.modelID, model.name))
      .map((model) => ({
        model,
        similarity: compareModelIds(upstreamModelId, model.modelID),
      }));
    const rank = { exact: 0, similar: 1, none: 2 } as const;

    return scored.sort((a, b) => rank[a.similarity] - rank[b.similarity]);
  }, [existingModels, search, upstreamModelId]);

  const label =
    target.kind === 'new'
      ? t('models.unassociated.createNewWith', { modelId: target.modelId })
      : existingModelsById.get(target.id)?.modelID || t('models.unassociated.targetModel');

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        // Reopening starts from the suggestions rather than the last query, which
        // by then belongs to whatever the user was looking for previously.
        if (!next) setSearch('');
      }}
    >
      <PopoverTrigger asChild>
        <Button variant='outline' size='sm' role='combobox' aria-expanded={open} className='h-7 max-w-[240px] justify-between gap-1 text-xs'>
          <span className='truncate'>{label}</span>
          <IconChevronDown className='h-3.5 w-3.5 shrink-0 opacity-50' />
        </Button>
      </PopoverTrigger>
      <PopoverContent className='w-[320px] p-0' align='end'>
        <Command shouldFilter={false}>
          <CommandInput placeholder={t('models.unassociated.searchTargetPlaceholder')} value={search} onValueChange={setSearch} />
          <CommandList>
            <CommandEmpty>{t('models.unassociated.noTargetResults')}</CommandEmpty>

            {newOptions.length > 0 && (
              <CommandGroup heading={t('models.unassociated.createNew')}>
                {newOptions.map((modelId) => (
                  <CommandItem
                    key={`new:${modelId}`}
                    value={`new ${modelId}`}
                    onSelect={() => {
                      onChange({ kind: 'new', modelId });
                      setOpen(false);
                    }}
                  >
                    <span className='truncate font-mono text-xs'>{modelId}</span>
                  </CommandItem>
                ))}
              </CommandGroup>
            )}

            {(builtinMatches.length > 0 || !search.trim()) && (
              <CommandGroup heading={t('models.unassociated.builtinModels')}>
                {builtinMatches.length > 0 ? (
                  builtinMatches.map((option) => (
                    <CommandItem
                      key={`builtin:${option.developer}:${option.modelId}`}
                      value={`builtin ${option.modelId}`}
                      onSelect={() => {
                        onChange({ kind: 'new', modelId: option.modelId });
                        setOpen(false);
                      }}
                    >
                      <div className='flex min-w-0 flex-1 flex-col'>
                        <span className='truncate font-mono text-xs'>{option.modelId}</span>
                        <span className='text-muted-foreground truncate text-[10px]'>{option.name}</span>
                      </div>
                      <Badge variant='secondary' className='shrink-0 text-[10px]'>
                        {option.developer}
                      </Badge>
                    </CommandItem>
                  ))
                ) : (
                  // Only shown before anything is typed: the library is too large to
                  // list, so this is what tells the user it can be searched at all.
                  <div className='text-muted-foreground px-2 py-1.5 text-[11px]'>{t('models.unassociated.builtinHint')}</div>
                )}
              </CommandGroup>
            )}

            <CommandGroup heading={t('models.unassociated.existingModels')}>
              {sortedExisting.map(({ model, similarity }) => {
                const id = extractNumberIDAsNumber(model.id);
                const count = countAssociations(model);
                const pending = pendingAppendCounts.get(id) || 0;
                const isSelected = target.kind === 'existing' && target.id === id;
                // Room is judged against what this import already queued, so the
                // cap cannot be exceeded by stacking sources onto one Model.
                const isFull = count + (isSelected ? 0 : pending) >= MAX_ASSOCIATIONS;

                return (
                  <CommandItem
                    key={model.id}
                    value={`${model.modelID} ${model.name}`}
                    disabled={isFull}
                    onSelect={() => {
                      if (isFull) return;
                      onChange({ kind: 'existing', id });
                      setOpen(false);
                    }}
                  >
                    <div className='flex min-w-0 flex-1 items-center gap-1.5'>
                      <span className='truncate font-mono text-xs'>{model.modelID}</span>
                      {similarity !== 'none' && (
                        <Badge variant='secondary' className='shrink-0 text-[10px]'>
                          {t('models.unassociated.similar')}
                        </Badge>
                      )}
                      {model.status === 'archived' && (
                        <Badge variant='outline' className='shrink-0 text-[10px] text-(--warning-soft-fg) dark:text-(--warning-soft-fg)'>
                          {t('models.unassociated.archived')}
                        </Badge>
                      )}
                    </div>
                    <span className='text-muted-foreground shrink-0 text-[10px]'>
                      ({count}/{MAX_ASSOCIATIONS})
                    </span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
