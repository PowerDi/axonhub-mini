import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import type { ChannelCircuitBreakerStats } from '../data/schema';

interface ChannelCircuitBreakerCellProps {
  stats: ChannelCircuitBreakerStats[];
}

function formatTime(value?: string | null) {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleTimeString();
}

export const ChannelCircuitBreakerCell = memo(({ stats }: ChannelCircuitBreakerCellProps) => {
  const { t } = useTranslation();

  if (stats.length === 0) return null;

  return (
    <div className='flex flex-wrap justify-center gap-1'>
      {stats.map((item) => {
        const isOpen = item.state === 'open';
        const isHalfOpen = item.state === 'half_open';
        const nextProbeAt = formatTime(item.nextProbeAt);

        return (
          <Tooltip key={item.modelID}>
            <TooltipTrigger asChild>
              <Badge
                variant='outline'
                className={cn(
                  'cursor-help max-w-36 truncate text-[10px]',
                  isOpen && 'border-(--destructive-soft-fg)/40 bg-(--destructive-soft-fg)/10 text-(--destructive-soft-fg)',
                  isHalfOpen && 'border-(--warning-soft-fg)/40 bg-(--warning-soft-fg)/10 text-(--warning-soft-fg)'
                )}
              >
                {item.modelID}
              </Badge>
            </TooltipTrigger>
            <TooltipContent>
              <div className='space-y-1 text-xs'>
                <div>{t(`channels.columns.circuitBreaker.state.${item.state}`)}</div>
                <div>
                  {t('channels.columns.circuitBreaker.failures')}: {item.consecutiveFailures}
                </div>
                {nextProbeAt && (
                  <div>
                    {t('channels.columns.circuitBreaker.nextProbeAt')}: {nextProbeAt}
                  </div>
                )}
              </div>
            </TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
});

ChannelCircuitBreakerCell.displayName = 'ChannelCircuitBreakerCell';
