'use client';

import { useState, useEffect, useMemo } from 'react';
import { IconSearch, IconPlayerPlay } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useTestChannel, useUpdateChannel } from '../data/channels';
import { Channel } from '../data/schema';
import { TestResultInline } from './test-response-summary';
import { resolveChannelEndpoints } from '../utils/merge';

type TestStatus = 'not_started' | 'testing' | 'success' | 'failed';

interface ModelTestResult {
  modelName: string;
  status: TestStatus;
  latency?: number;
  error?: string;
  message?: string;
}

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  channel: Channel;
}

export function ChannelsTestDialog({ open, onOpenChange, channel }: Props) {
  const { t } = useTranslation();
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedModels, setSelectedModels] = useState<string[]>([]);
  const [testResults, setTestResults] = useState<Record<string, ModelTestResult>>({});
  const [localSupportedModels, setLocalSupportedModels] = useState<string[]>(channel.supportedModels);
  const [isTesting, setIsTesting] = useState(false);
  const [isRemovePopoverOpen, setIsRemovePopoverOpen] = useState(false);
  const [apiFormat, setApiFormat] = useState('');
  const testChannel = useTestChannel({ silent: true });
  const updateChannel = useUpdateChannel();

  // Runtime-effective endpoints: user overrides merged over the channel
  // type's defaults — the same set the backend selects test endpoints from.
  const resolvedEndpoints = useMemo(
    () => resolveChannelEndpoints(channel.defaultEndpoints, channel.endpoints),
    [channel.defaultEndpoints, channel.endpoints]
  );
  const defaultApiFormat =
    resolvedEndpoints.find((ep) => ep.apiFormat === 'openai/chat_completions')?.apiFormat ?? resolvedEndpoints[0]?.apiFormat ?? '';

  // Filter models based on search query
  const filteredModels = localSupportedModels.filter((model) => model.toLowerCase().includes(searchQuery.toLowerCase()));

  // Initialize test results when dialog opens
  useEffect(() => {
    if (open) {
      const initialResults: Record<string, ModelTestResult> = {};
      channel.supportedModels.forEach((model) => {
        initialResults[model] = {
          modelName: model,
          status: 'not_started',
        };
      });
      setTestResults(initialResults);
      setLocalSupportedModels(channel.supportedModels);
      setSelectedModels([]);
      setSearchQuery('');
      setApiFormat(defaultApiFormat);
    }
  }, [open, channel.supportedModels, defaultApiFormat]);

  // Handle model selection
  const handleModelSelect = (modelName: string, checked: boolean) => {
    if (checked) {
      setSelectedModels((prev) => [...prev, modelName]);
    } else {
      setSelectedModels((prev) => prev.filter((m) => m !== modelName));
    }
  };

  // Handle select all
  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedModels(filteredModels);
    } else {
      setSelectedModels([]);
    }
  };

  // Test a single model
  const testModel = async (modelName: string) => {
    setTestResults((prev) => ({
      ...prev,
      [modelName]: { ...prev[modelName], status: 'testing' },
    }));

    try {
      const startTime = Date.now();
      const result = await testChannel.mutateAsync({
        channelID: channel.id,
        modelID: modelName,
        apiFormat: apiFormat || undefined,
      });
      const latency = (Date.now() - startTime) / 1000;

      setTestResults((prev) => ({
        ...prev,
        [modelName]: {
          ...prev[modelName],
          status: result.success ? 'success' : 'failed',
          latency: result.success ? result.latency || latency : undefined,
          error: result.success ? undefined : result.error || 'Test failed',
          message: result.message || undefined,
        },
      }));
    } catch (error) {
      setTestResults((prev) => ({
        ...prev,
        [modelName]: {
          ...prev[modelName],
          status: 'failed',
          error: error instanceof Error ? error.message : 'Unknown error',
        },
      }));
    }
  };

  // Test selected models
  const handleTestSelected = async () => {
    if (selectedModels.length === 0) return;

    setIsTesting(true);

    // Test models in parallel
    await Promise.all(selectedModels.map((model) => testModel(model)));

    setIsTesting(false);
  };

  // Get status badge
  const getStatusBadge = (status: TestStatus) => {
    switch (status) {
      case 'testing':
        return <Badge variant='secondary'>{t('channels.dialogs.test.testingModel')}</Badge>;
      case 'success':
        return (
          <Badge variant='default' className='border-success/40 bg-success/10 text-(--success-soft-fg)'>
            {t('channels.dialogs.test.testSuccess')}
          </Badge>
        );
      case 'failed':
        return <Badge variant='destructive'>{t('channels.dialogs.test.testFailed')}</Badge>;
      default:
        return <Badge variant='outline'>{t('channels.dialogs.test.notStarted')}</Badge>;
    }
  };

  const isAllSelected = filteredModels.length > 0 && filteredModels.every((model) => selectedModels.includes(model));
  const isIndeterminate = selectedModels.length > 0 && !isAllSelected;

  const failedModels = selectedModels.filter((model) => testResults[model]?.status === 'failed');

  const handleRemoveFailed = async () => {
    const failedModelNames = new Set(failedModels);
    const newSupportedModels = localSupportedModels.filter((model) => !failedModelNames.has(model));

    try {
      await updateChannel.mutateAsync({
        id: channel.id,
        input: {
          supportedModels: newSupportedModels,
        },
      });
      setLocalSupportedModels(newSupportedModels);
      setSelectedModels((prev) => prev.filter((model) => !failedModelNames.has(model)));
      setIsRemovePopoverOpen(false);
    } catch (error) {
      // Error is handled by useUpdateChannel toast
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col w-full max-w-full sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle className='text-base sm:text-lg'>{t('channels.dialogs.test.title')}</DialogTitle>
          <DialogDescription className='text-xs sm:text-sm'>{t('channels.dialogs.test.description', { name: channel.name })}</DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-3'>
          {/* Search + endpoint selector */}
          <div className='flex flex-col gap-2 sm:flex-row'>
            <div className='relative flex-1'>
              <IconSearch className='text-muted-foreground absolute top-1/2 left-3 h-3.5 w-3.5 -translate-y-1/2 transform' />
              <Input
                placeholder={t('channels.dialogs.test.searchPlaceholder')}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className='h-8 pl-9 text-sm'
              />
            </div>
            {resolvedEndpoints.length > 1 && (
              <div className='flex items-center gap-2'>
                <span className='text-muted-foreground shrink-0 text-xs whitespace-nowrap'>{t('channels.dialogs.test.endpointLabel')}</span>
                <Select value={apiFormat} onValueChange={setApiFormat}>
                  <SelectTrigger size='sm' className='h-8 w-full font-mono text-xs sm:w-64'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {resolvedEndpoints.map((ep) => (
                      <SelectItem key={ep.apiFormat} value={ep.apiFormat} className='font-mono text-xs'>
                        {ep.apiFormat}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>

          {/* Models Table */}
          <div className='min-h-0 flex-1 overflow-hidden rounded-lg border'>
            <div className='max-h-[28rem] overflow-auto'>
              <Table className='text-xs'>
                <TableHeader>
                  <TableRow>
                    <TableHead className='h-9 w-14 text-xs sm:w-12'>
                      <Checkbox
                        checked={isAllSelected}
                        onCheckedChange={handleSelectAll}
                        ref={(el) => {
                          if (el) {
                            const input = el.querySelector('input') as HTMLInputElement;
                            if (input) {
                              input.indeterminate = isIndeterminate;
                            }
                          }
                        }}
                        className='scale-100 sm:scale-75'
                      />
                    </TableHead>
                    <TableHead className='h-9 text-xs'>{t('channels.dialogs.test.modelNameColumn')}</TableHead>
                    <TableHead className='h-9 w-48 text-xs sm:w-64'>{t('channels.dialogs.test.statusColumn')}</TableHead>
                    <TableHead className='h-9 w-24 text-xs'></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredModels.map((model) => {
                    const result = testResults[model];
                    return (
                      <TableRow key={model} className='align-top'>
                        <TableCell className='py-2.5 align-top'>
                          <Checkbox
                            checked={selectedModels.includes(model)}
                            onCheckedChange={(checked) => handleModelSelect(model, !!checked)}
                            className='scale-100 sm:scale-75'
                          />
                        </TableCell>
                        <TableCell className='py-2.5 pr-4 align-top text-xs font-medium break-all sm:pr-8'>
                          <div>{model}</div>
                        </TableCell>
                        <TableCell className='w-48 min-w-[192px] py-2.5 align-top sm:w-64'>
                          <div className='flex flex-col items-start gap-1'>
                            {getStatusBadge(result?.status || 'not_started')}
                            {result?.latency && <div className='text-muted-foreground text-[10px]'>{result.latency.toFixed(2)}s</div>}
                            {/* Single-line truncated result under the status
                                badge; the full text lives in the click
                                popover so long errors never grow the row. */}
                            {(result?.error || result?.message) && (
                              <TestResultInline error={result?.error} message={result?.message} className='w-full min-w-0' />
                            )}
                          </div>
                        </TableCell>
                        <TableCell className='py-2.5 align-top'>
                          <Button
                            size='sm'
                            variant='outline'
                            onClick={() => testModel(model)}
                            disabled={result?.status === 'testing' || testChannel.isPending}
                            className='h-7 gap-1 px-2 text-xs'
                          >
                            <IconPlayerPlay className='h-3 w-3' />
                            {result?.status === 'testing' ? t('channels.dialogs.test.testingModel') : t('channels.dialogs.test.testModel')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          </div>
        </div>

        <DialogFooter className='flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-2'>
          <div className='flex flex-col sm:flex-row gap-2 w-full sm:w-auto'>
            <Button variant='outline' onClick={() => onOpenChange(false)} className='h-8 w-full text-xs sm:w-auto'>
              {t('common.buttons.cancel')}
            </Button>
            {failedModels.length > 0 && (
              <Popover open={isRemovePopoverOpen} onOpenChange={setIsRemovePopoverOpen}>
                <PopoverTrigger asChild>
                  <Button variant='destructive' size='sm' className='h-7 text-xs'>
                    {t('channels.dialogs.test.removeFailed')} ({failedModels.length})
                  </Button>
                </PopoverTrigger>
                <PopoverContent className='w-full sm:w-80'>
                  <div className='grid gap-4'>
                    <div className='space-y-2'>
                      <p className='text-muted-foreground text-sm'>{t('channels.dialogs.test.removeFailedConfirm')}</p>
                    </div>
                    <div className='flex justify-end gap-2'>
                      <Button
                        size='sm'
                        variant='destructive'
                        onClick={handleRemoveFailed}
                        disabled={updateChannel.isPending}
                        className='h-9 sm:h-8'
                      >
                        {updateChannel.isPending ? t('common.buttons.saving') : t('common.buttons.confirm')}
                      </Button>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            )}
          </div>
          <Button onClick={handleTestSelected} disabled={selectedModels.length === 0 || isTesting} className='h-8 text-xs sm:h-8'>
            <IconPlayerPlay className='mr-2 h-4 w-4' />
            {t('channels.dialogs.test.testAllButton', { count: selectedModels.length })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
