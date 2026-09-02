import { Skeleton } from '@/components/ui/skeleton';
import { TableCell, TableRow } from '@/components/ui/table';

interface TableSkeletonProps {
  rows?: number;
  columns?: number;
}

export function TableSkeleton({ rows = 5, columns = 1 }: TableSkeletonProps) {
  return (
    <>
      {Array.from({ length: rows }).map((_, rowIndex) => (
        <TableRow key={rowIndex} className='group/row hover:bg-transparent'>
          <TableCell colSpan={columns}>
            <Skeleton className='h-4 w-full rounded-md' />
          </TableCell>
        </TableRow>
      ))}
    </>
  );
}
