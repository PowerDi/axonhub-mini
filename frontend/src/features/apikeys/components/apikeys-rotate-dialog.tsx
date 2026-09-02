import { useState } from 'react';
import { Copy, RefreshCw, AlertTriangle, CheckIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { copyTextToClipboard } from '@/lib/clipboard';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useApiKeysContext } from '../context/apikeys-context';
import { useRotateApiKey } from '../data/apikeys';

export function ApiKeysRotateDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedApiKey, setSelectedApiKey } = useApiKeysContext();
  const rotateApiKey = useRotateApiKey();
  const [newKey, setNewKey] = useState<string | null>(null);
  const [isCopied, setIsCopied] = useState(false);

  const handleRotate = async () => {
    if (!selectedApiKey) return;

    try {
      const result = await rotateApiKey.mutateAsync(selectedApiKey.id);
      setNewKey(result.key);
      // Update the selected API key with the new key
      setSelectedApiKey({ ...selectedApiKey, key: result.key });
    } catch {
      // Error is handled by the mutation
    }
  };

  const handleCopy = async () => {
    if (newKey) {
      try {
        await copyTextToClipboard(newKey);
        setIsCopied(true);
        toast.success(t('apikeys.messages.copied'));
        setTimeout(() => setIsCopied(false), 2000);
      } catch {
        toast.error(t('common.errors.copyFailed'));
      }
    }
  };

  const handleClose = () => {
    setNewKey(null);
    closeDialog();
  };

  const maskedNewKey = newKey ? newKey.slice(0, 3) + '...' + newKey.slice(-4) : '';

  return (
    <Dialog open={isDialogOpen.rotate} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('apikeys.dialogs.rotate.title')}</DialogTitle>
          <DialogDescription>
            {t('apikeys.dialogs.rotate.description', { name: selectedApiKey?.name })}
          </DialogDescription>
        </DialogHeader>

        <Alert className="border-warning/40 bg-warning/10 dark:border-warning/40">
          <AlertTriangle className="h-4 w-4 text-(--warning-soft-fg) dark:text-(--warning-soft-fg)" />
          <AlertDescription className="text-(--warning-soft-fg) dark:text-(--warning-soft-fg)">
            {t('apikeys.dialogs.rotate.warning')}
          </AlertDescription>
        </Alert>

        {newKey ? (
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">{t('apikeys.dialogs.rotate.newKeyLabel')}</label>
              <div className="mt-1 flex items-center space-x-2">
                <code className="bg-muted flex-1 rounded-md p-3 font-mono text-sm break-all">{maskedNewKey}</code>
                <Button variant="outline" size="sm" onClick={handleCopy} className="flex-shrink-0">
                  {isCopied ? <CheckIcon className="h-4 w-4 text-(--success-soft-fg)" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>
            <Alert className="border-success/40 bg-success/10 dark:border-success/40">
              <AlertDescription className="text-(--success-soft-fg) dark:text-(--success-soft-fg)">
                {t('apikeys.dialogs.rotate.successMessage')}
              </AlertDescription>
            </Alert>
          </div>
        ) : null}

        <DialogFooter className="gap-2">
          <Button variant="outline" onClick={handleClose}>
            {newKey ? t('common.buttons.close') : t('common.buttons.cancel')}
          </Button>
          {!newKey && (
            <Button onClick={handleRotate} disabled={rotateApiKey.isPending}>
              {rotateApiKey.isPending ? (
                <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="mr-2 h-4 w-4" />
              )}
              {t('apikeys.dialogs.rotate.title')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
