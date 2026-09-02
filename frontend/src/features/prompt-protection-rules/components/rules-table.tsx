import { useEffect, useMemo, useState } from 'react';
import {
  ColumnDef,
  ColumnFiltersState,
  RowSelectionState,
  SortingState,
  VisibilityState,
  flexRender,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table';
import { IconBan, IconCheck, IconTrash, IconX } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { TableSkeleton } from '@/components/ui/table-skeleton';
import { ServerSidePagination } from '@/components/server-side-pagination';
import { PermissionGuard } from '@/components/permission-guard';
import { usePromptProtectionRules } from '../context/rules-context';
import { PromptProtectionRule, PromptProtectionRuleConnection } from '../data/schema';

interface RulesTableProps {
  columns: ColumnDef<PromptProtectionRule>[];
  data: PromptProtectionRule[];
  loading?: boolean;
  pageInfo?: PromptProtectionRuleConnection['pageInfo'];
  pageSize: number;
  totalCount?: number;
  nameFilter: string;
  sorting: SortingState;
  onSortingChange: (updater: SortingState | ((prev: SortingState) => SortingState)) => void;
  onNextPage: () => void;
  onPreviousPage: () => void;
  onPageSizeChange: (pageSize: number) => void;
  onNameFilterChange: (filter: string) => void;
  canWrite?: boolean;
}

export function RulesTable({
  columns,
  data,
  loading,
  pageInfo,
  pageSize,
  totalCount,
  nameFilter,
  sorting,
  onSortingChange,
  onNextPage,
  onPreviousPage,
  onPageSizeChange,
  onNameFilterChange,
  canWrite = true,
}: RulesTableProps) {
  const { t } = useTranslation();
  const { setSelectedRules, setResetRowSelection, setOpen } = usePromptProtectionRules();
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);

  useEffect(() => {
    const newColumnFilters: ColumnFiltersState = [];
    if (nameFilter) {
      newColumnFilters.push({ id: 'name', value: nameFilter });
    }
    setColumnFilters(newColumnFilters);
  }, [nameFilter]);

  const handleColumnFiltersChange = (updater: ColumnFiltersState | ((prev: ColumnFiltersState) => ColumnFiltersState)) => {
    const newFilters = typeof updater === 'function' ? updater(columnFilters) : updater;
    setColumnFilters(newFilters);

    const nextFilter = (newFilters.find((filter) => filter.id === 'name')?.value as string) || '';
    if (nextFilter !== nameFilter) {
      onNameFilterChange(nextFilter);
    }
  };

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
    },
    enableRowSelection: true,
    getRowId: (row) => row.id,
    onRowSelectionChange: setRowSelection,
    onSortingChange,
    onColumnFiltersChange: handleColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    manualPagination: true,
    manualFiltering: true,
  });

  const filteredSelectedRows = useMemo(() => table.getFilteredSelectedRowModel().rows, [data, rowSelection, table]);
  const selectedCount = filteredSelectedRows.length;

  useEffect(() => {
    setResetRowSelection(() => () => setRowSelection({}));
  }, [setResetRowSelection]);

  useEffect(() => {
    setSelectedRules(filteredSelectedRows.map((row) => row.original as PromptProtectionRule));
  }, [filteredSelectedRows, setSelectedRules]);

  useEffect(() => {
    if (Object.keys(rowSelection).length > 0 && data.length > 0) {
      const dataIds = new Set(data.map((item) => item.id));
      const selectedIds = Object.keys(rowSelection);
      if (selectedIds.some((id) => !dataIds.has(id))) {
        setRowSelection({});
      }
    }
  }, [data, rowSelection]);

  return (
    <div className='flex flex-1 flex-col gap-3 overflow-hidden'>
      <div className='flex flex-wrap items-center gap-2 sm:gap-3'>
        <Input
          placeholder={t('promptProtectionRules.filters.filterByName')}
          value={(table.getColumn('name')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('name')?.setFilterValue(event.target.value)}
          className='w-[200px] lg:w-[240px]'
        />
      </div>

      <div className='relative flex-1 overflow-auto rounded-lg border'>
        <Table>
          <TableHeader className='sticky top-0 z-20'>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className='group/row'>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    colSpan={header.colSpan}
                    className={header.column.columnDef.meta?.className ?? ''}
                  >
                    {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableSkeleton rows={pageSize} columns={columns.length} />
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} data-state={row.getIsSelected() && 'selected'} className='group/row'>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className={cell.column.columnDef.meta?.className ?? ''}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow className='hover:bg-transparent'>
                <TableCell colSpan={columns.length} className='h-24 text-center'>
                  {t('common.noData')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className='flex-shrink-0'>
        <ServerSidePagination
          pageInfo={pageInfo}
          pageSize={pageSize}
          dataLength={data.length}
          totalCount={totalCount}
          selectedRows={selectedCount}
          onNextPage={onNextPage}
          onPreviousPage={onPreviousPage}
          onPageSizeChange={onPageSizeChange}
        />
      </div>

      {selectedCount > 0 && canWrite && (
        <div className='fixed bottom-6 left-1/2 z-50 -translate-x-1/2'>
          <div className='bg-popover flex items-center gap-2 rounded-lg px-2 py-1.5 shadow-lg ring-1 ring-foreground/10'>
            <div className='flex items-center gap-1.5 px-1'>
              <Badge variant='secondary' className='tabular-nums'>
                {selectedCount}
              </Badge>
              <span className='text-muted-foreground text-sm'>{t('common.selected')}</span>
            </div>
            <div className='bg-border mx-1 h-6 w-px' />
            <PermissionGuard requiredScope='write_channels'>
              <>
                <Button variant='ghost' size='icon' className='text-(--success-soft-fg) hover:bg-success/10' onClick={() => setOpen('bulkEnable')} title={t('common.buttons.enable')}>
                  <IconCheck className='size-4' />
                </Button>
                <Button variant='ghost' size='icon' className='text-(--warning-soft-fg) hover:bg-warning/10' onClick={() => setOpen('bulkDisable')} title={t('common.buttons.disable')}>
                  <IconBan className='size-4' />
                </Button>
                <Button variant='ghost' size='icon' className='text-(--destructive-soft-fg) hover:bg-destructive/10' onClick={() => setOpen('bulkDelete')} title={t('common.buttons.delete')}>
                  <IconTrash className='size-4' />
                </Button>
              </>
            </PermissionGuard>
            <div className='bg-border mx-1 h-6 w-px' />
            <Button variant='ghost' size='icon' onClick={() => setRowSelection({})}>
              <IconX className='size-4' />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
