export const getStatusColor = (status: string) => {
  switch (status) {
    case 'completed':
      return 'border-success/40 bg-success/10 text-(--success-soft-fg)';
    case 'failed':
      return 'border-destructive/40 bg-destructive/10 text-(--destructive-soft-fg)';
    case 'pending':
      return 'border-warning/40 bg-warning/10 text-(--warning-soft-fg)';
    case 'processing':
      return 'border-info/40 bg-info/10 text-(--info-soft-fg)';
    default:
      return 'border-border bg-muted text-muted-foreground';
  }
};
