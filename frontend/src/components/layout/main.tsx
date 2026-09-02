import React from 'react';
import { cn } from '@/lib/utils';

interface MainProps extends React.HTMLAttributes<HTMLElement> {
  fixed?: boolean;
  ref?: React.Ref<HTMLElement>;
}

/** List-page container: p-4 base padding (spec §4); hub/settings pages can
 *  override with className='p-6'. */
export const Main = ({ fixed, className, ...props }: MainProps) => {
  return (
    <main
      className={cn('p-4', fixed && 'fixed-main flex min-h-0 min-w-0 grow flex-col overflow-hidden', className)}
      {...props}
    />
  );
};

Main.displayName = 'Main';
