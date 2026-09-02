'use client';

import { cva, type VariantProps } from 'class-variance-authority';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

// Soft solid-tone icon chip (spec §4 KPI 卡 IconBadge): solid soft fills only,
// tones map 1:1 onto the semantic soft-fg tokens so every fill stays theme-aware.
const iconBadgeVariants = cva('flex shrink-0 items-center justify-center [&>svg]:shrink-0', {
  variants: {
    tone: {
      default: 'bg-muted text-muted-foreground',
      primary: 'bg-primary/10 text-primary',
      success: 'bg-success/10 text-(--success-soft-fg)',
      warning: 'bg-warning/10 text-(--warning-soft-fg)',
      info: 'bg-info/10 text-(--info-soft-fg)',
      destructive: 'bg-destructive/10 text-(--destructive-soft-fg)',
    },
    size: {
      sm: 'size-7 rounded-md [&>svg]:size-3.5',
      md: 'size-8 rounded-lg [&>svg]:size-4',
      lg: 'size-10 rounded-xl [&>svg]:size-5',
    },
  },
  defaultVariants: {
    tone: 'default',
    size: 'md',
  },
});

type IconBadgeProps = VariantProps<typeof iconBadgeVariants> & {
  children?: ReactNode;
  className?: string;
};

export function IconBadge({ tone, size, className, children }: IconBadgeProps) {
  // Icons are decorative; the surrounding label carries meaning.
  return (
    <span aria-hidden='true' className={cn(iconBadgeVariants({ tone, size }), className)}>
      {children}
    </span>
  );
}
