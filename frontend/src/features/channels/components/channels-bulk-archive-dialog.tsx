'use client';

import { IconAlertTriangle, IconArchive } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useChannels } from '../context/channels-context';
import { useBulkArchiveChannels } from '../data/channels';

export function ChannelsBulkArchiveDialog() {
  const { t } = useTranslation();
  const { open, setOpen, selectedChannels, resetRowSelection, setSelectedChannels } = useChannels();
  const bulkArchiveChannels = useBulkArchiveChannels();

  const isDialogOpen = open === 'bulkArchive';
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

      await bulkArchiveChannels.mutateAsync(ids);
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
          setOpen('bulkArchive');
        }
      }}
      handleConfirm={handleConfirm}
      disabled={selectedCount === 0}
      isLoading={bulkArchiveChannels.isPending}
      confirmText={t('common.buttons.archive')}
      cancelBtnText={t('common.buttons.cancel')}
      title={
        <span className='text-destructive flex items-center gap-2'>
          <IconAlertTriangle className='h-4 w-4' />
          {t('channels.dialogs.bulkArchive.title')}
        </span>
      }
      desc={t('channels.dialogs.bulkArchive.description', { count: selectedCount })}
    >
      <div className='flex items-start gap-3 rounded-md border border-info/40 bg-info/10 p-3 text-sm dark:border-info/40 dark:bg-info/20'>
        <IconArchive className='mt-0.5 h-4 w-4 text-(--info-soft-fg) dark:text-(--info-soft-fg)' />
        <div className='space-y-1 text-left'>
          <p>{t('channels.dialogs.bulkArchive.warning')}</p>
        </div>
      </div>
    </ConfirmDialog>
  );
}
