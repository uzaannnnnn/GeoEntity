import { Circle, Polygon, Popup, Tooltip } from 'react-leaflet';
import { GeofenceDefinition, GeofencePosition } from '../../types/geofence';
import { formatRadius } from '../../utils/formatters';
import { GeofenceIcon, ActivityIcon } from '../Icons';

export { formatRadius };

export interface GeofenceOverlayLayerProps {
  geofences: GeofenceDefinition[];
  selectedGeofenceId?: string | null;
  onSelectGeofence?: (geofence: GeofenceDefinition) => void;
  onViewEvents?: (geofence: GeofenceDefinition) => void;
}

export function GeofenceOverlayLayer({
  geofences,
  selectedGeofenceId,
  onSelectGeofence,
  onViewEvents,
}: GeofenceOverlayLayerProps) {
  if (!geofences || geofences.length === 0) {
    return null;
  }

  return (
    <>
      {geofences.map((g) => {
        const isSelected = selectedGeofenceId === g.id;
        const isCircle = g.geometry.type === 'Circle';

        // Path styling rules:
        // - Enabled circle: Blue (#2563eb)
        // - Enabled polygon: Purple (#7c3aed)
        // - Disabled zone: Muted slate (#94a3b8) with dashed stroke
        // - Selected zone: Thicker stroke, deeper fill opacity
        const primaryColor = isCircle ? '#2563eb' : '#7c3aed';
        const color = g.enabled ? primaryColor : '#94a3b8';
        const fillColor = g.enabled ? primaryColor : '#cbd5e1';
        const fillOpacity = isSelected ? 0.35 : g.enabled ? 0.15 : 0.08;
        const weight = isSelected ? 4 : g.enabled ? 2 : 1.5;
        const dashArray = g.enabled ? undefined : '6, 6';

        const pathOptions = {
          color,
          fillColor,
          fillOpacity,
          weight,
          dashArray,
        };

        const key = `geofence-${g.id}-${isSelected ? 'sel' : 'unsel'}`;

        const popupContent = (
          <div className="geofence-map-popup">
            <div className="popup-header-row">
              <span className="popup-icon">
                <GeofenceIcon size={16} />
              </span>
              <strong className="popup-title">{g.name}</strong>
            </div>

            <div className="popup-meta-badges">
              <span className={`geofence-type-badge badge-${g.geometry.type.toLowerCase()}`}>
                {g.geometry.type === 'Circle'
                  ? `Circle • ${formatRadius(g.geometry.radius_m)}`
                  : `Polygon • ${g.geometry.coordinates[0]?.length || 0} vertices`}
              </span>
              <span className={`geofence-status-pill ${g.enabled ? 'status-enabled' : 'status-disabled'}`}>
                {g.enabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>

            <div className="popup-actions-row">
              {onViewEvents && (
                <button
                  type="button"
                  className="btn btn-sm btn-secondary popup-btn"
                  onClick={() => onViewEvents(g)}
                  title="View transition event history for this geofence"
                >
                  <ActivityIcon size={14} />
                  <span>View Events</span>
                </button>
              )}
            </div>
          </div>
        );

        const geom = g.geometry;

        if (geom.type === 'Circle' && geom.center) {
          // Leaflet expects [latitude, longitude], backend provides [longitude, latitude]
          const latLngCenter: [number, number] = [geom.center[1], geom.center[0]];

          return (
            <Circle
              key={key}
              center={latLngCenter}
              radius={geom.radius_m}
              pathOptions={pathOptions}
              eventHandlers={{
                click: () => {
                  onSelectGeofence?.(g);
                },
              }}
            >
              <Tooltip direction="top" offset={[0, -10]} opacity={0.9}>
                <span>
                  {g.name} ({g.enabled ? 'Active' : 'Disabled'})
                </span>
              </Tooltip>
              <Popup>{popupContent}</Popup>
            </Circle>
          );
        }

        if (geom.type === 'Polygon' && geom.coordinates && geom.coordinates[0]) {
          // Leaflet expects [[lat, lon], ...], backend provides [[lon, lat], ...]
          const latLngPositions: [number, number][] = geom.coordinates[0].map(
            ([lon, lat]: GeofencePosition) => [lat, lon]
          );

          return (
            <Polygon
              key={key}
              positions={latLngPositions}
              pathOptions={pathOptions}
              eventHandlers={{
                click: () => {
                  onSelectGeofence?.(g);
                },
              }}
            >
              <Tooltip direction="top" opacity={0.9}>
                <span>
                  {g.name} ({g.enabled ? 'Active' : 'Disabled'})
                </span>
              </Tooltip>
              <Popup>{popupContent}</Popup>
            </Polygon>
          );
        }

        return null;
      })}
    </>
  );
}
