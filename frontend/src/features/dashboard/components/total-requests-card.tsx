import { Database } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { IconBadge } from '@/components/ui/icon-badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useDashboardStats } from '../data/dashboard';

export function TotalRequestsCard() {
  const { t } = useTranslation();
  const { data: stats, isLoading, error } = useDashboardStats();

  const calculateGrowth = (current: number, previous: number): { percentage: number; isPositive: boolean } => {
    if (previous === 0) {
      return { percentage: current > 0 ? 100 : 0, isPositive: current > 0 };
    }
    const percentage = ((current - previous) / previous) * 100;
    return { percentage, isPositive: percentage >= 0 };
  };

  const growth = stats?.requestStats
    ? calculateGrowth(stats.requestStats.requestsThisWeek, stats.requestStats.requestsLastWeek)
    : { percentage: 0, isPositive: true };

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
            <IconBadge tone='primary' size='sm'>
              <Database />
            </IconBadge>
            <CardTitle className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
              {t('dashboard.stats.allTimeRequests')}
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
          <IconBadge tone='primary' size='sm'>
            <Database />
          </IconBadge>
          <CardTitle className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
            {t('dashboard.stats.allTimeRequests')}
          </CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className='flex items-end justify-between gap-2'>
          <div className='text-2xl font-semibold tracking-tight tabular-nums'>{formatNumber(stats?.totalRequests || 0)}</div>
          <span
            className={`text-xs font-medium tabular-nums ${growth.isPositive ? 'text-(--success-soft-fg)' : 'text-(--destructive-soft-fg)'}`}
          >
            {growth.isPositive ? '+' : ''}
            {growth.percentage.toFixed(0)}% {t('dashboard.stats.vsLastWeek')}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
