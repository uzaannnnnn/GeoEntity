import { useState } from 'react';
import { Entity } from '../types/entity';
import { StatusBadge } from './StatusBadge';
import { TypeBadge } from './TypeBadge';
import {
  ArrowLeftIcon,
  CenterMapIcon,
  EditIcon,
  TrashIcon,
  InfoIcon,
  AttributesIcon,
  TagIcon,
  ActivityIcon,
  GeofenceIcon,
} from './Icons';
import { formatCoordinate, formatDateTime } from '../utils/formatters';
import { AttributeValuesSection } from './advanced/AttributeValuesSection';
import { TagsCategoriesSection } from './advanced/TagsCategoriesSection';
import { ActivityTimelineSection } from './advanced/ActivityTimelineSection';
import { EntityGeofencesSection } from './geofences/EntityGeofencesSection';

export type EntityDetailTab = 'overview' | 'attributes' | 'tags' | 'activity' | 'geofences';

interface EntityDetailProps {
  entity: Entity;
  onBack: () => void;
  onEdit: (entity: Entity) => void;
  onDelete: (entity: Entity) => void;
  onCenterMap: (entity: Entity) => void;
  onToast?: (type: 'success' | 'error' | 'info', message: string, title?: string) => void;
  onViewGeofenceEvents?: (geofenceId: string, geofenceName: string) => void;
  onSelectGeofenceOnMap?: (geofenceId: string) => void;
}

export function EntityDetail({
  entity,
  onBack,
  onEdit,
  onDelete,
  onCenterMap,
  onToast = () => {},
  onViewGeofenceEvents,
  onSelectGeofenceOnMap,
}: EntityDetailProps) {
  const [activeTab, setActiveTab] = useState<EntityDetailTab>('overview');

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
            title="Edit entity base properties"
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

        {/* Tab Navigation */}
        <div className="detail-tabs-bar" role="tablist" aria-label="Entity Details Sections">
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'overview'}
            aria-controls="panel-overview"
            id="tab-overview"
            className={`detail-tab-btn ${activeTab === 'overview' ? 'active' : ''}`}
            onClick={() => setActiveTab('overview')}
          >
            <InfoIcon size={15} />
            <span>Overview</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'attributes'}
            aria-controls="panel-attributes"
            id="tab-attributes"
            className={`detail-tab-btn ${activeTab === 'attributes' ? 'active' : ''}`}
            onClick={() => setActiveTab('attributes')}
          >
            <AttributesIcon size={15} />
            <span>Attributes</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'tags'}
            aria-controls="panel-tags"
            id="tab-tags"
            className={`detail-tab-btn ${activeTab === 'tags' ? 'active' : ''}`}
            onClick={() => setActiveTab('tags')}
          >
            <TagIcon size={15} />
            <span>Tags &amp; Categories</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'activity'}
            aria-controls="panel-activity"
            id="tab-activity"
            className={`detail-tab-btn ${activeTab === 'activity' ? 'active' : ''}`}
            onClick={() => setActiveTab('activity')}
          >
            <ActivityIcon size={15} />
            <span>Activity</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'geofences'}
            aria-controls="panel-geofences"
            id="tab-geofences"
            className={`detail-tab-btn ${activeTab === 'geofences' ? 'active' : ''}`}
            onClick={() => setActiveTab('geofences')}
          >
            <GeofenceIcon size={15} />
            <span>Geofences</span>
          </button>
        </div>

        {/* Tab Panels */}
        <div className="detail-tab-body">
          {activeTab === 'overview' && (
            <div
              id="panel-overview"
              role="tabpanel"
              aria-labelledby="tab-overview"
              className="tab-panel animate-fade-in"
            >
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
                <h3 className="section-title">Metadata &amp; Timestamps</h3>
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
          )}

          {activeTab === 'attributes' && (
            <div
              id="panel-attributes"
              role="tabpanel"
              aria-labelledby="tab-attributes"
              className="tab-panel animate-fade-in"
            >
              <AttributeValuesSection key={`${entity.id}:${entity.type}`} entity={entity} onToast={onToast} />
            </div>
          )}

          {activeTab === 'tags' && (
            <div
              id="panel-tags"
              role="tabpanel"
              aria-labelledby="tab-tags"
              className="tab-panel animate-fade-in"
            >
              <TagsCategoriesSection key={entity.id} entity={entity} onToast={onToast} />
            </div>
          )}

          {activeTab === 'activity' && (
            <div
              id="panel-activity"
              role="tabpanel"
              aria-labelledby="tab-activity"
              className="tab-panel animate-fade-in"
            >
              <ActivityTimelineSection key={entity.id} entity={entity} />
            </div>
          )}

          {activeTab === 'geofences' && (
            <div
              id="panel-geofences"
              role="tabpanel"
              aria-labelledby="tab-geofences"
              className="tab-panel animate-fade-in"
            >
              <EntityGeofencesSection
                key={entity.id}
                entity={entity}
                onViewGeofenceEvents={onViewGeofenceEvents}
                onSelectGeofenceOnMap={onSelectGeofenceOnMap}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
