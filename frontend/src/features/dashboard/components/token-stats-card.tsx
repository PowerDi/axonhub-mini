import { useState } from 'react';
import { BarChart4 } from 'lucide-react';
import { IconInfoCircle } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { IconBadge } from '@/components/ui/icon-badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { useTokenStats } from '../data/dashboard';

type TimeRange = 'allTime' | 'thisMonth' | 'thisWeek' | 'thisDay';

function formatLastUpdated(timestamp: string | null, locale: string): string {
  if (!timestamp) return '';
  const date = new Date(timestamp);
  return date.toLocaleString(locale, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  });
}

interface LastUpdatedInfoProps {
  lastUpdated: string | null;
  locale: string;
  t: (key: string, options?: Record<string, string>) => string;
}

function LastUpdatedInfo({ lastUpdated, locale, t }: LastUpdatedInfoProps) {
  if (!lastUpdated) return null;

  const formattedTime = formatLastUpdated(lastUpdated, locale);
  const label = t('dashboard.stats.updated', { time: formattedTime });

  return (
    <>
      <div className='hidden h-5 w-5 sm:block'>
        <TooltipProvider delayDuration={0}>
          <Tooltip>
            <TooltipTrigger asChild>
              <button type='button' className='text-muted-foreground hover:text-foreground flex h-5 w-5 items-center justify-center rounded-full transition-colors'>
                <IconInfoCircle className='h-3.5 w-3.5' />
              </button>
            </TooltipTrigger>
            <TooltipContent>
              <span>{label}</span>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
      <div className='-my-2.5 h-11 w-11 sm:hidden'>
        <Popover>
          <PopoverTrigger asChild>
            <button type='button' className='text-muted-foreground hover:text-foreground flex h-11 w-11 items-center justify-center rounded-full transition-colors'>
              <IconInfoCircle className='h-4 w-4' />
            </button>
          </PopoverTrigger>
          <PopoverContent className='w-fit'>
            <span className='text-sm'>{label}</span>
          </PopoverContent>
        </Popover>
      </div>
    </>
  );
}

export function TokenStatsCard() {
  const { t, i18n } = useTranslation();
  const { data: stats, isLoading, error } = useTokenStats();
  const [timeRange, setTimeRange] = useState<TimeRange>('thisDay');

  if (isLoading) {
    return (
      <Card className='min-w-0'>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[120px]' />
          <Skeleton className='h-6 w-[180px]' />
        </CardHeader>
        <CardContent>
          <div className='grid grid-cols-3 gap-2'>
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className='space-y-2'>
                <Skeleton className='h-3 w-[40px]' />
                <Skeleton className='h-4 w-[60px]' />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className='min-w-0'>
        <CardHeader className='flex flex-col sm:flex-row sm:items-center sm:justify-between space-y-2 sm:space-y-0 pb-2'>
          <div className='flex items-center gap-2 min-w-0'>
            <IconBadge tone='primary' size='sm'>
              <BarChart4 />
            </IconBadge>
            <CardTitle className='text-muted-foreground truncate text-xs font-medium tracking-wide uppercase'>
              {t('dashboard.cards.tokenStats')}
            </CardTitle>
          </div>
          <div className='flex items-center gap-1 shrink-0'>
            <span className='text-muted-foreground rounded-md px-2 py-1 text-xs'>{t('dashboard.stats.month')}</span>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-(--destructive-soft-fg)'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  const getTokens = (range: TimeRange) => {
    if (range === 'allTime') {
      return {
        input: stats?.totalInputTokensAllTime || 0,
        output: stats?.totalOutputTokensAllTime || 0,
        cached: stats?.totalCachedTokensAllTime || 0,
      };
    }
    if (range === 'thisDay') {
      return {
        input: stats?.totalInputTokensToday || 0,
        output: stats?.totalOutputTokensToday || 0,
        cached: stats?.totalCachedTokensToday || 0,
      };
    }
    if (range === 'thisMonth') {
      return {
        input: stats?.totalInputTokensThisMonth || 0,
        output: stats?.totalOutputTokensThisMonth || 0,
        cached: stats?.totalCachedTokensThisMonth || 0,
      };
    }
    return {
      input: stats?.totalInputTokensThisWeek || 0,
      output: stats?.totalOutputTokensThisWeek || 0,
      cached: stats?.totalCachedTokensThisWeek || 0,
    };
  };

  const tokens = getTokens(timeRange);

  return (
    <Card className='min-w-0'>
      <CardHeader className='flex flex-row items-start justify-between gap-2 pb-2 sm:items-center'>
        <div className='flex min-w-0 items-center gap-2'>
          <IconBadge tone='primary' size='sm'>
            <BarChart4 />
          </IconBadge>
          <CardTitle className='text-muted-foreground truncate text-xs font-medium tracking-wide uppercase'>
            {t('dashboard.cards.tokenStats')}
          </CardTitle>
        </div>
        <div className='flex shrink-0 items-center gap-1 whitespace-nowrap'>
          {/* <span className='text-xs text-muted-foreground'>{t('dashboard.stats.this')}</span> */}
          {timeRange === 'allTime' && (
            <LastUpdatedInfo
              lastUpdated={stats?.lastUpdated ?? null}
              locale={i18n.language}
              t={t}
            />
          )}
          <Tabs value={timeRange} onValueChange={(v) => setTimeRange(v as TimeRange)}>
            <TabsList className='h-7 p-0.5'>
              <TabsTrigger value='allTime' className='h-6 px-2 text-[11px]'>
                {t('dashboard.stats.all')}
              </TabsTrigger>
              <TabsTrigger value='thisMonth' className='h-6 px-2 text-[11px]'>
                {t('dashboard.stats.month')}
              </TabsTrigger>
              <TabsTrigger value='thisWeek' className='h-6 px-2 text-[11px]'>
                {t('dashboard.stats.week')}
              </TabsTrigger>
              <TabsTrigger value='thisDay' className='h-6 px-2 text-[11px]'>
                {t('dashboard.stats.day')}
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </CardHeader>
      <CardContent>
        <div className='grid grid-cols-3 gap-2'>
          <div className='bg-muted/40 rounded-lg border px-2.5 py-2'>
            <div className='text-muted-foreground truncate text-[11px] leading-none font-medium'>{t('dashboard.stats.input')}</div>
            <div className='mt-1.5 truncate text-xs font-semibold tabular-nums'>{formatNumber(tokens.input)}</div>
          </div>
          <div className='bg-muted/40 rounded-lg border px-2.5 py-2'>
            <div className='text-muted-foreground truncate text-[11px] leading-none font-medium'>{t('dashboard.stats.output')}</div>
            <div className='mt-1.5 truncate text-xs font-semibold tabular-nums'>{formatNumber(tokens.output)}</div>
          </div>
          <div className='bg-muted/40 rounded-lg border px-2.5 py-2'>
            <div className='text-muted-foreground truncate text-[11px] leading-none font-medium'>{t('dashboard.stats.cached')}</div>
            <div className='text-muted-foreground mt-1.5 truncate text-xs font-semibold tabular-nums'>
              {formatNumber(tokens.cached)}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
