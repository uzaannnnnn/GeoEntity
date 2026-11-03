import { GeofenceDefinition } from '../../types/geofence';
import { formatDateTime, formatRadius } from '../../utils/formatters';
import {
  GeofenceIcon,
  PlusIcon,
  EditIcon,
  TrashIcon,
  ActivityIcon,
} from '../Icons';

interface GeofenceListProps {
  geofences: GeofenceDefinition[];
  selectedGeofenceId?: string | null;
  onSelectGeofence: (geofence: GeofenceDefinition) => void;
  onToggleEnabled: (geofence: GeofenceDefinition) => Promise<void>;
  onEditGeofence: (geofence: GeofenceDefinition) => void;
  onDeleteGeofence: (geofence: GeofenceDefinition) => void;
  onViewEvents: (geofence: GeofenceDefinition) => void;
  onCreateClick: () => void;
  isLoading?: boolean;
}

export function GeofenceList({
  geofences,
  selectedGeofenceId,
  onSelectGeofence,
  onToggleEnabled,
  onEditGeofence,
  onDeleteGeofence,
  onViewEvents,
  onCreateClick,
  isLoading = false,
}: GeofenceListProps) {
  const enabledCount = geofences.filter((g) => g.enabled).length;

  return (
    <div className="geofence-list-container">
      <div className="list-top-bar">
        <div className="list-stats-info">
          <span>
            Total Geofences: <strong>{geofences.length}</strong> (
            <span className="text-success">{enabledCount} Active</span>,{' '}
            <span className="text-muted">{geofences.length - enabledCount} Disabled</span>)
          </span>
        </div>

        <button
          type="button"
          className="btn btn-primary btn-sm"
          onClick={onCreateClick}
          title="Create a new geofence"
        >
          <PlusIcon size={16} />
          <span>New Geofence</span>
        </button>
      </div>

      {isLoading && geofences.length === 0 && (
        <div className="section-loading-state" role="status">
          <span className="spinner-icon spinner-md" aria-hidden="true" />
          <span>Loading geofences...</span>
        </div>
      )}

      {!isLoading && geofences.length === 0 && (
        <div className="geofences-empty-state">
          <div className="empty-icon-wrap">
            <GeofenceIcon size={32} />
          </div>
          <h4>No Geofences Created</h4>
          <p>
            Create circle or polygon geofences to monitor entity containment and review transition
            history.
          </p>
          <button type="button" className="btn btn-primary" onClick={onCreateClick}>
            <PlusIcon size={16} />
            <span>Create Geofence</span>
          </button>
        </div>
      )}

      {geofences.length > 0 && (
        <div className="geofences-grid">
          {geofences.map((g) => {
            const isSelected = selectedGeofenceId === g.id;
            const geom = g.geometry;
            const vertexCount = geom.type === 'Polygon' && geom.coordinates?.[0]?.length ? geom.coordinates[0].length : 0;

            return (
              <div
                key={g.id}
                className={`geofence-card ${isSelected ? 'selected' : ''} ${g.enabled ? 'is-enabled' : 'is-disabled'}`}
                data-testid={`geofence-card-${g.id}`}
                onClick={() => onSelectGeofence(g)}
              >
                <div className="geofence-card-header">
                  <div className="geofence-title-col">
                    <span className={`geofence-type-indicator type-${g.geometry.type.toLowerCase()}`}>
                      <GeofenceIcon size={16} />
                    </span>
                    <h4 className="geofence-name" title={g.name}>
                      {g.name}
                    </h4>
                  </div>

                  <div
                    className="geofence-toggle-wrap"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <label
                      className="switch-label"
                      title={g.enabled ? 'Click to disable' : 'Click to enable'}
                    >
                      <input
                        type="checkbox"
                        checked={g.enabled}
                        onChange={() => onToggleEnabled(g)}
                        aria-label={`Toggle active state for ${g.name}`}
                      />
                      <span className="switch-slider" />
                    </label>
                  </div>
                </div>

                <div className="geofence-card-meta">
                  <div className="meta-pill">
                    {geom.type === 'Circle' ? (
                      <span>Circle • {formatRadius(geom.radius_m)}</span>
                    ) : (
                      <span>Polygon • {vertexCount} vertices</span>
                    )}
                  </div>
                  <div className="meta-dates">
                    <span title={`Created: ${g.created_at}`}>
                      Updated {formatDateTime(g.updated_at)}
                    </span>
                  </div>
                </div>

                <div className="geofence-card-actions" onClick={(e) => e.stopPropagation()}>
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    onClick={() => onViewEvents(g)}
                    title="View historical transition events for this geofence"
                  >
                    <ActivityIcon size={14} />
                    <span>Events</span>
                  </button>

                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => onEditGeofence(g)}
                    title="Edit geofence name or boundary geometry"
                    aria-label={`Edit ${g.name}`}
                  >
                    <EditIcon size={14} />
                    <span>Edit</span>
                  </button>

                  <button
                    type="button"
                    className="btn btn-ghost btn-sm btn-danger-hover"
                    onClick={() => onDeleteGeofence(g)}
                    title="Delete geofence"
                    aria-label={`Delete ${g.name}`}
                  >
                    <TrashIcon size={14} />
                    <span>Delete</span>
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
