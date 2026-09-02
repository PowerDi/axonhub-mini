import { Activity } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { IconBadge } from '@/components/ui/icon-badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useDashboardStats } from '../data/dashboard';

export function TodayRequestsCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading, error } = useDashboardStats();

  if (isLoading) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[120px]' />
          <Skeleton className='h-4 w-4' />
        </CardHeader>
        <CardContent>
          <div className='space-y-2'>
            <Skeleton className='h-8 w-[80px]' />
            <Skeleton className='mt-1 h-4 w-[140px]' />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <div className='flex items-center gap-2'>
            <IconBadge tone='info' size='sm'>
              <Activity />
            </IconBadge>
            <CardTitle className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
              {t('dashboard.stats.todayRequests')}
            </CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-(--destructive-soft-fg)'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <IconBadge tone='info' size='sm'>
            <Activity />
          </IconBadge>
          <CardTitle className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
            {t('dashboard.stats.todayRequests')}
          </CardTitle>
        </div>
        <div className='bg-primary size-2 animate-ping rounded-full' />
      </CardHeader>
      <CardContent className='flex flex-col gap-3'>
        <div className='text-2xl font-semibold tracking-tight tabular-nums'>
          {formatNumber(stats?.requestStats?.requestsToday || 0)}
        </div>
        <div className='grid grid-cols-2 gap-2'>
          <div className='bg-muted/40 rounded-lg border px-2.5 py-2'>
            <div className='text-muted-foreground truncate text-[11px] leading-none font-medium'>
              {t('dashboard.stats.thisWeek')}
            </div>
            <div className='mt-1.5 truncate text-xs font-semibold tabular-nums'>
              {formatNumber(stats?.requestStats?.requestsThisWeek || 0)}
            </div>
          </div>
          <div className='bg-muted/40 rounded-lg border px-2.5 py-2'>
            <div className='text-muted-foreground truncate text-[11px] leading-none font-medium'>
              {t('dashboard.stats.thisMonth')}
            </div>
            <div className='mt-1.5 truncate text-xs font-semibold tabular-nums'>
              {formatNumber(stats?.requestStats?.requestsThisMonth || 0)}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
