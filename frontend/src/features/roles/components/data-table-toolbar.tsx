import { Cross2Icon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  isFiltered?: boolean;
}

export function DataTableToolbar<TData>({ table, isFiltered: externalIsFiltered }: DataTableToolbarProps<TData>) {
  const { t } = useTranslation();
  const tableState = table.getState();
  const isFiltered = externalIsFiltered ?? tableState.columnFilters.length > 0;

  return (
    <div className='flex flex-wrap items-center gap-2 sm:gap-3'>
        <Input
          placeholder={t('roles.searchRoles')}
          value={(table.getColumn('search')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('search')?.setFilterValue(event.target.value)}
          className='w-[200px] lg:w-[240px]'
        />
        {isFiltered && (
          <Button variant='ghost' onClick={() => table.resetColumnFilters()} className='px-2 lg:px-2.5'>
            {t('common.filters.reset')}
            <Cross2Icon className='size-3.5' />
          </Button>
        )}
    </div>
  );
}
