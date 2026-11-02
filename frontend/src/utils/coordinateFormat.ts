/**
 * Formats a coordinate number to exactly 6 decimal places.
 * Example: -6.2088 -> "-6.208800"
 */
export function formatCoordinateSixDecimals(value: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return '0.000000';
  }
  return value.toFixed(6);
}

/**
 * Formats map center coordinates for copying.
 * Requirement: exactly "latitude, longitude" with 6 decimal places.
 * Example: "-6.208800, 106.845600"
 */
export function formatCenterCopyText(lat: number, lng: number): string {
  return `${formatCoordinateSixDecimals(lat)}, ${formatCoordinateSixDecimals(lng)}`;
}

/**
 * Copies plain text to the clipboard with fallback support for environments
 * where navigator.clipboard is unavailable or permission is denied.
 */
export async function copyTextToClipboard(text: string): Promise<boolean> {
  // Try modern asynchronous Clipboard API if available in secure context
  if (typeof navigator !== 'undefined' && navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fall through to legacy DOM execCommand fallback
    }
  }

  // Fallback for non-secure contexts or older browsers
  if (typeof document !== 'undefined') {
    try {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      textArea.style.position = 'fixed';
      textArea.style.top = '-9999px';
      textArea.style.left = '-9999px';
      textArea.setAttribute('readonly', '');
      document.body.appendChild(textArea);
      textArea.select();
      const successful = document.execCommand('copy');
      document.body.removeChild(textArea);
      return successful;
    } catch {
      return false;
    }
  }

  return false;
}
