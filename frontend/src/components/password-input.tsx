import * as React from 'react';
import { IconEye, IconEyeOff } from '@tabler/icons-react';
import { cn } from '@/lib/utils';
import { Button } from './ui/button';

type PasswordInputProps = Omit<React.InputHTMLAttributes<HTMLInputElement>, 'type'>;

const PasswordInput = React.forwardRef<HTMLInputElement, PasswordInputProps>(({ className, disabled, ...props }, ref) => {
  const [showPassword, setShowPassword] = React.useState(false);
  return (
    <div className={cn('relative rounded-lg', className)}>
      <input
        type={showPassword ? 'text' : 'password'}
        className='border-input placeholder:text-muted-foreground dark:bg-input/30 focus-visible:border-ring focus-visible:ring-(--focus-ring) flex h-8 w-full rounded-lg border bg-transparent px-2.5 py-1 text-sm transition-[color,box-shadow] outline-none focus-visible:ring-3 focus-visible:ring-inset disabled:cursor-not-allowed disabled:opacity-50'
        ref={ref}
        disabled={disabled}
        {...props}
      />
      <Button
        type='button'
        size='icon-xs'
        variant='ghost'
        disabled={disabled}
        className='text-muted-foreground absolute top-1/2 right-1 -translate-y-1/2'
        onClick={() => setShowPassword((prev) => !prev)}
      >
        {showPassword ? <IconEye size={16} /> : <IconEyeOff size={16} />}
      </Button>
    </div>
  );
});
PasswordInput.displayName = 'PasswordInput';

export { PasswordInput };
