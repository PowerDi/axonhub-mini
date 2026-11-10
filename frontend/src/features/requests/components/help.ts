export const getStatusColor = (status: string) => {
  switch (status) {
    case 'completed':
      return 'border-green-500/40 bg-green-500/10 text-green-600';
    case 'failed':
      return 'border-destructive/40 bg-destructive/10 text-destructive';
    case 'pending':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-600';
    case 'processing':
      return 'border-blue-500/40 bg-blue-500/10 text-blue-600';
    default:
      return 'border-border bg-muted text-muted-foreground';
  }
};
