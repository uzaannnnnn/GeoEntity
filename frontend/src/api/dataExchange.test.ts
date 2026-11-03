import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  dataExchangeApi,
  sanitizeFilename,
  extractFilenameFromHeader,
  triggerBrowserDownload,
} from './dataExchange';
import { ApiError } from './client';
import { ImportPreview, ImportResult } from '../types/dataExchange';

describe('dataExchange filename helpers', () => {
  it('sanitizes filenames and strips dangerous path traversal characters', () => {
    expect(sanitizeFilename('normal-file.csv', 'fallback.csv')).toBe('normal-file.csv');
    expect(sanitizeFilename('"quoted_file.geojson"', 'fallback.geojson')).toBe('quoted_file.geojson');
    expect(sanitizeFilename('../../etc/passwd', 'fallback.csv')).toBe('passwd');
    expect(sanitizeFilename('..\\..\\windows\\system32.csv', 'fallback.csv')).toBe('system32.csv');
    expect(sanitizeFilename('..', 'fallback.csv')).toBe('fallback.csv');
    expect(sanitizeFilename('', 'fallback.csv')).toBe('fallback.csv');
  });

  it('extracts filename from Content-Disposition headers', () => {
    const fallback = 'default.csv';
    expect(extractFilenameFromHeader(null, fallback)).toBe(fallback);
    expect(
      extractFilenameFromHeader(
        'attachment; filename="geoentity-entities-20261103-101500.csv"',
        fallback
      )
    ).toBe('geoentity-entities-20261103-101500.csv');
    expect(
      extractFilenameFromHeader(
        'attachment; filename=geoentity-entities-export.geojson',
        fallback
      )
    ).toBe('geoentity-entities-export.geojson');
    expect(
      extractFilenameFromHeader(
        "attachment; filename*=UTF-8''geoentity%20export.csv",
        fallback
      )
    ).toBe('geoentity export.csv');
  });
});

describe('dataExchange browser download helper', () => {
  it('triggers browser download and revokes object URL in browser environment', () => {
    const createObjectURLMock = vi.fn().mockReturnValue('blob:http://localhost/mock-uuid');
    const revokeObjectURLMock = vi.fn();
    const clickMock = vi.fn();
    const appendChildMock = vi.fn();
    const removeChildMock = vi.fn();

    const mockLink = {
      href: '',
      style: {},
      setAttribute: vi.fn(),
      click: clickMock,
    };

    const mockDocument = {
      createElement: vi.fn().mockReturnValue(mockLink),
      body: {
        appendChild: appendChildMock,
        removeChild: removeChildMock,
      },
    };

    vi.stubGlobal('window', {
      URL: {
        createObjectURL: createObjectURLMock,
        revokeObjectURL: revokeObjectURLMock,
      },
    });
    vi.stubGlobal('document', mockDocument);

    const blob = new Blob(['test,csv,data'], { type: 'text/csv' });
    triggerBrowserDownload(blob, 'export.csv');

    expect(createObjectURLMock).toHaveBeenCalledWith(blob);
    expect(mockLink.setAttribute).toHaveBeenCalledWith('download', 'export.csv');
    expect(clickMock).toHaveBeenCalled();
    expect(appendChildMock).toHaveBeenCalledWith(mockLink);
    expect(removeChildMock).toHaveBeenCalledWith(mockLink);

    vi.unstubAllGlobals();
  });
});

describe('dataExchangeApi.exportEntities', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('exports CSV with current search and category filters', async () => {
    const mockBlob = new Blob(['id,name,type\n'], { type: 'text/csv' });
    const mockResponse = {
      ok: true,
      status: 200,
      headers: {
        get: (header: string) => {
          if (header.toLowerCase() === 'content-disposition') {
            return 'attachment; filename="geoentity-entities-20261103-101500.csv"';
          }
          if (header.toLowerCase() === 'content-type') {
            return 'text/csv; charset=utf-8';
          }
          return null;
        },
      },
      blob: async () => mockBlob,
    };

    (globalThis.fetch as any).mockResolvedValueOnce(mockResponse);

    const result = await dataExchangeApi.exportEntities('csv', {
      q: 'Truck Alpha',
      type: 'vehicle',
      status: 'active',
      spreadsheet_safe: true,
    });

    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/exports/entities.csv?q=Truck+Alpha&type=vehicle&status=active&spreadsheet_safe=true'),
      expect.objectContaining({
        method: 'GET',
        headers: { Accept: 'text/csv' },
      })
    );
    expect(result.filename).toBe('geoentity-entities-20261103-101500.csv');
    expect(result.blob).toBe(mockBlob);
  });

  it('exports GeoJSON without pagination parameters', async () => {
    const mockBlob = new Blob(['{"type":"FeatureCollection"}'], { type: 'application/geo+json' });
    const mockResponse = {
      ok: true,
      status: 200,
      headers: {
        get: (header: string) => {
          if (header.toLowerCase() === 'content-disposition') {
            return 'attachment; filename="geoentity-entities-20261103-101500.geojson"';
          }
          return 'application/geo+json';
        },
      },
      blob: async () => mockBlob,
    };

    (globalThis.fetch as any).mockResolvedValueOnce(mockResponse);

    const result = await dataExchangeApi.exportEntities('geojson', {
      type: 'all',
      status: 'all',
    });

    // When type and status are 'all', they are omitted from query params
    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/exports/entities.geojson',
      expect.objectContaining({
        method: 'GET',
        headers: { Accept: 'application/geo+json' },
      })
    );
    expect(result.filename).toBe('geoentity-entities-20261103-101500.geojson');
  });

  it('throws ApiError when server returns 413 EXPORT_RESULT_LIMIT', async () => {
    const mockResponse = {
      ok: false,
      status: 413,
      statusText: 'Payload Too Large',
      headers: {
        get: (header: string) => (header.toLowerCase() === 'content-type' ? 'application/json' : null),
      },
      json: async () => ({
        error: {
          code: 'EXPORT_RESULT_LIMIT',
          message: 'Export exceeds the 100000 entity limit',
        },
      }),
    };

    (globalThis.fetch as any).mockResolvedValueOnce(mockResponse);

    await expect(dataExchangeApi.exportEntities('csv')).rejects.toThrow(ApiError);
    await (globalThis.fetch as any).mockResolvedValueOnce(mockResponse);
    try {
      await dataExchangeApi.exportEntities('csv');
    } catch (err: any) {
      expect(err.code).toBe('EXPORT_RESULT_LIMIT');
      expect(err.status).toBe(413);
    }
  });
});

