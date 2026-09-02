'use client';

import { ChevronLeftIcon, ChevronRightIcon, DoubleArrowLeftIcon } from '@radix-ui/react-icons';
import type { PageInfo } from '@/gql/pagination';
import { useTranslation } from 'react-i18next';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

interface ServerSidePaginationProps {
  pageInfo?: PageInfo;
  pageSize: number;
  dataLength: number;
  totalCount?: number;
  selectedRows: number;
  onNextPage: () => void;
  onPreviousPage: () => void;
  onFirstPage?: () => void;
  onPageSizeChange: (pageSize: number) => void;
  onResetCursor?: () => void;
}

export function ServerSidePagination({
  pageInfo,
  pageSize,
  dataLength,
  totalCount,
  selectedRows,
  onNextPage,
  onPreviousPage,
  onFirstPage,
  onPageSizeChange,
  onResetCursor,
}: ServerSidePaginationProps) {
  const { t } = useTranslation();
  const { resetCursor } = usePaginationSearch({
    defaultPageSize: 20,
  });

  return (
    <div className='text-muted-foreground flex flex-wrap items-center justify-end gap-x-3 gap-y-2 px-1 text-xs sm:text-sm'>
      <span className='hidden sm:inline'>
        {totalCount !== undefined
          ? t('pagination.selectedInfoWithTotal', { selectedRows, dataLength, totalCount })
          : t('pagination.selectedInfo', { selectedRows, dataLength })}
      </span>
      <div className='flex flex-wrap items-center gap-3'>
        <div className='flex items-center gap-1'>
          <p className='hidden text-sm sm:block'>{t('pagination.rowsPerPage')}</p>
          <Select
            value={`${pageSize}`}
            onValueChange={(value) => {
              onPageSizeChange(Number(value));
            }}
          >
            <SelectTrigger className='h-8 w-[70px]'>
              <SelectValue placeholder={pageSize} />
            </SelectTrigger>
            <SelectContent side='top'>
              {[10, 20, 30, 40, 50].map((size) => (
                <SelectItem key={size} value={`${size}`}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='flex items-center justify-center'>
          <div className='flex items-center gap-1'>
            <span className='text-muted-foreground'>
              {pageInfo?.hasPreviousPage ? t('pagination.hasPrevious') : t('pagination.firstPage')}
            </span>
            <span className='text-muted-foreground'>|</span>
            <span className='text-muted-foreground'>{pageInfo?.hasNextPage ? t('pagination.hasNext') : t('pagination.lastPage')}</span>
          </div>
        </div>
        <div className='flex items-center gap-1'>
          <Button
            variant='outline'
            size='icon'
            className='hidden lg:inline-flex'
            onClick={onFirstPage || onResetCursor || resetCursor}
            disabled={!pageInfo?.hasPreviousPage}
          >
            <span className='sr-only'>{t('pagination.firstPage')}</span>
            <DoubleArrowLeftIcon className='size-4' />
          </Button>
          <Button variant='outline' size='icon' onClick={onPreviousPage} disabled={!pageInfo?.hasPreviousPage}>
            <span className='sr-only'>{t('pagination.previousPage')}</span>
            <ChevronLeftIcon className='size-4' />
          </Button>
          <Button variant='outline' size='icon' onClick={onNextPage} disabled={!pageInfo?.hasNextPage}>
            <span className='sr-only'>{t('pagination.nextPage')}</span>
            <ChevronRightIcon className='size-4' />
          </Button>
          {/* NOT SUPPORTED */}
          {/* <Button
            variant='outline'
            className='hidden lg:inline-flex'
            onClick={onNextPage}
            disabled={!pageInfo?.hasNextPage}
          >
            <span className='sr-only'>{t('pagination.lastPage')}</span>
            <DoubleArrowRightIcon className='h-4 w-4' />
          </Button> */}
        </div>
      </div>
    </div>
  );
}
