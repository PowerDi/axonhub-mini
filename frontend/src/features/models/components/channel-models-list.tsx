import { Badge } from '@/components/ui/badge';
import { useTranslation } from 'react-i18next';

interface ChannelModel {
  requestModel: string;
}

interface Channel {
  id: string | number;
  name: string;
  type: string;
  status: string;
}

interface ChannelModelsListProps {
  channels: Array<{
    channel: Channel;
    models: ChannelModel[];
  }>;
  emptyMessage?: string;
}

export function ChannelModelsList({ channels, emptyMessage }: ChannelModelsListProps) {
  const { t } = useTranslation();

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'enabled':
        return 'bg-success/10 text-(--success-soft-fg) border-success/40 dark:text-(--success-soft-fg) dark:border-success/40';
      case 'disabled':
        return 'bg-muted text-muted-foreground border-border dark:bg-muted dark:text-muted-foreground dark:border-border';
      case 'archived':
        return 'bg-warning/10 text-(--warning-soft-fg) border-warning/40 dark:text-(--warning-soft-fg) dark:border-warning/40';
      default:
        return 'bg-muted text-muted-foreground border-border dark:bg-muted dark:text-muted-foreground dark:border-border';
    }
  };

  const getTypeColor = (type: string) => {
    const colors = {
      openai: 'bg-info/10 text-(--info-soft-fg) border-info/40 dark:text-(--info-soft-fg)',
      anthropic: 'bg-primary/10 text-primary border-primary/40 dark:text-primary',
      deepseek: 'bg-info/10 text-(--info-soft-fg) border-info/40 dark:text-(--info-soft-fg)',
      doubao: 'bg-warning/10 text-(--warning-soft-fg) border-warning/40 dark:text-(--warning-soft-fg)',
      kimi: 'bg-primary/10 text-primary border-primary/40 dark:text-primary',
    };
    return colors[type as keyof typeof colors] || 'bg-muted text-muted-foreground border-border dark:bg-muted dark:text-muted-foreground';
  };

  if (channels.length === 0) {
    return (
      <p className='text-muted-foreground py-8 text-center text-sm'>
        {emptyMessage || t('models.dialogs.association.noConnections')}
      </p>
    );
  }

  return (
    <div className='space-y-3'>
      {channels.map((conn) => (
        <div key={conn.channel.id} className='rounded-lg border p-3'>
          <div className='mb-2 flex items-start justify-between gap-2'>
            <div className='flex items-center gap-1.5 flex-wrap'>
              <span className='text-sm font-medium'>{conn.channel.name}</span>
              <Badge variant='outline' className={`h-5 px-1.5 text-[10px] font-normal ${getTypeColor(conn.channel.type)}`}>
                {t(`channels.types.${conn.channel.type}`, conn.channel.type)}
              </Badge>
              <Badge variant='outline' className={`h-5 px-1.5 text-[10px] font-normal ${getStatusColor(conn.channel.status)}`}>
                {t(`channels.status.${conn.channel.status}`)}
              </Badge>
            </div>
          </div>
          <div className='space-y-1'>
            {conn.models.map((model) => (
              <div key={model.requestModel} className='bg-muted rounded px-2 py-1 text-xs'>
                {model.requestModel}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}