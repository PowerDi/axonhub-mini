import Cookies from 'js-cookie';
import { Outlet } from '@tanstack/react-router';
import { cn } from '@/lib/utils';
import { useVersionCheck } from '@/hooks/use-version-check';
import { SidebarProvider } from '@/components/ui/sidebar';
import { AppHeader } from '@/components/layout/app-header';
import { AppSidebar } from '@/components/layout/app-sidebar';
import SkipToMain from '@/components/skip-to-main';
import { OnboardingProvider } from '@/features/onboarding';
import { useSidebarData } from './sidebar';

interface Props {
  children?: React.ReactNode;
}

export function AuthenticatedLayout({ children }: Props) {
  const defaultOpen = Cookies.get('sidebar_state') !== 'false';
  const sidebarData = useSidebarData();

  // Check for new version on mount (only for owners)
  useVersionCheck();

  return (
    <SidebarProvider defaultOpen={defaultOpen} className='fixed inset-0 min-h-0 flex-col overflow-hidden'>
      <AppHeader />
      <div className='flex flex-1 overflow-hidden'>
        <SkipToMain />
        <AppSidebar sidebarData={sidebarData} />
        <div
          id='content'
          className={cn(
            'ml-auto w-full max-w-full',
            'peer-data-[state=collapsed]:w-[calc(100%-var(--sidebar-width-icon)-1rem)]',
            'peer-data-[state=expanded]:w-[calc(100%-var(--sidebar-width))]',
            'sm:transition-[width] sm:duration-200 sm:ease-linear',
            'flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden pt-14',
            'group-data-[scroll-locked=1]/body:h-full',
            'has-[main.fixed-main]:group-data-[scroll-locked=1]/body:h-svh'
          )}
        >
          {/* Inset content panel (spec §3.7): hairline ring instead of shadow at rest.
              Left margin is kept (no ml-0): flush against the sidebar, the ring line
              sits under the sidebar rail strip and its hover bar and reads as covered. */}
          <div className='ring-foreground/10 m-2 flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl bg-background ring-1 dark:ring-border'>
            <div className='flex min-h-0 flex-1 flex-col overflow-auto has-[main.fixed-main]:overflow-hidden'>
              <OnboardingProvider>{children ? children : <Outlet />}</OnboardingProvider>
            </div>
          </div>
        </div>
      </div>
    </SidebarProvider>
  );
}
