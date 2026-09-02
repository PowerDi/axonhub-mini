import React from 'react';
import { cn } from '@/lib/utils';

interface HeaderProps extends React.HTMLAttributes<HTMLElement> {
  /** Kept for API compatibility — page heads are now in-flow (spec §4),
   *  only the 56px app topbar stays fixed. */
  fixed?: boolean;
  ref?: React.Ref<HTMLElement>;
}

/** In-flow page head: calm title row, scrolls with content (spec §4). */
export const Header = ({ className, fixed: _fixed, children, ...props }: HeaderProps) => {
  // Don't render if there's no children
  if (!children) {
    return null;
  }

  return (
    <header className={cn('flex flex-wrap items-center justify-between gap-x-3 gap-y-2 px-4 pt-4', className)} {...props}>
      {children}
    </header>
  );
};

Header.displayName = 'Header';
