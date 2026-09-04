/**
 * Summarizes a channel test response message for inline display:
 * takes the first non-empty line and truncates it to `maxLen` characters.
 */
export function summarizeTestResponse(message: string, maxLen = 120): {
  summary: string;
  truncated: boolean;
} {
  const trimmed = message.trim();
  if (!trimmed) return { summary: '', truncated: false };

  const lines = trimmed.split('\n');
  const firstLine = (lines.find((line) => line.trim().length > 0) ?? '').trim();
  const hasMoreLines = lines.slice(1).some((line) => line.trim().length > 0);

  if (firstLine.length > maxLen) {
    return { summary: `${firstLine.slice(0, maxLen)}…`, truncated: true };
  }
  return { summary: firstLine, truncated: hasMoreLines };
}

// `[429] Insufficient credits` — backend prefix (tester.go)
const bracketCodeRe = /^\[(\d{3})\]\s*/;
// `... with status 429 Too Many Requests` / `... with status 429` — httpclient error text
const withStatusRe = /with status (\d{3})\b/;

/**
 * Extracts an HTTP status code from a channel test error text.
 * Recognizes the backend `[429] message` prefix and the httpclient
 * `with status 429` phrasing. Returns null when no code is present.
 */
export function extractTestStatusCode(errorText: string): {
  code: string;
  message: string;
} | null {
  if (!errorText) return null;

  const bracket = errorText.match(bracketCodeRe);
  if (bracket) {
    return {
      code: bracket[1],
      message: errorText.slice(bracket[0].length).trim(),
    };
  }

  const withStatus = errorText.match(withStatusRe);
  if (withStatus) {
    return { code: withStatus[1], message: errorText.trim() };
  }

  return null;
}

/**
 * Pretty-prints a message when it is valid JSON (2-space indent).
 * Returns null for anything that does not parse as JSON.
 */
export function formatTestResponseJson(message: string): string | null {
  const trimmed = message.trim();
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return null;

  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch (_error) {
    return null;
  }
}
