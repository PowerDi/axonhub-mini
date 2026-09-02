'use client';

import type { ReactNode } from 'react';
import { Card, CardContent, CardDescription, CardHeader } from '@/components/ui/card';

type SettingsSectionCardProps = {
  /** Card title; rendered as a calm h2 (text-base font-medium). */
  title?: string;
  description?: string;
  /** Section content (form fields, tables, …). */
  children: ReactNode;
  /**
   * Right-aligned header actions (test / save buttons). When present the
   * header always renders — the buttons need a host.
   */
  actions?: ReactNode;
  /**
   * Drop the header entirely (spec §3.3 SettingsSectionCard): single-card tabs
   * whose title restates the tab label skip the duplicated title/description
   * stack. Multi-card tabs keep headers — they are the only section labels.
   */
  headerless?: boolean;
  /** Extra classes for the wrapping Card. */
  className?: string;
}

/**
 * Settings section shell (metapi-go settings-section-card adapted to tabs):
 * header renders only for cards with actions or with non-duplicated titles.
 */
export function SettingsSectionCard({ title, description, children, actions, headerless = false, className }: SettingsSectionCardProps) {
  return (
    <Card className={className}>
      {actions || (!headerless && title) ? (
        <CardHeader className={actions ? 'flex flex-row items-start justify-between gap-4' : undefined}>
          <div className='space-y-1'>
            {/* h2: the page-level h1 lives in the page header; card titles are L2 */}
            <h2 className='text-base leading-snug font-medium'>{title}</h2>
            {description ? <CardDescription>{description}</CardDescription> : null}
          </div>
          {actions ? <div className='flex shrink-0 gap-2'>{actions}</div> : null}
        </CardHeader>
      ) : null}
      <CardContent>{children}</CardContent>
    </Card>
  );
}

/** Loading placeholder while the runtime-settings query is fetching. */
export function SettingsSectionSkeleton() {
  return (
    <Card>
      <CardContent className='space-y-4 pt-4'>
        <div className='space-y-1'>
          <div className='bg-muted h-5 w-40 animate-pulse rounded-md' />
          <div className='bg-muted h-4 w-64 animate-pulse rounded-md' />
        </div>
        <div className='bg-muted h-8 w-full animate-pulse rounded-lg' />
        <div className='bg-muted h-8 w-full animate-pulse rounded-lg' />
        <div className='bg-muted h-8 w-1/2 animate-pulse rounded-lg' />
      </CardContent>
    </Card>
  );
}
