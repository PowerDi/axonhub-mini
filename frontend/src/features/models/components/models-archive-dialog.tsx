'use client';

import { IconArchive, IconCheck, IconInfoCircle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useModels } from '../context/models-context';
import { useUpdateModelStatus } from '../data/models';

export function ModelsArchiveDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentRow } = useModels();
  const updateModelStatus = useUpdateModelStatus();

  const isOpen = open === 'archive';
  const isArchived = currentRow?.status === 'archived';

  const handleStatusChange = () => {
    if (!currentRow) return;
    updateModelStatus.mutate(
      { id: currentRow.id, status: isArchived ? 'enabled' : 'archived' },
      { onSuccess: () => setOpen(null) },
    );
  };

  const handleClose = () => {
    setOpen(null);
  };

  const getDescription = () => {
    if (!currentRow) return null;
    const baseDescription = t(isArchived ? 'models.dialogs.status.restoreDescription' : 'models.dialogs.status.archiveDescription', {
      name: currentRow.name,
    });
    const infoText = t(isArchived ? 'models.dialogs.status.restoreInfo' : 'models.dialogs.status.archiveWarning');

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
      open={isOpen}
      onOpenChange={handleClose}
      handleConfirm={handleStatusChange}
      disabled={updateModelStatus.isPending}
      title={
        <span className={isArchived ? 'text-(--success-soft-fg)' : 'text-(--warning-soft-fg)'}>
          {isArchived ? <IconCheck className='mr-1 inline-block stroke-success' size={18} /> : <IconArchive className='mr-1 inline-block stroke-warning' size={18} />}
          {t(isArchived ? 'models.dialogs.status.restoreTitle' : 'models.dialogs.status.archiveTitle')}
        </span>
      }
      desc={getDescription()}
      confirmText={t(isArchived ? 'common.buttons.restore' : 'common.buttons.archive')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
