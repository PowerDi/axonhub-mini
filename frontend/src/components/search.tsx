import { IconSearch } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { useSearch } from '@/context/search-context';
import { Button } from './ui/button';

interface Props {
  className?: string;
  type?: React.HTMLInputTypeAttribute;
  placeholder?: string;
}

/** Search trigger per spec §3.7: outline h-8 with an inline ⌘K kbd hint. */
export function Search({ className = '', placeholder }: Props) {
  const { setOpen } = useSearch();
  const { t } = useTranslation();
  const defaultPlaceholder = placeholder || t('search.placeholder');
  return (
    <Button
      variant='outline'
      className={cn('text-muted-foreground h-8 w-[220px] justify-start px-2.5 font-normal lg:w-[240px]', className)}
      onClick={() => setOpen(true)}
    >
      <IconSearch aria-hidden='true' className='size-4' />
      <span className='truncate'>{defaultPlaceholder}</span>
      <kbd className='bg-muted text-muted-foreground pointer-events-none ml-auto flex h-5 min-w-5 items-center justify-center gap-0.5 rounded-sm border px-1 font-mono text-[11px] font-medium select-none'>
        <span>⌘</span>K
      </kbd>
    </Button>
  );
}
