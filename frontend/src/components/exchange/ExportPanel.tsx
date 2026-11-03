import { useState, useCallback } from 'react';
import { EntityListFilters } from '../EntityList';
import { dataExchangeApi, triggerBrowserDownload } from '../../api/dataExchange';
import { ApiError } from '../../api/client';
import { DownloadIcon, AlertIcon, CheckIcon, InfoIcon } from '../Icons';

interface ExportPanelProps {
  activeFilters: EntityListFilters;
  matchingCount: number;
}

export function ExportPanel({ activeFilters, matchingCount }: ExportPanelProps) {
  const [format, setFormat] = useState<'csv' | 'geojson'>('csv');
  const [spreadsheetSafe, setSpreadsheetSafe] = useState<boolean>(true);
  const [isExporting, setIsExporting] = useState<boolean>(false);
  const [exportError, setExportError] = useState<string | null>(null);
  const [successFilename, setSuccessFilename] = useState<string | null>(null);

  const hasFilters = Boolean(
    activeFilters.q.trim() || activeFilters.type !== 'all' || activeFilters.status !== 'all'
  );

  const handleExport = useCallback(async () => {
    setIsExporting(true);
    setExportError(null);
    setSuccessFilename(null);

    try {
      const { blob, filename } = await dataExchangeApi.exportEntities(format, {
        q: activeFilters.q,
        type: activeFilters.type,
        status: activeFilters.status,
        spreadsheet_safe: format === 'csv' ? spreadsheetSafe : undefined,
      });

      triggerBrowserDownload(blob, filename);
      setSuccessFilename(filename);
    } catch (err) {
      if (err instanceof ApiError) {
        setExportError(err.message);
      } else {
        setExportError('Failed to generate export file. Please check server connection.');
      }
    } finally {
      setIsExporting(false);
    }
  }, [format, activeFilters, spreadsheetSafe]);

  return (
    <div className="export-panel">
      <div className="export-intro">
        <p className="intro-text">
          Download entities matching your active filter and search scope. Export operations retrieve
          the complete matching dataset and are not bounded by list pagination pages.
        </p>
      </div>

      {/* Scope Summary Box */}
      <div className="export-scope-card">
        <div className="scope-header">
          <span className="scope-title">Export Scope</span>
          <span className="scope-count-badge">
            {matchingCount} {matchingCount === 1 ? 'entity' : 'entities'}
          </span>
        </div>
        <div className="scope-details-grid">
          <div className="scope-item">
            <span className="scope-label">Search Query:</span>
            <span className="scope-value">
              {activeFilters.q.trim() ? `"${activeFilters.q.trim()}"` : 'None (all)'}
            </span>
          </div>
          <div className="scope-item">
            <span className="scope-label">Type Filter:</span>
            <span className="scope-value">
              {activeFilters.type !== 'all' ? activeFilters.type : 'All Types'}
            </span>
          </div>
          <div className="scope-item">
            <span className="scope-label">Status Filter:</span>
            <span className="scope-value">
              {activeFilters.status !== 'all' ? activeFilters.status : 'All Statuses'}
            </span>
          </div>
        </div>
        {hasFilters && (
          <p className="scope-note">
            <InfoIcon size={14} /> Only entities satisfying the above filters will be exported.
          </p>
        )}
      </div>

      {/* Format Selection */}
      <div className="export-form-group">
        <label className="form-label" id="export-format-label">
          Choose Export Format:
        </label>
        <div className="format-toggle-group" role="radiogroup" aria-labelledby="export-format-label">
          <button
            type="button"
            className={`btn-format-toggle ${format === 'csv' ? 'active' : ''}`}
            onClick={() => {
              setFormat('csv');
              setSuccessFilename(null);
            }}
            role="radio"
            aria-checked={format === 'csv'}
          >
            <strong>CSV</strong>
            <span className="format-desc">Tabular dataset with JSON metadata columns</span>
          </button>
          <button
            type="button"
            className={`btn-format-toggle ${format === 'geojson' ? 'active' : ''}`}
            onClick={() => {
              setFormat('geojson');
              setSuccessFilename(null);
            }}
            role="radio"
            aria-checked={format === 'geojson'}
          >
            <strong>GeoJSON</strong>
            <span className="format-desc">RFC 7946 FeatureCollection with Point coordinates</span>
          </button>
        </div>
      </div>

      {/* Format-Specific Options */}
      {format === 'csv' && (
        <div className="export-options-box">
          <label className="checkbox-option-label">
            <input
              type="checkbox"
              checked={spreadsheetSafe}
              onChange={(e) => setSpreadsheetSafe(e.target.checked)}
              disabled={isExporting}
            />
            <span className="option-title">Spreadsheet-safe formula prefixing</span>
          </label>
          <p className="option-help">
            Prefixes cells that begin with formula operators (<code>=</code>, <code>+</code>, <code>-</code>, <code>@</code>)
            with a single quote (<code>'</code>) to prevent automatic formula execution in spreadsheet applications
            (Excel, LibreOffice Calc).
          </p>
        </div>
      )}

      {/* Error Feedback */}
      {exportError && (
        <div className="alert-box alert-error" role="alert">
          <AlertIcon size={16} />
          <div className="alert-content">
            <strong>Export Error:</strong> {exportError}
          </div>
        </div>
      )}

      {/* Success Feedback */}
      {successFilename && (
        <div className="alert-box alert-success" role="status">
          <CheckIcon size={16} />
          <div className="alert-content">
            <strong>Download Started:</strong> Saved as <code>{successFilename}</code>
          </div>
        </div>
      )}

      {/* Action Footer */}
      <div className="export-actions-row">
        <button
          type="button"
          className="btn btn-primary btn-export"
          onClick={handleExport}
          disabled={isExporting || matchingCount === 0}
          title={matchingCount === 0 ? 'No matching entities to export' : `Export ${matchingCount} entities`}
        >
          <DownloadIcon size={16} />
          <span>{isExporting ? 'Generating Export...' : `Export ${format.toUpperCase()} (${matchingCount})`}</span>
        </button>
      </div>
    </div>
  );
}
