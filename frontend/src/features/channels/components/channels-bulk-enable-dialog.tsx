'use client';

import { IconAlertTriangle, IconCheck } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useChannels } from '../context/channels-context';
import { useBulkEnableChannels } from '../data/channels';

export function ChannelsBulkEnableDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedChannels, resetRowSelection, setSelectedChannels } = useChannels();
  const bulkEnableChannels = useBulkEnableChannels();

  const isDialogOpen = open === 'bulkEnable';
  const selectedCount = selectedChannels.length;

  if (selectedCount === 0 && !isDialogOpen) {
    return null;
  }

  const handleConfirm = async () => {
    try {
      const ids = selectedChannels.map((channel) => channel.id);
      if (ids.length === 0) {
        return;
      }

      await bulkEnableChannels.mutateAsync(ids);
      resetRowSelection();
      setSelectedChannels([]);
      setOpen(null);
    } catch (error) {
    }
  };

  return (
    <ConfirmDialog
      open={isDialogOpen}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          setOpen(null);
        } else {
          setOpen('bulkEnable');
        }
      }}
      handleConfirm={handleConfirm}
      disabled={selectedCount === 0}
      isLoading={bulkEnableChannels.isPending}
      confirmText={t('common.buttons.enable')}
      cancelBtnText={t('common.buttons.cancel')}
      title={
        <span className='text-primary flex items-center gap-2'>
          <IconAlertTriangle className='h-4 w-4' />
          {t('channels.dialogs.bulkEnable.title')}
        </span>
      }
      desc={t('channels.dialogs.bulkEnable.description', { count: selectedCount })}
    >
      <div className='flex items-start gap-3 rounded-md border border-success/40 bg-success/10 p-3 text-sm dark:border-success/40 dark:bg-success/20'>
        <IconCheck className='mt-0.5 h-4 w-4 text-(--success-soft-fg) dark:text-(--success-soft-fg)' />
        <div className='space-y-1 text-left'>
          <p>{t('channels.dialogs.bulkEnable.warning')}</p>
        </div>
      </div>
    </ConfirmDialog>
  );
}
