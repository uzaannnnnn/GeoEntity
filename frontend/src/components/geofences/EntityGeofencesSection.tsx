import { useState, useEffect, useCallback, useRef } from 'react';
import { Entity } from '../../types/entity';
import { EntityGeofenceMembership } from '../../types/geofence';
import { geofencesApi } from '../../api/geofences';
import { ApiError } from '../../api/client';
import { formatCoordinate, formatDateTime } from '../../utils/formatters';
import { GeofenceIcon, CheckIcon, AlertIcon, RefreshIcon, ActivityIcon, InfoIcon } from '../Icons';

interface EntityGeofencesSectionProps {
  entity: Entity;
  onViewGeofenceEvents?: (geofenceId: string, geofenceName: string) => void;
  onSelectGeofenceOnMap?: (geofenceId: string) => void;
}

export function EntityGeofencesSection({
  entity,
  onViewGeofenceEvents,
  onSelectGeofenceOnMap,
}: EntityGeofencesSectionProps) {
  const [memberships, setMemberships] = useState<EntityGeofenceMembership[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Stale request guard
  const activeEntityIdRef = useRef(entity.id);
  activeEntityIdRef.current = entity.id;

  const fetchMemberships = useCallback(async () => {
    const currentId = entity.id;
    setIsLoading(true);
    setError(null);

    try {
      const data = await geofencesApi.getEntityGeofences(currentId);
      // Guard against stale response if user switched entities
      if (activeEntityIdRef.current === currentId) {
        setMemberships(data);
      }
    } catch (err) {
      if (activeEntityIdRef.current === currentId) {
        if (err instanceof ApiError) {
          setError(err.message);
        } else {
          setError('Failed to load geofence memberships. Please check connection.');
        }
      }
    } finally {
      if (activeEntityIdRef.current === currentId) {
        setIsLoading(false);
      }
    }
  }, [entity.id]);

  useEffect(() => {
    fetchMemberships();
  }, [fetchMemberships]);

  const insideCount = memberships.filter((m) => m.state === 'inside').length;

  return (
    <div className="entity-geofences-section">
      <div className="section-header-row">
        <div>
          <h3 className="section-title">Geofence Memberships</h3>
          <p className="section-subtitle">
            Authoritative spatial containment determined by backend geometry engine on coordinate updates.
          </p>
        </div>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={fetchMemberships}
          disabled={isLoading}
          title="Refresh geofence memberships"
          aria-label="Refresh geofence memberships"
        >
          <RefreshIcon size={14} className={isLoading ? 'animate-spin' : ''} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Semantic notice */}
      <div className="semantic-notice-box">
        <InfoIcon size={16} className="text-muted" />
        <span className="info-text">
          Containment is derived from persisted entity coordinates (
          <code>{formatCoordinate(entity.latitude)}&deg;, {formatCoordinate(entity.longitude)}&deg;</code>
          ). Transitions are emitted only on coordinate changes.
        </span>
      </div>

      {isLoading && (
        <div className="section-loading-state" role="status">
          <span className="spinner-icon spinner-md" aria-hidden="true" />
          <span>Evaluating current geofence containment...</span>
        </div>
      )}

      {error && !isLoading && (
        <div className="alert-box alert-error" role="alert">
          <AlertIcon size={16} />
          <div className="alert-content">
            <strong>Error:</strong> {error}
          </div>
          <button type="button" className="btn btn-sm btn-secondary" onClick={fetchMemberships}>
            Retry
          </button>
        </div>
      )}

      {!isLoading && !error && memberships.length === 0 && (
        <div className="section-empty-state">
          <div className="empty-icon-wrap">
            <GeofenceIcon size={28} />
          </div>
          <h4>No Geofences Configured</h4>
          <p>There are currently no geofence definitions in the system.</p>
        </div>
      )}

      {!isLoading && !error && memberships.length > 0 && (
        <div className="memberships-list-wrapper">
          <div className="memberships-summary-bar">
            <span className="summary-stat">
              Currently Inside: <strong>{insideCount}</strong> of {memberships.length} active zone(s)
            </span>
          </div>

          <div className="memberships-cards-grid">
            {memberships.map((m) => {
              const isInside = m.state === 'inside';
              const isInactive = m.state === 'inactive';

              return (
                <div
                  key={m.geofence_id}
                  className={`membership-card state-${m.state}`}
                  data-testid={`membership-${m.geofence_id}`}
                >
                  <div className="membership-card-header">
                    <div className="geofence-name-group">
                      <GeofenceIcon size={18} className="geofence-card-icon" />
                      <h4 className="geofence-card-title">{m.geofence_name}</h4>
                    </div>

                    <div className="membership-state-badge">
                      {isInside && (
                        <span className="badge-inside">
                          <CheckIcon size={14} />
                          <span>Inside Zone</span>
                        </span>
                      )}
                      {!isInside && !isInactive && (
                        <span className="badge-outside">
                          <span>Outside Zone</span>
                        </span>
                      )}
                      {isInactive && (
                        <span className="badge-inactive">
                          <AlertIcon size={14} />
                          <span>Zone Disabled</span>
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="membership-card-body">
                    <div className="eval-detail-row">
                      <span className="eval-label">Evaluated Coordinates:</span>
                      <span className="eval-value">
                        {formatCoordinate(m.evaluated_latitude)}&deg;, {formatCoordinate(m.evaluated_longitude)}&deg;
                      </span>
                    </div>

                    <div className="eval-detail-row">
                      <span className="eval-label">Evaluated At:</span>
                      <span className="eval-value" title={m.evaluated_at}>
                        {formatDateTime(m.evaluated_at)}
                      </span>
                    </div>

                    <div className="eval-detail-row">
                      <span className="eval-label">Zone Updated:</span>
                      <span className="eval-value" title={m.definition_updated_at}>
                        {formatDateTime(m.definition_updated_at)}
                      </span>
                    </div>
                  </div>

                  <div className="membership-card-actions">
                    {onSelectGeofenceOnMap && (
                      <button
                        type="button"
                        className="btn btn-ghost btn-sm"
                        onClick={() => onSelectGeofenceOnMap(m.geofence_id)}
                        title="Highlight this geofence on the map"
                      >
                        <GeofenceIcon size={14} />
                        <span>View on Map</span>
                      </button>
                    )}
                    {onViewGeofenceEvents && (
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm"
                        onClick={() => onViewGeofenceEvents(m.geofence_id, m.geofence_name)}
                        title="View transition events for this geofence"
                      >
                        <ActivityIcon size={14} />
                        <span>Inspect Events</span>
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
