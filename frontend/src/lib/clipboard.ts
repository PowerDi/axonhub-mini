/*
 * Clipboard helper with a graceful degradation path.
 *
 * `navigator.clipboard` is only exposed in a secure context (HTTPS or localhost). AxonHub is
 * frequently self-hosted and reached over plain HTTP on a LAN, where the async Clipboard API is
 * either missing entirely or present but rejecting. The legacy `document.execCommand('copy')`
 * branch below is the only thing that makes copy buttons work on those deployments, so please
 * do not delete it as dead code.
 */

// copyLegacy copies via a temporary textarea, the only path available outside a secure context.
function copyLegacy(text: string): void {
  if (typeof document === 'undefined') {
    throw new Error('Clipboard unavailable: no DOM');
  }

  if (typeof document.execCommand !== 'function') {
    throw new Error('Clipboard unavailable: execCommand is not supported');
  }

  const activeElement = document.activeElement as HTMLElement | null;
  const previousActiveElement = typeof activeElement?.focus === 'function' ? activeElement : null;
  const selection = typeof window !== 'undefined' && typeof window.getSelection === 'function' ? window.getSelection() : null;
  const previousRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null;

  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.setAttribute('aria-hidden', 'true');
  textarea.setAttribute('tabindex', '-1');
  // Keep the node inside the viewport: an off-screen offset such as `left: -9999px` still makes
  // some browsers scroll when the node is focused.
  textarea.style.position = 'fixed';
  textarea.style.top = '0';
  textarea.style.left = '0';
  textarea.style.width = '1px';
  textarea.style.height = '1px';
  textarea.style.padding = '0';
  textarea.style.border = 'none';
  textarea.style.outline = 'none';
  textarea.style.boxShadow = 'none';
  textarea.style.background = 'transparent';
  textarea.style.opacity = '0.01';
  textarea.style.pointerEvents = 'none';

  document.body.appendChild(textarea);

  try {
    textarea.focus({ preventScroll: true });
    textarea.select();
    // iOS Safari ignores select() on a readonly textarea, setSelectionRange is the reliable path.
    textarea.setSelectionRange(0, text.length);

    if (!document.execCommand('copy')) {
      throw new Error('Clipboard unavailable: execCommand("copy") failed');
    }
  } finally {
    textarea.remove();

    // Restore whatever the user had selected and focused before we hijacked it.
    if (previousRange && selection) {
      selection.removeAllRanges();
      selection.addRange(previousRange);
    }
    previousActiveElement?.focus({ preventScroll: true });
  }
}

// copyTextToClipboard writes text to the clipboard, falling back to the legacy path outside a
// secure context. It rejects when no path succeeds so callers can surface their own error state.
export async function copyTextToClipboard(text: string): Promise<void> {
  const canUseAsyncClipboard =
    typeof navigator !== 'undefined' &&
    typeof navigator.clipboard?.writeText === 'function' &&
    typeof window !== 'undefined' &&
    window.isSecureContext === true;

  if (canUseAsyncClipboard) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Some browsers expose the API over HTTP but reject the write, so fall through to legacy.
    }
  }

  copyLegacy(text);
}
