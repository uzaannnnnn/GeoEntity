import { Entity } from '../types/entity';
import { StatusBadge } from './StatusBadge';
import { TypeBadge } from './TypeBadge';
import {
  ArrowLeftIcon,
  CenterMapIcon,
  EditIcon,
  TrashIcon,
} from './Icons';
import { formatCoordinate, formatDateTime } from '../utils/formatters';

interface EntityDetailProps {
  entity: Entity;
  onBack: () => void;
  onEdit: (entity: Entity) => void;
  onDelete: (entity: Entity) => void;
  onCenterMap: (entity: Entity) => void;
}

export function EntityDetail({
  entity,
  onBack,
  onEdit,
  onDelete,
  onCenterMap,
}: EntityDetailProps) {
  return (
    <div className="entity-detail-panel animate-fade-in">
      <div className="detail-top-nav">
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={onBack}
          aria-label="Back to entities list"
        >
          <ArrowLeftIcon size={16} />
          <span>All Entities</span>
        </button>

        <div className="detail-quick-actions">
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={() => onCenterMap(entity)}
            title="Center map on this entity"
          >
            <CenterMapIcon size={16} />
            <span>Center</span>
          </button>
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={() => onEdit(entity)}
            title="Edit entity"
          >
            <EditIcon size={16} />
            <span>Edit</span>
          </button>
          <button
            type="button"
            className="btn btn-danger-outline btn-sm"
            onClick={() => onDelete(entity)}
            title="Delete entity"
          >
            <TrashIcon size={16} />
            <span>Delete</span>
          </button>
        </div>
      </div>

      <div className="detail-content">
        <div className="detail-header-card">
          <div className="detail-badges">
            <TypeBadge type={entity.type} />
            <StatusBadge status={entity.status} />
          </div>
          <h2 className="detail-name">{entity.name}</h2>
          <div className="detail-id-row">
            <span className="detail-id-label">UUID:</span>
            <code className="detail-id-value">{entity.id}</code>
          </div>
        </div>

        <div className="detail-section">
          <h3 className="section-title">Description</h3>
          <p className="detail-description">
            {entity.description.trim() ? (
              entity.description
            ) : (
              <span className="text-muted italic">No description provided.</span>
            )}
          </p>
        </div>

        <div className="detail-section">
          <h3 className="section-title">Geographic Coordinates</h3>
          <div className="coords-grid">
            <div className="coord-box">
              <span className="coord-label">Latitude</span>
              <span className="coord-value">{formatCoordinate(entity.latitude)}&deg;</span>
            </div>
            <div className="coord-box">
              <span className="coord-label">Longitude</span>
              <span className="coord-value">{formatCoordinate(entity.longitude)}&deg;</span>
            </div>
          </div>
        </div>

        <div className="detail-section">
          <h3 className="section-title">Metadata & Timestamps</h3>
          <div className="metadata-table">
            <div className="meta-row">
              <span className="meta-key">Created</span>
              <span className="meta-val" title={entity.created_at}>
                {formatDateTime(entity.created_at)}
              </span>
            </div>
            <div className="meta-row">
              <span className="meta-key">Last Updated</span>
              <span className="meta-val" title={entity.updated_at}>
                {formatDateTime(entity.updated_at)}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
