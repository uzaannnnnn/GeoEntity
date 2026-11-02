import { Circle, Marker, Tooltip } from 'react-leaflet';
import { UserLocation } from '../../utils/geolocation';
import { formatCoordinate } from '../../utils/formatters';
import { createUserLocationDivIcon } from './markerIcons';

interface UserLocationLayerProps {
  location: UserLocation | null;
}

export function UserLocationLayer({ location }: UserLocationLayerProps) {
  if (!location || !Number.isFinite(location.latitude) || !Number.isFinite(location.longitude)) {
    return null;
  }

  const hasAccuracy = Number.isFinite(location.accuracy) && location.accuracy > 0;

  return (
    <>
      {hasAccuracy && (
        <Circle
          center={[location.latitude, location.longitude]}
          radius={location.accuracy}
          pathOptions={{
            color: '#0284c7',
            fillColor: '#38bdf8',
            fillOpacity: 0.15,
            weight: 1.5,
            dashArray: '4, 4',
          }}
        />
      )}

      <Marker
        position={[location.latitude, location.longitude]}
        icon={createUserLocationDivIcon()}
        zIndexOffset={1500}
      >
        <Tooltip direction="top" offset={[0, -18]} opacity={0.95}>
          <div className="marker-tooltip user-location-tooltip">
            <strong>Your Current Location</strong>
            <span>
              {formatCoordinate(location.latitude)}, {formatCoordinate(location.longitude)}
            </span>
            {hasAccuracy && (
              <span className="tooltip-sub">~{Math.round(location.accuracy)}m accuracy</span>
            )}
          </div>
        </Tooltip>
      </Marker>
    </>
  );
}
