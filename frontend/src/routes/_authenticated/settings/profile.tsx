import { createFileRoute } from '@tanstack/react-router';
import SettingsProfile from '@/features/settings/profile';

export const Route = createFileRoute('/_authenticated/settings/profile')({
  component: SettingsProfile,
  validateSearch: (search: Record<string, unknown>): { oidc_link?: string } => {
    return {
      oidc_link: typeof search.oidc_link === 'string' ? search.oidc_link : undefined,
    };
  },
});
