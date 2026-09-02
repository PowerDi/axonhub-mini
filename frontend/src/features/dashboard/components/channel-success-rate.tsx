import { ActivityIcon, CheckCircle2Icon, XCircleIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { IconBadge } from '@/components/ui/icon-badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useChannelSuccessRates } from '../data/dashboard';

export function ChannelSuccessRate() {
  const { t } = useTranslation();
  const { data: channels, isLoading, error } = useChannelSuccessRates();

  if (isLoading) {
    return (
      <div className='@container'>
        <div
          tabIndex={0}
          className='grid max-h-[322px] grid-cols-1 gap-x-6 gap-y-6 overflow-y-auto [scrollbar-gutter:stable] @md:grid-cols-2 @2xl:grid-cols-3'
        >
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className='flex items-center'>
              <Skeleton className='h-9 w-9 rounded-md' />
              <div className='ml-4 space-y-1'>
                <Skeleton className='h-4 w-[120px]' />
                <Skeleton className='h-3 w-[160px]' />
              </div>
              <Skeleton className='ml-auto h-4 w-[60px]' />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className='text-sm text-(--destructive-soft-fg)'>
        {t('dashboard.charts.errorLoadingChannelSuccessRate')} {error.message}
      </div>
    );
  }

  if (!channels || channels.length === 0) {
    return <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noChannelData')}</div>;
  }

  return (
    <div className='@container'>
      <div
        tabIndex={0}
        className='grid max-h-[322px] grid-cols-1 gap-x-6 gap-y-6 overflow-y-auto [scrollbar-gutter:stable] @md:grid-cols-2 @2xl:grid-cols-3'
      >
        {channels.map((channel) => (
          <div key={channel.channelId} className='flex items-center'>
            <IconBadge tone='primary' size='md' className='mr-4'>
              <ActivityIcon />
            </IconBadge>
            <div className='min-w-0 space-y-1'>
              <p className='truncate text-sm leading-none font-medium'>{channel.channelName || '-'}</p>
              <div className='text-muted-foreground flex gap-3 text-sm tabular-nums'>
                <span className='flex items-center gap-1'>
                  <CheckCircle2Icon className='text-(--success-soft-fg) h-3 w-3' />
                  {formatNumber(channel.successCount)}
                </span>
                <span className='flex items-center gap-1'>
                  <XCircleIcon className='text-(--destructive-soft-fg) h-3 w-3' />
                  {formatNumber(channel.failedCount)}
                </span>
              </div>
            </div>
            <div className='ml-auto pl-2 font-medium tabular-nums'>{channel.successRate.toFixed(1)}%</div>
          </div>
        ))}
      </div>
    </div>
  );
}
