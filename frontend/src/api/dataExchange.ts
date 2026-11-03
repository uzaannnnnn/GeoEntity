import { API_BASE_URL, ApiError, getCSRFToken, notifyAuthenticationRequired } from './client';
import {
  ImportFormat,
  ImportIdMode,
  ImportPreview,
  ImportResult,
  ExportFilters,
} from '../types/dataExchange';
import { ApiErrorResponse } from '../types/entity';

/**
 * Sanitizes a filename from the Content-Disposition header.
 * Disallows path traversal slashes or control characters.
 */
export function sanitizeFilename(filename: string, fallback: string): string {
  if (!filename) return fallback;
  // Remove outer quotes
  let cleaned = filename.replace(/^["']|["']$/g, '').trim();
  // Strip any directory traversal or path separators
  cleaned = cleaned.replace(/^.*[\\/]/, '');
  // Disallow null bytes or control chars
  cleaned = cleaned.replace(/[\x00-\x1f\x7f]/g, '');
  if (!cleaned || cleaned === '.' || cleaned === '..') {
    return fallback;
  }
  return cleaned;
}

/**
 * Extracts filename from Content-Disposition header.
 */
export function extractFilenameFromHeader(header: string | null, fallback: string): string {
  if (!header) return fallback;
  // Match filename*="..." or filename="..." or filename=...
  const match = header.match(/filename\*?=(?:UTF-8'')?(?:["']?([^"';\n]+)["']?)/i);
  if (match && match[1]) {
    try {
      const decoded = decodeURIComponent(match[1]);
      return sanitizeFilename(decoded, fallback);
    } catch {
      return sanitizeFilename(match[1], fallback);
    }
  }
  return fallback;
}

/**
 * Safe browser download handler using temporary object URL.
 */
export function triggerBrowserDownload(blob: Blob, filename: string): void {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return;
  }
  const urlHelper = window.URL || URL;
  const url = urlHelper.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.setAttribute('download', filename);
  link.style.display = 'none';
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  setTimeout(() => {
    urlHelper.revokeObjectURL(url);
  }, 1000);
}

export const dataExchangeApi = {
  /**
   * Export all matching entities as CSV or GeoJSON.
   */
  async exportEntities(
    format: 'csv' | 'geojson',
    filters?: ExportFilters
  ): Promise<{ blob: Blob; filename: string }> {
    const params = new URLSearchParams();
    if (filters?.q && filters.q.trim()) {
      params.set('q', filters.q.trim());
    }
    if (filters?.type && filters.type !== 'all') {
      params.set('type', filters.type);
    }
    if (filters?.status && filters.status !== 'all') {
      params.set('status', filters.status);
    }
    if (format === 'csv' && filters?.spreadsheet_safe) {
      params.set('spreadsheet_safe', 'true');
    }

    const queryStr = params.toString();
    const url = `${API_BASE_URL}/exports/entities.${format}${queryStr ? `?${queryStr}` : ''}`;

    const response = await fetch(url, {
      method: 'GET',
      headers: {
        Accept: format === 'csv' ? 'text/csv' : 'application/geo+json',
      },
    });
    if (response.status === 401) notifyAuthenticationRequired();

    if (!response.ok) {
      const contentType = response.headers.get('content-type') || '';
      if (contentType.includes('application/json')) {
        try {
          const errorJson = (await response.json()) as ApiErrorResponse;
          if (errorJson?.error) {
            throw new ApiError(
              response.status,
              errorJson.error.code || 'EXPORT_ERROR',
              errorJson.error.message || `Export failed with status ${response.status}`,
              errorJson.error.field,
              errorJson.error.details
            );
          }
        } catch (err) {
          if (err instanceof ApiError) throw err;
        }
      }
      throw new ApiError(
        response.status,
        'EXPORT_FAILED',
        `Export request failed with status ${response.status}: ${response.statusText}`
      );
    }

    const disposition = response.headers.get('Content-Disposition');
    const fallbackFilename = `geoentity-entities-${Date.now()}.${format}`;
    const filename = extractFilenameFromHeader(disposition, fallbackFilename);
    const blob = await response.blob();

    return { blob, filename };
  },

  /**
   * Request read-only preview of an import file.
   */
  async previewImport(
    format: ImportFormat,
    file: File | Blob,
    idMode: ImportIdMode = 'preserve'
  ): Promise<ImportPreview> {
    const url = `${API_BASE_URL}/imports/${format}/preview?id_mode=${encodeURIComponent(idMode)}`;
    const contentType = format === 'csv' ? 'text/csv' : 'application/geo+json';

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': contentType,
        Accept: 'application/json',
        ...(getCSRFToken() ? { 'X-CSRF-Token': getCSRFToken() as string } : {}),
      },
      body: file,
    });
    if (response.status === 401) notifyAuthenticationRequired();

    const isJson = (response.headers.get('content-type') || '').includes('application/json');

    if (!response.ok) {
      if (isJson) {
        try {
          const errorJson = (await response.json()) as ApiErrorResponse;
          if (errorJson?.error) {
            throw new ApiError(
              response.status,
              errorJson.error.code || 'INVALID_IMPORT',
              errorJson.error.message || 'Import preview failed',
              errorJson.error.field,
              errorJson.error.details
            );
          }
        } catch (err) {
          if (err instanceof ApiError) throw err;
        }
      }
      throw new ApiError(
        response.status,
        'IMPORT_PREVIEW_FAILED',
        `Import preview failed with status ${response.status}: ${response.statusText}`
      );
    }

    if (!isJson) {
      throw new ApiError(
        response.status,
        'INVALID_RESPONSE',
        'Server did not return a JSON response'
      );
    }

    const result = await response.json();
    return result.data as ImportPreview;
  },

  /**
   * Confirm atomic execution of an import using exact previewed bytes and token.
   */
  async commitImport(
    format: ImportFormat,
    file: File | Blob,
    previewSha256: string,
    idMode: ImportIdMode = 'preserve'
  ): Promise<ImportResult> {
    const url = `${API_BASE_URL}/imports/${format}/commit?id_mode=${encodeURIComponent(idMode)}`;
    const contentType = format === 'csv' ? 'text/csv' : 'application/geo+json';

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': contentType,
        'X-Preview-SHA256': previewSha256,
        ...(getCSRFToken() ? { 'X-CSRF-Token': getCSRFToken() as string } : {}),
        Accept: 'application/json',
      },
      body: file,
    });
    if (response.status === 401) notifyAuthenticationRequired();

    const isJson = (response.headers.get('content-type') || '').includes('application/json');

    if (!response.ok) {
      if (isJson) {
        try {
          const errorJson = (await response.json()) as ApiErrorResponse;
          if (errorJson?.error) {
            throw new ApiError(
              response.status,
              errorJson.error.code || 'IMPORT_COMMIT_FAILED',
              errorJson.error.message || 'Import execution failed',
              errorJson.error.field,
              errorJson.error.details
            );
          }
        } catch (err) {
          if (err instanceof ApiError) throw err;
        }
      }
      throw new ApiError(
        response.status,
        'IMPORT_COMMIT_FAILED',
        `Import execution failed with status ${response.status}: ${response.statusText}`
      );
    }

    if (!isJson) {
      throw new ApiError(
        response.status,
        'INVALID_RESPONSE',
        'Server did not return a JSON response'
      );
    }

    const result = await response.json();
    return result.data as ImportResult;
  },
};
