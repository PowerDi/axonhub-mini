'use client';

import { IconArchive, IconCheck, IconInfoCircle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useApiKeysContext } from '../context/apikeys-context';
import { useUpdateApiKeyStatus } from '../data/apikeys';

export function ApiKeysArchiveDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedApiKey, resetRowSelection } = useApiKeysContext();
  const updateApiKeyStatus = useUpdateApiKeyStatus();

  if (!selectedApiKey) return null;

  const isArchived = selectedApiKey.status === 'archived';

  const handleArchive = async () => {
    try {
      await updateApiKeyStatus.mutateAsync({
        id: selectedApiKey.id,
        status: isArchived ? 'enabled' : 'archived',
      });
      closeDialog('archive');
      resetRowSelection();
    } catch (_error) {
      // Error will be handled by the mutation's error state
    }
  };

  const getDescription = () => {
    const baseDescription = t(
      isArchived ? 'apikeys.dialogs.archive.restoreDescription' : 'apikeys.dialogs.archive.description',
      { name: selectedApiKey.name }
    );
    const warningText = t(
      isArchived ? 'apikeys.dialogs.archive.restoreInfo' : 'apikeys.dialogs.archive.warning'
    );

    return (
      <div className='space-y-3'>
        <p>{baseDescription}</p>
        <div className='rounded-md border border-info/40 bg-info/10 p-3 dark:border-info/40 dark:bg-info/20'>
          <div className='flex items-start space-x-2'>
            <IconInfoCircle className='mt-0.5 h-4 w-4 flex-shrink-0 text-(--info-soft-fg) dark:text-(--info-soft-fg)' />
            <div className='text-sm text-(--info-soft-fg) dark:text-(--info-soft-fg)'>
              <p>{warningText}</p>
            </div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <ConfirmDialog
      open={isDialogOpen.archive}
      onOpenChange={() => closeDialog('archive')}
      handleConfirm={handleArchive}
      disabled={updateApiKeyStatus.isPending}
      title={
        <span className={isArchived ? 'text-(--success-soft-fg)' : 'text-(--warning-soft-fg)'}>
          {isArchived ? (
            <IconCheck className='mr-1 inline-block stroke-success' size={18} />
          ) : (
            <IconArchive className='mr-1 inline-block stroke-warning' size={18} />
          )}
          {t(isArchived ? 'apikeys.dialogs.archive.restoreTitle' : 'apikeys.dialogs.archive.title')}
        </span>
      }
      desc={getDescription()}
      confirmText={t(isArchived ? 'common.buttons.restore' : 'common.buttons.archive')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
