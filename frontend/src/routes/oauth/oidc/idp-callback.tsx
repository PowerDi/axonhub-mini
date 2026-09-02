import { useEffect, useRef } from 'react';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useOIDCExchange } from '@/features/auth/data/auth';
import { Loader2 } from 'lucide-react';
import { toast } from 'sonner';

export const Route = createFileRoute('/oauth/oidc/idp-callback')({
  component: OIDCCallback,
  validateSearch: (search: Record<string, unknown>) => {
    return {
      code: (search.code as string) || '',
      error: search.error as string | undefined,
      error_description: search.error_description as string | undefined,
    };
  },
});

function OIDCCallback() {
  const { code, error, error_description } = Route.useSearch();
  const navigate = useNavigate();
  const exchangeMutation = useOIDCExchange();
  const hasAttemptedRef = useRef(false);

  useEffect(() => {
    // Prevent strict mode double-firing
    if (hasAttemptedRef.current) return;
    hasAttemptedRef.current = true;

    if (error) {
      console.error('OIDC Error:', error, error_description);
      toast.error(error_description || error || 'Authentication failed');
      navigate({ to: '/sign-in' });
      return;
    }

    if (!code) {
      toast.error('Missing authorization code');
      navigate({ to: '/sign-in' });
      return;
    }

    exchangeMutation.mutate(code);
  }, [code, error, error_description, navigate, exchangeMutation]);

  return (
    <div className="bg-muted flex h-screen w-full flex-col items-center justify-center">
      <div className="bg-card text-card-foreground ring-foreground/10 flex flex-col items-center space-y-4 rounded-xl p-8 text-center ring-1">
        <div className="bg-muted flex h-16 w-16 items-center justify-center rounded-full">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
        <div className="space-y-2">
          <h2 className="text-xl font-semibold text-muted-foreground">Completing Sign In</h2>
          <p className="text-sm text-muted-foreground">Please wait while we verify your credentials...</p>
        </div>
      </div>
    </div>
  );
}
