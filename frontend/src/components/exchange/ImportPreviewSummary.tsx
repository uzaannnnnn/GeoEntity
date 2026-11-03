import { ImportPreview } from '../../types/dataExchange';
import { CheckIcon, AlertIcon, InfoIcon, TagIcon, CategoryIcon } from '../Icons';
import { IDMappingTable } from './IDMappingTable';

interface ImportPreviewSummaryProps {
  preview: ImportPreview;
}

export function ImportPreviewSummary({ preview }: ImportPreviewSummaryProps) {
  const hasErrors = preview.invalid_rows > 0 || preview.errors.length > 0;
  const hasExistingIdConflicts = preview.id_mode === 'preserve' && preview.existing_ids.length > 0;
  const isEligible = !hasErrors && !hasExistingIdConflicts && preview.valid_rows > 0;

  return (
    <div className="import-preview-summary">
      {/* Metric Stat Cards */}
      <div className="preview-metrics-row">
        <div className="metric-card metric-total">
          <span className="metric-label">Total Records</span>
          <span className="metric-val">{preview.total_rows}</span>
        </div>
        <div className={`metric-card ${preview.valid_rows > 0 ? 'metric-valid' : ''}`}>
          <span className="metric-label">Valid Records</span>
          <span className="metric-val">{preview.valid_rows}</span>
        </div>
        <div className={`metric-card ${preview.invalid_rows > 0 ? 'metric-invalid' : ''}`}>
          <span className="metric-label">Invalid Records</span>
          <span className="metric-val">{preview.invalid_rows}</span>
        </div>
        <div className={`metric-card ${preview.warnings.length > 0 ? 'metric-warnings' : ''}`}>
          <span className="metric-label">Warnings</span>
          <span className="metric-val">{preview.warnings.length}</span>
        </div>
      </div>

      {/* Authoritative Status Banner */}
      {isEligible ? (
        <div className="preview-status-banner banner-success" role="status">
          <div className="banner-icon">
            <CheckIcon size={20} />
          </div>
          <div className="banner-body">
            <strong>Ready to Import {preview.valid_rows} Entities</strong>
            <p>
              All records passed validation. When confirmed, entities will be created in a single
              atomic database transaction. Existing entities will not be modified.
            </p>
          </div>
        </div>
      ) : (
        <div className="preview-status-banner banner-danger" role="alert">
          <div className="banner-icon">
            <AlertIcon size={20} />
          </div>
          <div className="banner-body">
            <strong>Import Blocked: Errors Detected</strong>
            <p>
              {hasExistingIdConflicts
                ? `Conflict: ${preview.existing_ids.length} entity ID(s) already exist in the database. Choose "Regenerate IDs" to assign fresh UUIDs or remove duplicate IDs from the file.`
                : hasErrors
                ? `The file contains ${preview.invalid_rows} invalid record(s). Imports are strictly CREATE-ONLY and atomic; all validation errors must be resolved before confirmation.`
                : 'No valid records found in file.'}
            </p>
          </div>
        </div>
      )}

      {/* New Catalog Items (Tags / Categories to be created) */}
      {preview.new_catalog_items.length > 0 && (
        <div className="preview-catalog-notice">
          <div className="notice-header">
            <InfoIcon size={16} />
            <span>
              New Catalog Items to be Created ({preview.new_catalog_items.length}
              {preview.new_catalog_items_truncated ? '+' : ''})
            </span>
          </div>
          <div className="catalog-pills-list">
            {preview.new_catalog_items.map((item, idx) => (
              <span
                key={`cat-${idx}`}
                className={`catalog-pill pill-${item.kind === 'tag' ? 'tags' : 'categories'}`}
              >
                {item.kind === 'tag' ? <TagIcon size={12} /> : <CategoryIcon size={12} />}
                <span className="pill-kind">{item.kind === 'tag' ? 'Tag' : 'Category'}:</span>
                <span className="pill-name">{item.name}</span>
              </span>
            ))}
          </div>
        </div>
      )}

      {preview.id_map.length > 0 && (
        <IDMappingTable mappings={preview.id_map} title="Preview ID Mapping" />
      )}

      {/* Duplicate Candidates Warning */}
      {preview.duplicate_candidates.length > 0 && (
        <div className="preview-duplicates-notice">
          <div className="notice-header">
            <InfoIcon size={16} />
            <span>
              Existing Name Matches ({preview.duplicate_candidates.length}
              {preview.duplicate_candidates_truncated ? '+' : ''})
            </span>
          </div>
          <p className="notice-sub">
            Some records match existing entity names. GeoEntity names are not required to be unique,
            so this will not block import.
          </p>
        </div>
      )}
    </div>
  );
}
