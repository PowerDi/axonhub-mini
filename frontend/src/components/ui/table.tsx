import * as React from 'react';
import { cn } from '@/lib/utils';

/**
 * Table primitives restyled to the metapi-go data-table spec (§3.5):
 * hairline rows, no zebra stripes, faint header tint, 40px header / 60px rows
 * (row height acts as a minimum — multi-line content grows), tabular numerals.
 */
const Table = React.forwardRef<HTMLTableElement, React.HTMLAttributes<HTMLTableElement>>(({ className, ...props }, ref) => (
  <table ref={ref} data-slot='table' className={cn('w-full caption-bottom text-sm tabular-nums', className)} {...props} />
));
Table.displayName = 'Table';

/** Scrollable outer frame: `rounded-lg border overflow-hidden` container that hugs the table. */
function TableContainer({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot='table-container' className={cn('relative w-full overflow-x-auto rounded-lg border', className)} {...props} />;
}

function TableHeader({ className, ...props }: React.ComponentProps<'thead'>) {
  // [&_tr]:h-10 re-pins header rows to the 40px header height — the 60px row
  // minimum on TableRow must not leak into the header.
  return <thead data-slot='table-header' className={cn('[&_tr]:h-10 [&_tr]:border-b [&_tr]:hover:bg-transparent', className)} {...props} />;
}

function TableBody({ className, ...props }: React.ComponentProps<'tbody'>) {
  return <tbody data-slot='table-body' className={cn('[&_tr:last-child]:border-0', className)} {...props} />;
}

function TableFooter({ className, ...props }: React.ComponentProps<'tfoot'>) {
  return <tfoot data-slot='table-footer' className={cn('bg-(--table-header) border-t font-medium [&>tr]:last:border-b-0', className)} {...props} />;
}

const TableRow = React.forwardRef<HTMLTableRowElement, React.ComponentProps<'tr'>>(({ className, ...props }, ref) => (
  <tr
    ref={ref}
    data-slot='table-row'
    // h-[60px] on a <tr> behaves as a minimum height: multi-line cells still grow the row
    className={cn(
      'hover:bg-(--table-row-hover) data-[state=selected]:bg-(--table-row-selected) h-[60px] border-b transition-colors duration-100',
      className
    )}
    {...props}
  />
));
TableRow.displayName = 'TableRow';

function TableHead({ className, ...props }: React.ComponentProps<'th'>) {
  return (
    <th
      data-slot='table-head'
      className={cn(
        'text-foreground bg-(--table-header) h-10 px-2 text-left align-middle font-medium whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]',
        className
      )}
      {...props}
    />
  );
}

function TableCell({ className, ...props }: React.ComponentProps<'td'>) {
  return (
    <td
      data-slot='table-cell'
      className={cn('p-2 align-middle whitespace-nowrap [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]', className)}
      {...props}
    />
  );
}

function TableCaption({ className, ...props }: React.ComponentProps<'caption'>) {
  return <caption data-slot='table-caption' className={cn('text-muted-foreground mt-4 text-sm', className)} {...props} />;
}

export { Table, TableContainer, TableHeader, TableBody, TableFooter, TableHead, TableRow, TableCell, TableCaption };
