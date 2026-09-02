import { ShieldCheck } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { IconBadge } from '@/components/ui/icon-badge';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { useDashboardStats } from '../data/dashboard';

export function SuccessRateCard() {
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
            <Skeleton className='mt-2 h-2 w-full' />
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
            <IconBadge tone='success' size='sm'>
              <ShieldCheck />
            </IconBadge>
            <CardTitle className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
              {t('dashboard.cards.successRate')}
            </CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-(--destructive-soft-fg)'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  const successRate =
    stats && stats.totalRequests > 0 ? (((stats.totalRequests - stats.failedRequests) / stats.totalRequests) * 100).toFixed(1) : '0.0';

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <IconBadge tone='success' size='sm'>
            <ShieldCheck />
          </IconBadge>
          <CardTitle className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
            {t('dashboard.cards.successRate')}
          </CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <div className='flex flex-col gap-3'>
          <div className='flex items-end justify-between'>
            <div className='text-2xl font-semibold tracking-tight tabular-nums'>
              {successRate}
              <span className='text-muted-foreground text-lg font-normal'>%</span>
            </div>
          </div>
          <Progress value={parseFloat(successRate)} className='h-2' />
          <div className='flex justify-between text-xs'>
            <span className='text-muted-foreground tabular-nums'>
              {formatNumber(stats?.failedRequests || 0)} {t('dashboard.stats.failedRequests')}
            </span>
            <span className='text-muted-foreground'>{t('dashboard.stats.average')}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
