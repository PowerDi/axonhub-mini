import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg border border-transparent text-sm font-medium whitespace-nowrap transition-[color,background-color,border-color,box-shadow,transform] outline-none select-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-focus-ring active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        // primary has no hover color shift on <button>; press feedback is active:translate-y-px (spec §3.1)
        default: 'bg-primary text-primary-foreground [a]:hover:bg-primary/80',
        destructive:
          'bg-destructive/10 text-(--destructive-soft-fg) hover:bg-destructive/20 focus-visible:ring-destructive/20 dark:bg-destructive/20 dark:hover:bg-destructive/30',
        outline:
          'border-border bg-background hover:bg-muted hover:text-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50',
        secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
        ghost: 'hover:bg-muted hover:text-foreground',
        link: 'text-primary underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-8 px-2.5',
        xs: "h-6 gap-1 rounded-md px-2 text-xs [&_svg:not([class*=\"size-\"])]:size-3",
        sm: "h-7 rounded-md px-2.5 text-[0.8rem] [&_svg:not([class*=\"size-\"])]:size-3.5",
        lg: 'h-9 px-2.5',
        icon: 'size-8',
        'icon-xs': 'size-6',
        'icon-sm': 'size-7',
        'icon-lg': 'size-9',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
);

function isIconSize(size: VariantProps<typeof buttonVariants>['size']) {
  return typeof size === 'string' && size.startsWith('icon');
}

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  }) {
  const Comp = asChild ? Slot : 'button';

  return (
    <Comp
      data-slot='button'
      // Icon-only buttons are the densest controls in the app (row actions,
      // dialog close). Mark them so the coarse-pointer hit-area rule in
      // index.css expands their tap target to ≥40px without changing the
      // visual size (fine pointers keep the current density).
      data-hit-area={isIconSize(size) ? true : undefined}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
