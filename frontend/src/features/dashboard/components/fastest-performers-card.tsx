'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { UseQueryResult } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { TimePeriodSelector, type FastestTimeWindow } from '@/components/time-period-selector';
import { safeNumber, safeToFixed, sanitizeChartData, type ChartData } from '../utils/chart-helpers';

// 5 colors matches the slice limit in chartData processing (.slice(0, 5))
const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];

interface PerformerListProps {
  data: ChartData[];
  height?: number;
  noDataLabel: string;
}

function PerformerList({ data, height = 280, noDataLabel }: PerformerListProps) {
  const safeData = sanitizeChartData(data);

  if (safeData.length === 0) {
    return (
      <div className='flex h-[250px] items-center justify-center text-muted-foreground text-sm'>
        {noDataLabel}
      </div>
    );
  }

  const maxThroughput = safeData.reduce((max, item) => Math.max(max, safeNumber(item.throughput)), 0) || 1;

  return (
    <div className='space-y-1.5' style={{ minHeight: height }}>
      {safeData.map((item, index) => {
        const throughput = safeNumber(item.throughput);
        const percent = Math.max(2, (throughput / maxThroughput) * 100);
        return (
          <div key={`${item.name}-${index}`} className='rounded-lg border px-3 py-2 transition-colors hover:bg-muted/50'>
            <div className='flex items-center justify-between gap-3'>
              <div className='flex min-w-0 items-center gap-2'>
                <span className='bg-muted flex size-5 shrink-0 items-center justify-center rounded-md text-[10px] font-semibold tabular-nums'>
                  {index + 1}
                </span>
                <span className='truncate text-sm font-medium'>{item.name}</span>
              </div>
              <div className='shrink-0 text-right leading-tight whitespace-nowrap'>
                <span className='text-xs font-semibold tabular-nums'>{safeToFixed(throughput, 0)} tok/s</span>
                <span className='text-muted-foreground ml-2 text-xs tabular-nums'>
                  {formatNumber(safeNumber(item.requestCount))} req
                </span>
              </div>
            </div>
            <div className='bg-muted mt-2 h-1.5 w-full overflow-hidden rounded-full'>
              <div
                className='h-full rounded-full'
                style={{ width: `${percent}%`, backgroundColor: COLORS[index % COLORS.length] }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

interface ThroughputData {
  throughput?: number;
  requestCount?: number;
}

interface FastestPerformersCardProps<T extends ThroughputData> {
  title: string;
  description: (totalRequests: number) => string;
  noDataLabel: string;
  useData: (timeWindow: string) => UseQueryResult<T[], Error>;
  getName: (item: T) => string | null;
}

export function FastestPerformersCard<T extends ThroughputData>({
  title,
  description,
  noDataLabel,
  useData,
  getName,
}: FastestPerformersCardProps<T>) {
  const { t } = useTranslation();
  const [timeWindow, setTimeWindow] = useState<FastestTimeWindow>('month');

  const { data: items, isLoading, isFetching, error } = useData(timeWindow);

  if (isLoading && !items) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className='h-5 w-[180px]' />
          <Skeleton className='h-4 w-[120px]' />
        </CardHeader>
        <CardContent>
          <div className='space-y-1.5'>
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className='h-[54px] w-full rounded-lg' />
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-(--destructive-soft-fg)'>
            {t('common.loadError')}: {error.message}
          </div>
        </CardContent>
      </Card>
    );
  }

  const chartData: ChartData[] = (items || [])
    .slice(0, 5)
    .filter((item) => item != null)
    .map((item) => ({
      name: getName(item) ?? 'Unknown',
      throughput: safeNumber(item.throughput ?? 0),
      requestCount: safeNumber(item.requestCount ?? 0),
    }))
    .sort((a, b) => b.throughput - a.throughput);

  const totalRequests = chartData.reduce((sum, item) => sum + item.requestCount, 0);

  return (
    <Card className='h-full'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='space-y-1'>
          <CardTitle className='text-base font-medium'>{title}</CardTitle>
          <CardDescription>{description(totalRequests)}</CardDescription>
        </div>
        <TimePeriodSelector value={timeWindow} onChange={setTimeWindow} periods={['month', 'week', 'day']} />
      </CardHeader>
      <CardContent className='relative'>
        <PerformerList data={chartData} noDataLabel={noDataLabel} />
        {isFetching && (
          <div className='absolute inset-0 flex items-center justify-center bg-background/50'>
            <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
          </div>
        )}
      </CardContent>
    </Card>
  );
}
