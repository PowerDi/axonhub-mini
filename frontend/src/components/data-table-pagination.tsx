'use client';

import { ChevronLeftIcon, ChevronRightIcon, DoubleArrowLeftIcon, DoubleArrowRightIcon } from '@radix-ui/react-icons';
import { Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

interface DataTablePaginationProps<TData> {
  table: Table<TData>;
}

/** Bottom bar per spec §3.5: right-aligned showing-info + page-size select + nav buttons,
 *  current page rendered as a primary button with tabular numerals. */
export function DataTablePagination<TData>({ table }: DataTablePaginationProps<TData>) {
  const { t } = useTranslation();
  const currentPage = table.getState().pagination.pageIndex + 1;
  const totalPages = table.getPageCount();
  const pageSize = table.getState().pagination.pageSize;
  const selectedRows = table.getFilteredSelectedRowModel().rows.length;
  const totalRows = table.getFilteredRowModel().rows.length;
  const rangeStart = totalRows === 0 ? 0 : (currentPage - 1) * pageSize + 1;
  const rangeEnd = Math.min(currentPage * pageSize, totalRows);

  return (
    <div className='text-muted-foreground flex flex-wrap items-center justify-end gap-x-3 gap-y-2 px-1 text-xs sm:text-sm'>
      {selectedRows > 0 && <span>{t('pagination.selectedRows', { selected: selectedRows, total: totalRows })}</span>}
      <span>{t('pagination.showing', { start: rangeStart, end: rangeEnd, total: totalRows })}</span>
      <Select
        value={`${pageSize}`}
        onValueChange={(value) => {
          table.setPageSize(Number(value));
        }}
      >
        <SelectTrigger className='h-8 w-[70px]' aria-label={t('pagination.rowsPerPage')}>
          <SelectValue placeholder={pageSize} />
        </SelectTrigger>
        <SelectContent side='top'>
          {[10, 20, 30, 40, 50, 100].map((size) => (
            <SelectItem key={size} value={`${size}`}>
              {size}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <div className='flex items-center gap-1'>
        <Button
          variant='outline'
          size='icon'
          className='hidden lg:inline-flex'
          onClick={() => table.setPageIndex(0)}
          disabled={!table.getCanPreviousPage()}
        >
          <span className='sr-only'>{t('pagination.firstPage')}</span>
          <DoubleArrowLeftIcon className='size-4' />
        </Button>
        <Button variant='outline' size='icon' onClick={() => table.previousPage()} disabled={!table.getCanPreviousPage()}>
          <span className='sr-only'>{t('pagination.previousPage')}</span>
          <ChevronLeftIcon className='size-4' />
        </Button>
        <Button variant='default' size='icon' className='min-w-8 px-1 font-semibold' aria-current='page'>
          {currentPage}
        </Button>
        <span className='px-0.5 tabular-nums'>/ {totalPages}</span>
        <Button variant='outline' size='icon' onClick={() => table.nextPage()} disabled={!table.getCanNextPage()}>
          <span className='sr-only'>{t('pagination.nextPage')}</span>
          <ChevronRightIcon className='size-4' />
        </Button>
        <Button
          variant='outline'
          size='icon'
          className='hidden lg:inline-flex'
          onClick={() => table.setPageIndex(table.getPageCount() - 1)}
          disabled={!table.getCanNextPage()}
        >
          <span className='sr-only'>{t('pagination.lastPage')}</span>
          <DoubleArrowRightIcon className='size-4' />
        </Button>
      </div>
    </div>
  );
}
