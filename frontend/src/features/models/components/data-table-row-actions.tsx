import { useState } from 'react';
import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import { Row } from '@tanstack/react-table';
import { IconEdit, IconArchive, IconTrash, IconNote, IconArchiveOff } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { PermissionGuard } from '@/components/permission-guard';
import { useModels } from '../context/models-context';
import { Model } from '../data/schema';

interface DataTableRowActionsProps {
  row: Row<Model>;
}

export function DataTableRowActions({ row }: DataTableRowActionsProps) {
  const { t } = useTranslation();
  const { setOpen, setCurrentRow } = useModels();
  const { channelPermissions } = usePermissions();
  const model = row.original;
  const [menuOpen, setMenuOpen] = useState(false);

  if (!channelPermissions.canWrite) {
    return null;
  }

  const openRowDialog = (dialog: Parameters<typeof setOpen>[0]) => {
    setMenuOpen(false);
    setCurrentRow(row.original);
    window.setTimeout(() => setOpen(dialog), 0);
  };

  return (
    <div className='flex items-center gap-1'>
      <Button
        variant='ghost'
        size='icon'
        onClick={() => {
          setCurrentRow(row.original);
          setOpen('edit');
        }}
        data-testid='row-edit-button'
      >
        <IconEdit size={16} />
        <span className='sr-only'>{t('common.actions.edit')}</span>
      </Button>
      <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
        <DropdownMenuTrigger asChild>
          <Button variant='ghost' size='icon' className='data-[state=open]:bg-muted' data-testid='row-actions'>
            <DotsHorizontalIcon className='h-4 w-4' />
            <span className='sr-only'>{t('common.actions.openMenu')}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[160px]'>
          <PermissionGuard requiredScope='write_channels'>
            <>
              <DropdownMenuItem onClick={() => openRowDialog('edit')}>
                <IconEdit size={16} className='mr-2' />
                {t('common.actions.edit')}
              </DropdownMenuItem>

              <DropdownMenuItem onClick={() => openRowDialog('association')}>
                <IconNote size={16} className='mr-2' />
                {t('models.actions.manageAssociation')}
              </DropdownMenuItem>

              {channelPermissions.canRead && <DropdownMenuSeparator />}

              {model.status !== 'archived' ? (
                <DropdownMenuItem onClick={() => openRowDialog('archive')} className='text-(--warning-soft-fg)! hover:bg-warning/10!'>
                  <IconArchive size={16} className='mr-2' />
                  {t('common.buttons.archive')}
                </DropdownMenuItem>
              ) : (
                <DropdownMenuItem onClick={() => openRowDialog('archive')} className='text-(--success-soft-fg)! hover:bg-success/10!'>
                  <IconArchiveOff size={16} className='mr-2' />
                  {t('common.buttons.restore')}
                </DropdownMenuItem>
              )}

              <DropdownMenuItem
                onClick={() => openRowDialog('delete')}
                className='text-(--destructive-soft-fg)! hover:bg-destructive/10!'
              >
                <IconTrash size={16} className='mr-2' />
                {t('common.buttons.delete')}
              </DropdownMenuItem>
            </>
          </PermissionGuard>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
