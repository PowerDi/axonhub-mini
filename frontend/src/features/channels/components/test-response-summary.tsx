'use client';

import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { extractTestStatusCode, formatTestResponseJson, summarizeTestResponse } from '../utils/test-response';

interface Props {
  message: string;
  error?: string;
  className?: string;
}

/**
 * Inline display of a channel test outcome:
 * - failure: HTTP status code badge (when extractable) + error text
 * - response message: pretty-printed JSON block, or the one-line
 *   summary with a Popover for the full text
 */
export function TestResponseSummary({ message, error, className = '' }: Props) {
  const { t } = useTranslation();

  if (error) {
    const extracted = extractTestStatusCode(error);
    const code = extracted?.code;
    // With the `[429] ` prefix the code lives in the badge; fall back to the
    // full error text otherwise so nothing is lost.
    const errorText = extracted && extracted.message ? extracted.message : error;

    return (
      <div className={`max-w-full space-y-1 ${className}`}>
        {code && (
          <div>
            <Badge variant='outline' className='text-(--destructive-soft-fg) font-mono text-xs'>
              {code}
            </Badge>
          </div>
        )}
        <div className='text-(--destructive-soft-fg) text-xs break-words whitespace-pre-wrap'>{errorText}</div>
      </div>
    );
  }

  if (!message) return null;

  const json = formatTestResponseJson(message);
  if (json) {
    return (
      <div className={`max-w-full ${className}`}>
        <pre className='text-muted-foreground max-h-48 overflow-auto rounded-md border bg-muted/50 p-2 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words'>
          {json}
        </pre>
      </div>
    );
  }

  const { summary, truncated } = summarizeTestResponse(message);
  if (!summary) return null;

  if (truncated) {
    return (
      <div className={className}>
        <Popover>
          <PopoverTrigger asChild>
            <button
              type='button'
              className='text-muted-foreground block w-full max-w-full truncate text-left text-xs hover:underline focus:outline-none'
              title={summary}
            >
              {summary}
            </button>
          </PopoverTrigger>
          <PopoverContent className='w-80 p-3' align='start'>
            <div className='text-muted-foreground mb-1.5 text-xs font-medium'>{t('channels.dialogs.test.responsePreview')}</div>
            <div className='max-h-48 overflow-y-auto text-xs break-words whitespace-pre-wrap'>{message}</div>
          </PopoverContent>
        </Popover>
      </div>
    );
  }

  return (
    <div className={`text-muted-foreground truncate text-xs ${className}`} title={summary}>
      {summary}
    </div>
  );
}

const inlineSummaryMaxLen = 80;

interface TestResultInlineProps {
  error?: string;
  message?: string;
  className?: string;
}

/**
 * Single-line inline display of a channel test outcome, meant to sit right
 * under the status badge without ever growing the row:
 * - failure: `[status code] truncated error text` in destructive color
 * - success: truncated first-line response summary
 * Clicking the line opens a Popover with the full content (JSON messages
 * stay pretty-printed), mirroring TestResponseSummary's Popover pattern.
 */
export function TestResultInline({ error, message, className = '' }: TestResultInlineProps) {
  const { t } = useTranslation();

  const popoverBody = (content: string) => {
    const json = formatTestResponseJson(content);
    if (json) {
      return (
        <pre className='max-h-64 overflow-auto rounded-md border bg-muted/50 p-2 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words'>
          {json}
        </pre>
      );
    }
    return <div className='max-h-64 overflow-y-auto text-xs break-words whitespace-pre-wrap'>{content}</div>;
  };

  if (error) {
    const extracted = extractTestStatusCode(error);
    const code = extracted?.code;
    const errorText = extracted?.message ? extracted.message : error;
    const { summary } = summarizeTestResponse(errorText, inlineSummaryMaxLen);
    if (!summary) return null;

    return (
      <Popover>
        <PopoverTrigger asChild>
          <button
            type='button'
            className={`flex w-full max-w-full min-w-0 items-center gap-1 text-left focus:outline-none ${className}`}
          >
            {code && (
              <Badge variant='outline' className='text-(--destructive-soft-fg) shrink-0 px-1.5 font-mono text-[10px]'>
                {code}
              </Badge>
            )}
            <span className='text-(--destructive-soft-fg) truncate text-xs hover:underline'>{summary}</span>
          </button>
        </PopoverTrigger>
        <PopoverContent className='w-96 max-w-[80vw] p-3' align='start'>
          <div className='text-muted-foreground mb-1.5 text-xs font-medium'>{t('channels.dialogs.test.errorPreview')}</div>
          {popoverBody(errorText)}
        </PopoverContent>
      </Popover>
    );
  }

  if (!message) return null;

  const { summary } = summarizeTestResponse(message, inlineSummaryMaxLen);
  if (!summary) return null;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type='button'
          className={`flex w-full max-w-full min-w-0 items-center text-left focus:outline-none ${className}`}
        >
          <span className='text-muted-foreground truncate text-xs hover:underline'>{summary}</span>
        </button>
      </PopoverTrigger>
      <PopoverContent className='w-96 max-w-[80vw] p-3' align='start'>
        <div className='text-muted-foreground mb-1.5 text-xs font-medium'>{t('channels.dialogs.test.responsePreview')}</div>
        {popoverBody(message)}
      </PopoverContent>
    </Popover>
  );
}
