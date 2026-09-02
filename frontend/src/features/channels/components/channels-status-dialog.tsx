'use client';

import { useState } from 'react';
import { IconAlertTriangle, IconFlask, IconLoader2 } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useUpdateChannelStatus, useTestChannel } from '../data/channels';
import { Channel } from '../data/schema';
import { ErrorDisplay } from '../utils/error-formatter';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

export function ChannelsStatusDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannelStatus = useUpdateChannelStatus();
  const testChannel = useTestChannel();
  const [testResult, setTestResult] = useState<{
    success: boolean;
    latency?: number;
    message?: string | null;
    error?: string | null;
  } | null>(null);

  const handleStatusChange = async () => {
    const newStatus = currentRow.status === 'enabled' ? 'disabled' : 'enabled';

    try {
      await updateChannelStatus.mutateAsync({
        id: currentRow.id,
        status: newStatus,
      });
      onOpenChange(false);
      setTestResult(null);
    } catch (error) {
    }
  };

  const handleTestChannel = async () => {
    try {
      const result = await testChannel.mutateAsync({
        channelID: currentRow.id,
        modelID: currentRow.defaultTestModel || undefined,
      });
      setTestResult({
        success: result.success,
        latency: result.latency,
        message: result.message,
        error: result.error,
      });
    } catch (error) {
      setTestResult({
        success: false,
        error: error instanceof Error ? error.message : 'Unknown error occurred',
      });
    }
  };

  const isDisabling = currentRow.status === 'enabled';
  const title = isDisabling ? t('channels.dialogs.status.disable.title') : t('channels.dialogs.status.enable.title');

  // Enhanced description with warning for enabling
  const getDescription = () => {
    if (isDisabling) {
      return t('channels.dialogs.status.disable.description', { name: currentRow.name });
    }

    const baseDescription = t('channels.dialogs.status.enable.description', { name: currentRow.name });
    const warningText = t('channels.dialogs.status.enable.warning');
    return (
      <div className='min-w-0 space-y-3'>
        <p>{baseDescription}</p>
        <div className='rounded-md border border-warning/40 bg-warning/10 p-3 dark:border-warning/40 dark:bg-warning/20'>
          <div className='flex items-start space-x-2'>
            <IconAlertTriangle className='mt-0.5 h-4 w-4 flex-shrink-0 text-(--warning-soft-fg) dark:text-(--warning-soft-fg)' />
            <div className='text-sm text-(--warning-soft-fg) dark:text-(--warning-soft-fg)'>
              <p className='font-medium'>{t('channels.dialogs.status.enable.warningTitle')}</p>
              <p className='mt-1'>{warningText}</p>
            </div>
          </div>
        </div>

        {/* Test section */}
        {currentRow.defaultTestModel && (
          <div className='space-y-2'>
            <div className='flex items-center justify-between'>
              <span className='text-sm font-medium'>{t('channels.dialogs.status.enable.testRecommended')}</span>
              <Button variant='outline' size='sm' onClick={handleTestChannel} disabled={testChannel.isPending} className='h-8'>
                {testChannel.isPending ? <IconLoader2 className='mr-1 h-3 w-3 animate-spin' /> : <IconFlask className='mr-1 h-3 w-3' />}
                {t('channels.dialogs.status.enable.testButton')}
              </Button>
            </div>

            {testResult && (
              <div
                className={`rounded p-3 text-sm ${
                  testResult.success
                    ? 'border border-success/40 bg-success/10 text-(--success-soft-fg) dark:border-success/40 dark:bg-success/20 dark:text-(--success-soft-fg)'
                    : 'border border-destructive/40 bg-destructive/10 text-(--destructive-soft-fg) dark:border-destructive/40 dark:bg-destructive/20 dark:text-(--destructive-soft-fg)'
                }`}
              >
                <div className='space-y-2'>
                  <div className='font-medium'>
                    {testResult.success
                      ? t('channels.dialogs.status.enable.testSuccess', { latency: testResult.latency?.toFixed(2) })
                      : t('channels.dialogs.status.enable.testFailed')}
                  </div>

                  {/* Show test message if available */}
                  {testResult.message && testResult.success && (
                    <div className='text-xs opacity-75'>
                      <span className='font-medium'>{t('channels.dialogs.status.enable.testMessage')}:</span> {testResult.message}
                    </div>
                  )}

                  {/* Show detailed error if test failed */}
                  {testResult.error && !testResult.success && (
                    <div className='text-xs'>
                      <span className='font-medium'>{t('channels.dialogs.status.enable.errorDetails')}:</span>
                      <div className='mt-1 min-w-0 max-w-full overflow-hidden rounded border-l-2 border-destructive/40 bg-destructive/10 p-2 dark:border-destructive/40 dark:bg-destructive/30'>
                        <ErrorDisplay
                          error={testResult.error}
                          className='min-w-0 max-w-full'
                          messageClassName='block max-w-full break-all whitespace-pre-wrap text-xs font-medium text-(--destructive-soft-fg) dark:text-(--destructive-soft-fg)'
                        />
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    );
  };

  const actionText = isDisabling ? t('common.buttons.disable') : t('common.buttons.enable');

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleStatusChange}
      disabled={updateChannelStatus.isPending}
      title={
        <span className={isDisabling ? 'text-destructive' : 'text-(--success-soft-fg)'}>
          <IconAlertTriangle className={`${isDisabling ? 'stroke-destructive' : 'stroke-success'} mr-1 inline-block`} size={18} />
          {title}
        </span>
      }
      desc={getDescription()}
      confirmText={actionText}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
