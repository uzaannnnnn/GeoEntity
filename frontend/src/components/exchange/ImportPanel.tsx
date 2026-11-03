import { useState, useCallback } from 'react';
import {
  ImportFormat,
  ImportIdMode,
  ImportPreview,
  ImportResult,
} from '../../types/dataExchange';
import { dataExchangeApi } from '../../api/dataExchange';
import { ApiError } from '../../api/client';
import { FileSelector } from './FileSelector';
import { ImportPreviewSummary } from './ImportPreviewSummary';
import { ValidationTable } from './ValidationTable';
import { ImportResultView } from './ImportResultView';
import { AlertIcon, InfoIcon } from '../Icons';

interface ImportPanelProps {
  onImportSuccess: (result: ImportResult) => void;
  onDone: () => void;
}

export function ImportPanel({ onImportSuccess, onDone }: ImportPanelProps) {
  const [format, setFormat] = useState<ImportFormat>('csv');
  const [idMode, setIdMode] = useState<ImportIdMode>('preserve');
  const [selectedFile, setSelectedFile] = useState<File | null>(null);

  // Preview state
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [isPreviewLoading, setIsPreviewLoading] = useState<boolean>(false);
  const [previewError, setPreviewError] = useState<string | null>(null);

  // Commit execution state
  const [isCommitting, setIsCommitting] = useState<boolean>(false);
  const [commitError, setCommitError] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);

  // Format change invalidates active file and preview
  const handleFormatChange = (newFormat: ImportFormat) => {
    if (newFormat !== format) {
      setFormat(newFormat);
      setSelectedFile(null);
      setPreview(null);
      setPreviewError(null);
      setCommitError(null);
      setImportResult(null);
    }
  };

  // File change invalidates preview
  const handleFileSelect = (file: File | null) => {
    setSelectedFile(file);
    setPreview(null);
    setPreviewError(null);
    setCommitError(null);
  };

  // ID Mode change invalidates preview
  const handleIdModeChange = (newIdMode: ImportIdMode) => {
    if (newIdMode !== idMode) {
      setIdMode(newIdMode);
      setPreview(null);
      setPreviewError(null);
      setCommitError(null);
    }
  };

  // Request non-mutating preview from server
  const handleRequestPreview = useCallback(async () => {
    if (!selectedFile) return;

    setIsPreviewLoading(true);
    setPreviewError(null);
    setCommitError(null);

    try {
      const data = await dataExchangeApi.previewImport(format, selectedFile, idMode);
      setPreview(data);
    } catch (err) {
      setPreview(null);
      if (err instanceof ApiError) {
        setPreviewError(err.message);
      } else {
        setPreviewError('Failed to generate import preview. Please verify server connectivity.');
      }
    } finally {
      setIsPreviewLoading(false);
    }
  }, [format, selectedFile, idMode]);

  // Execute confirmed import
  const handleConfirmImport = useCallback(async () => {
    if (!selectedFile || !preview || !preview.preview_sha256) return;

    setIsCommitting(true);
    setCommitError(null);

    try {
      const result = await dataExchangeApi.commitImport(
        format,
        selectedFile,
        preview.preview_sha256,
        idMode
      );
      setImportResult(result);
      onImportSuccess(result);
    } catch (err) {
      if (err instanceof ApiError) {
        setCommitError(err.message);
      } else {
        setCommitError(
          'Network connection lost during import. The operation may have completed on the server; please check current entity counts or refresh the dashboard before retrying.'
        );
      }
    } finally {
      setIsCommitting(false);
    }
  }, [format, selectedFile, preview, idMode, onImportSuccess]);

  // Reset to import another file
  const handleReset = () => {
    setSelectedFile(null);
    setPreview(null);
    setPreviewError(null);
    setCommitError(null);
    setImportResult(null);
  };

  // If already imported successfully, render result summary
  if (importResult) {
    return (
      <ImportResultView
        result={importResult}
        onDone={onDone}
        onImportAnother={handleReset}
      />
    );
  }

  const isEligibleToConfirm = Boolean(
    preview &&
      preview.valid_rows > 0 &&
      preview.invalid_rows === 0 &&
      preview.errors.length === 0 &&
      (preview.id_mode === 'regenerate' || preview.existing_ids.length === 0)
  );

  return (
    <div className="import-panel">
      {/* Intro Guidance */}
      <div className="import-intro">
        <p className="intro-text">
          Import entities from CSV or GeoJSON in a single atomic transaction.
          Imports are <strong>strictly CREATE-ONLY</strong>; existing entities will never be overwritten.
        </p>
      </div>

      {/* Step 1: Format Toggle */}
      <div className="import-form-group">
        <label className="form-label" id="import-format-label">
          1. Choose File Format:
        </label>
        <div className="format-toggle-group" role="radiogroup" aria-labelledby="import-format-label">
          <button
            type="button"
            className={`btn-format-toggle ${format === 'csv' ? 'active' : ''}`}
            onClick={() => handleFormatChange('csv')}
            disabled={isPreviewLoading || isCommitting}
            role="radio"
            aria-checked={format === 'csv'}
          >
            <strong>CSV</strong>
            <span className="format-desc">Tabular records with JSON metadata columns</span>
          </button>
          <button
            type="button"
            className={`btn-format-toggle ${format === 'geojson' ? 'active' : ''}`}
            onClick={() => handleFormatChange('geojson')}
            disabled={isPreviewLoading || isCommitting}
            role="radio"
            aria-checked={format === 'geojson'}
          >
            <strong>GeoJSON</strong>
            <span className="format-desc">RFC 7946 FeatureCollection or Feature</span>
          </button>
        </div>
      </div>

      {/* Step 2: File Selector */}
      <div className="import-form-group">
        <label className="form-label">
          2. Select {format.toUpperCase()} File:
        </label>
        <FileSelector
          format={format}
          selectedFile={selectedFile}
          onFileSelect={handleFileSelect}
          disabled={isPreviewLoading || isCommitting}
        />
      </div>

      {/* Step 3: ID Mode Selection */}
      <div className="import-form-group">
        <label className="form-label" id="id-mode-label">
          3. Entity ID Handling:
        </label>
        <div className="id-mode-options" role="radiogroup" aria-labelledby="id-mode-label">
          <label className={`id-mode-card ${idMode === 'preserve' ? 'active' : ''}`}>
            <input
              type="radio"
              name="id_mode"
              value="preserve"
              checked={idMode === 'preserve'}
              onChange={() => handleIdModeChange('preserve')}
              disabled={isPreviewLoading || isCommitting}
            />
            <div className="id-mode-text">
              <strong>Preserve Source IDs</strong>
              <span>Retain original UUIDs from file. Import will reject if any ID already exists in database.</span>
            </div>
          </label>

          <label className={`id-mode-card ${idMode === 'regenerate' ? 'active' : ''}`}>
            <input
              type="radio"
              name="id_mode"
              value="regenerate"
              checked={idMode === 'regenerate'}
              onChange={() => handleIdModeChange('regenerate')}
              disabled={isPreviewLoading || isCommitting}
            />
            <div className="id-mode-text">
              <strong>Regenerate IDs</strong>
              <span>Generate new deterministic UUIDs for all entities, preventing ID collision conflicts.</span>
            </div>
          </label>
        </div>
      </div>

      {/* Step 4: Preview Trigger */}
      <div className="preview-action-row">
        <button
          type="button"
          className="btn btn-secondary btn-preview"
          onClick={handleRequestPreview}
          disabled={!selectedFile || isPreviewLoading || isCommitting}
        >
          {isPreviewLoading ? 'Analyzing File...' : 'Preview Import'}
        </button>
        <span className="preview-hint-text">
          <InfoIcon size={14} /> Preview is read-only and will NOT modify the database.
        </span>
      </div>

      {/* Preview Error Alert */}
      {previewError && (
        <div className="alert-box alert-error" role="alert">
          <AlertIcon size={16} />
          <div className="alert-content">
            <strong>Preview Failed:</strong> {previewError}
          </div>
        </div>
      )}

      {/* Step 5: Preview Results & Validation Details */}
      {preview && (
        <div className="preview-results-container">
          <ImportPreviewSummary preview={preview} />

          <ValidationTable
            format={format}
            errors={preview.errors}
            warnings={preview.warnings}
            errorsTruncated={preview.errors_truncated}
            warningsTruncated={preview.warnings_truncated}
          />
        </div>
      )}

      {/* Commit Error Alert */}
      {commitError && (
        <div className="alert-box alert-error" role="alert">
          <AlertIcon size={16} />
          <div className="alert-content">
            <strong>Execution Failed:</strong> {commitError}
          </div>
        </div>
      )}

      {/* Step 6: Confirmation & Execution Bar */}
      {preview && (
        <div className="import-confirm-bar">
          <div className="confirm-explainer">
            {isEligibleToConfirm ? (
              <span>
                Ready to create <strong>{preview.valid_rows}</strong> new entities.
              </span>
            ) : (
              <span className="text-danger">
                Resolve all validation errors above to enable import execution.
              </span>
            )}
          </div>

          <button
            type="button"
            className="btn btn-primary btn-confirm-import"
            onClick={handleConfirmImport}
            disabled={!isEligibleToConfirm || isCommitting}
            title={
              !isEligibleToConfirm
                ? 'Import is disabled because the file contains validation errors or ID conflicts'
                : `Execute atomic import of ${preview.valid_rows} entities`
            }
          >
            {isCommitting ? 'Importing Entities...' : `Import ${preview.valid_rows} Entities`}
          </button>
        </div>
      )}

      {isCommitting && (
        <div className="import-progress-indicator" role="status" aria-live="polite">
          <div className="progress-bar-indeterminate" />
          <span className="progress-label">
            Writing entities and metadata in atomic database transaction...
          </span>
        </div>
      )}
    </div>
  );
}
