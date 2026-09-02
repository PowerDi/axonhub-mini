'use client';

import { IconArchive, IconCheck, IconInfoCircle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useUpdateChannelStatus } from '../data/channels';
import { Channel } from '../data/schema';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow: Channel;
}

export function ChannelsArchiveDialog({ open, onOpenChange, currentRow }: Props) {
  const { t } = useTranslation();
  const updateChannelStatus = useUpdateChannelStatus();
  const isArchived = currentRow.status === 'archived';

  const handleStatusChange = async () => {
    try {
      await updateChannelStatus.mutateAsync({
        id: currentRow.id,
        status: isArchived ? 'enabled' : 'archived',
      });
      onOpenChange(false);
    } catch (_error) {
      // Error will be handled by the mutation's error state
    }
  };

  const getDescription = () => {
    const baseDescription = t(isArchived ? 'channels.dialogs.status.restore.description' : 'channels.dialogs.status.archive.description', {
      name: currentRow.name,
    });
    const infoText = t(isArchived ? 'channels.dialogs.status.restore.info' : 'channels.dialogs.status.archive.warning');

    return (
      <div className='space-y-3'>
        <p>{baseDescription}</p>
        <div className='rounded-md border border-info/40 bg-info/10 p-3 dark:border-info/40 dark:bg-info/20'>
          <div className='flex items-start space-x-2'>
            <IconInfoCircle className='mt-0.5 h-4 w-4 flex-shrink-0 text-(--info-soft-fg) dark:text-(--info-soft-fg)' />
            <div className='text-sm text-(--info-soft-fg) dark:text-(--info-soft-fg)'>
              <p>{infoText}</p>
            </div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={handleStatusChange}
      disabled={updateChannelStatus.isPending}
      title={
        <span className={isArchived ? 'text-(--success-soft-fg)' : 'text-(--warning-soft-fg)'}>
          {isArchived ? <IconCheck className='mr-1 inline-block stroke-success' size={18} /> : <IconArchive className='mr-1 inline-block stroke-warning' size={18} />}
          {t(isArchived ? 'channels.dialogs.status.restore.title' : 'channels.dialogs.status.archive.title')}
        </span>
      }
      desc={getDescription()}
      confirmText={t(isArchived ? 'common.buttons.restore' : 'common.buttons.archive')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
