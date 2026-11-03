import React, { useRef, useState, useCallback } from 'react';
import { ImportFormat } from '../../types/dataExchange';
import { UploadIcon, FileTextIcon, CloseIcon, AlertIcon } from '../Icons';

interface FileSelectorProps {
  format: ImportFormat;
  selectedFile: File | null;
  onFileSelect: (file: File | null) => void;
  disabled?: boolean;
}

const MAX_BYTES = 5 * 1024 * 1024; // 5 MiB

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

export function FileSelector({
  format,
  selectedFile,
  onFileSelect,
  disabled = false,
}: FileSelectorProps) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [isDragOver, setIsDragOver] = useState(false);
  const [clientError, setClientError] = useState<string | null>(null);

  const accept = format === 'csv' ? '.csv,text/csv' : '.geojson,.json,application/geo+json,application/json';

  const validateAndSelect = useCallback(
    (file: File) => {
      setClientError(null);
      if (!file) return;

      if (file.size === 0) {
        setClientError('The selected file is empty.');
        onFileSelect(null);
        return;
      }

      if (file.size > MAX_BYTES) {
        setClientError(`File size (${formatFileSize(file.size)}) exceeds the 5 MiB limit.`);
        onFileSelect(null);
        return;
      }

      const lowerName = file.name.toLowerCase();
      if (format === 'csv' && !lowerName.endsWith('.csv')) {
        setClientError('Please select a valid .csv file.');
        onFileSelect(null);
        return;
      }
      if (format === 'geojson' && !lowerName.endsWith('.geojson') && !lowerName.endsWith('.json')) {
        setClientError('Please select a valid .geojson or .json file.');
        onFileSelect(null);
        return;
      }

      onFileSelect(file);
    },
    [format, onFileSelect]
  );

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      validateAndSelect(file);
    }
  };

  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragOver(false);
    if (disabled) return;
    const file = e.dataTransfer.files?.[0];
    if (file) {
      validateAndSelect(file);
    }
  };

  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    if (!disabled) {
      setIsDragOver(true);
    }
  };

  const handleDragLeave = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragOver(false);
  };

  const handleClear = () => {
    setClientError(null);
    onFileSelect(null);
    if (inputRef.current) {
      inputRef.current.value = '';
    }
  };

  return (
    <div className="file-selector-container">
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        onChange={handleInputChange}
        disabled={disabled}
        className="file-hidden-input"
        id="file-upload-input"
        aria-label={`Select ${format.toUpperCase()} file to import`}
      />

      {!selectedFile ? (
        <div
          className={`file-dropzone ${isDragOver ? 'drag-over' : ''} ${disabled ? 'disabled' : ''}`}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onClick={() => {
            if (!disabled) inputRef.current?.click();
          }}
          onKeyDown={(e) => {
            if (!disabled && (e.key === 'Enter' || e.key === ' ')) {
              e.preventDefault();
              inputRef.current?.click();
            }
          }}
          role="button"
          tabIndex={disabled ? -1 : 0}
          aria-label={`Click or drag and drop to select a ${format.toUpperCase()} file`}
        >
          <div className="dropzone-icon">
            <UploadIcon size={28} />
          </div>
          <div className="dropzone-text">
            <span className="dropzone-title">
              Click to choose a {format.toUpperCase()} file or drag and drop
            </span>
            <span className="dropzone-sub">
              {format === 'csv'
                ? 'Supported: .csv up to 5 MiB (max 5,000 rows)'
                : 'Supported: .geojson or .json up to 5 MiB (max 5,000 features)'}
            </span>
          </div>
        </div>
      ) : (
        <div className="file-selected-card">
          <div className="file-info-group">
            <div className="file-icon-badge">
              <FileTextIcon size={20} />
            </div>
            <div className="file-details">
              <span className="file-name" title={selectedFile.name}>
                {selectedFile.name}
              </span>
              <span className="file-meta">
                {formatFileSize(selectedFile.size)} • {selectedFile.type || format.toUpperCase()}
              </span>
            </div>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-clear-file"
            onClick={handleClear}
            disabled={disabled}
            aria-label="Remove selected file"
            title="Remove selected file"
          >
            <CloseIcon size={16} />
            <span>Remove</span>
          </button>
        </div>
      )}

      {clientError && (
        <div className="file-client-error" role="alert">
          <AlertIcon size={14} />
          <span>{clientError}</span>
        </div>
      )}

      <div className="file-format-hints">
        {format === 'csv' ? (
          <p className="format-hint-text">
            <strong>CSV Schema:</strong> Required headers: <code>name, type, status, latitude, longitude</code>. Optional:{' '}
            <code>id, description, created_at, updated_at, attributes_json, previous_type_values_json, tags_json, categories_json</code>.
          </p>
        ) : (
          <p className="format-hint-text">
            <strong>GeoJSON Schema:</strong> RFC 7946 <code>FeatureCollection</code> or <code>Feature</code> with Point geometry{' '}
            <code>[longitude, latitude]</code>. Properties: <code>name, type, status, description, attributes, tags, categories</code>.
          </p>
        )}
      </div>
    </div>
  );
}
