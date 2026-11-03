import { ImportResult } from '../../types/dataExchange';
import { CheckIcon, TagIcon, CategoryIcon } from '../Icons';
import { IDMappingTable } from './IDMappingTable';

interface ImportResultViewProps {
  result: ImportResult;
  onDone: () => void;
  onImportAnother: () => void;
}

export function ImportResultView({
  result,
  onDone,
  onImportAnother,
}: ImportResultViewProps) {
  return (
    <div className="import-result-view">
      <div className="result-success-badge">
        <CheckIcon size={36} />
      </div>

      <h3 className="result-title">Import Successfully Completed</h3>
      <p className="result-subtitle">
        {result.created} {result.created === 1 ? 'entity has' : 'entities have'} been created in
        an atomic database transaction. Existing entities were not modified.
      </p>

      <div className="result-stats-card">
        <div className="result-stat-row">
          <span className="stat-label">Batch ID:</span>
          <span className="stat-code" title={result.batch_id}>
            {result.batch_id}
          </span>
        </div>
        <div className="result-stat-row">
          <span className="stat-label">Format:</span>
          <span className="stat-val">{result.format.toUpperCase()}</span>
        </div>
        <div className="result-stat-row">
          <span className="stat-label">Entities Created:</span>
          <span className="stat-val font-semibold">{result.created}</span>
        </div>
        {result.tags_created > 0 && (
          <div className="result-stat-row">
            <span className="stat-label">
              <TagIcon size={14} /> New Tags:
            </span>
            <span className="stat-val">{result.tags_created}</span>
          </div>
        )}
        {result.categories_created > 0 && (
          <div className="result-stat-row">
            <span className="stat-label">
              <CategoryIcon size={14} /> New Categories:
            </span>
            <span className="stat-val">{result.categories_created}</span>
          </div>
        )}
        {result.id_map && result.id_map.length > 0 && (
          <div className="result-stat-row">
            <span className="stat-label">IDs Regenerated:</span>
            <span className="stat-val">{result.id_map.length}</span>
          </div>
        )}
      </div>

      {result.id_map.length > 0 && (
        <IDMappingTable mappings={result.id_map} title="Created ID Mapping" />
      )}

      <div className="result-actions-row">
        <button
          type="button"
          className="btn btn-secondary"
          onClick={onImportAnother}
        >
          Import Another File
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={onDone}
        >
          Close & View Dashboard
        </button>
      </div>
    </div>
  );
}