describe('dataExchangeApi.previewImport', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('sends raw file bytes with correct content-type and parses preview DTO', async () => {
    const mockPreview: ImportPreview = {
      format: 'csv',
      total_rows: 10,
      valid_rows: 10,
      invalid_rows: 0,
      warnings: [],
      errors: [],
      errors_truncated: false,
      warnings_truncated: false,
      duplicate_candidates_truncated: false,
      existing_ids_truncated: false,
      new_catalog_items_truncated: false,
      duplicate_candidates: [],
      existing_ids: [],
      new_catalog_items: [],
      preview_sha256: 'mock_sha256_hash_123',
      id_mode: 'preserve',
      id_map: [],
    };

    const mockResponse = {
      ok: true,
      status: 200,
      headers: {
        get: () => 'application/json',
      },
      json: async () => ({ data: mockPreview }),
    };

    (globalThis.fetch as any).mockResolvedValueOnce(mockResponse);

    const testFile = new File(['name,type,status,latitude,longitude\nAlpha,vehicle,active,-6.2,106.8'], 'test.csv', {
      type: 'text/csv',
    });

    const result = await dataExchangeApi.previewImport('csv', testFile, 'preserve');

    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/imports/csv/preview?id_mode=preserve',
      expect.objectContaining({
        method: 'POST',
        headers: {
          'Content-Type': 'text/csv',
          Accept: 'application/json',
        },
        body: testFile,
      })
    );
    expect(result.preview_sha256).toBe('mock_sha256_hash_123');
    expect(result.valid_rows).toBe(10);
  });

  it('handles 400 INVALID_IMPORT with validation issue details', async () => {
    const mockResponse = {
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      headers: {
        get: () => 'application/json',
      },
      json: async () => ({
        error: {
          code: 'INVALID_IMPORT',
          message: 'Import contains invalid records',
          details: [
            { row: 2, field: 'latitude', code: 'INVALID_COORDINATE', message: 'Latitude must be between -90 and 90' },
          ],
        },
      }),
    };

    (globalThis.fetch as any).mockResolvedValueOnce(mockResponse);

    const testFile = new File(['bad,content'], 'test.csv');
    try {
      await dataExchangeApi.previewImport('csv', testFile);
      expect.fail('Should have thrown ApiError');
    } catch (err: any) {
      expect(err instanceof ApiError).toBe(true);
      expect(err.code).toBe('INVALID_IMPORT');
      expect(err.details).toHaveLength(1);
      expect(err.details[0].field).toBe('latitude');
    }
  });
});

describe('dataExchangeApi.commitImport', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('sends X-Preview-SHA256 header and exact file bytes to commit', async () => {
    const mockResult: ImportResult = {
      batch_id: '550e8400-e29b-41d4-a716-446655440000',
      format: 'geojson',
      imported: 5,
      created: 5,
      tags_created: 1,
      categories_created: 0,
      id_map: [],
    };

    const mockResponse = {
      ok: true,
      status: 201,
      headers: {
        get: () => 'application/json',
      },
      json: async () => ({ data: mockResult }),
    };

    (globalThis.fetch as any).mockResolvedValueOnce(mockResponse);

    const testFile = new File(['{"type":"FeatureCollection","features":[]}'], 'test.geojson', {
      type: 'application/geo+json',
    });

    const result = await dataExchangeApi.commitImport('geojson', testFile, 'sha256_token_abc', 'regenerate');

    expect(globalThis.fetch).toHaveBeenCalledWith(
      '/api/imports/geojson/commit?id_mode=regenerate',
      expect.objectContaining({
        method: 'POST',
        headers: {
          'Content-Type': 'application/geo+json',
          'X-Preview-SHA256': 'sha256_token_abc',
          Accept: 'application/json',
        },
        body: testFile,
      })
    );
    expect(result.created).toBe(5);
    expect(result.tags_created).toBe(1);
  });

  it('handles 409 PREVIEW_STALE when bytes differ from preview token', async () => {
    const mockResponse = {
      ok: false,
      status: 409,
      statusText: 'Conflict',
      headers: {
        get: () => 'application/json',
      },
      json: async () => ({
        error: {
          code: 'PREVIEW_STALE',
          message: 'Import content or options differ from the preview',
        },
      }),
    };

    (globalThis.fetch as any).mockResolvedValueOnce(mockResponse);

    const testFile = new File(['modified'], 'test.csv');
    await expect(
      dataExchangeApi.commitImport('csv', testFile, 'stale_token')
    ).rejects.toThrow(ApiError);
  });
});
