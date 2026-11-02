import { Marker, Tooltip } from 'react-leaflet';
import { formatCoordinate } from '../../utils/formatters';
import { createDraftDivIcon } from './markerIcons';

interface DraftLocationMarkerProps {
  draftLocation: { lat: number; lng: number } | null;
  onDraftDragEnd?: (lat: number, lng: number) => void;
}

export function DraftLocationMarker({
  draftLocation,
  onDraftDragEnd,
}: DraftLocationMarkerProps) {
  if (!draftLocation || !Number.isFinite(draftLocation.lat) || !Number.isFinite(draftLocation.lng)) {
    return null;
  }

  return (
    <Marker
      position={[draftLocation.lat, draftLocation.lng]}
      icon={createDraftDivIcon()}
      draggable={Boolean(onDraftDragEnd)}
      eventHandlers={{
        dragend: (e) => {
          if (onDraftDragEnd) {
            const marker = e.target;
            const pos = marker.getLatLng();
            onDraftDragEnd(pos.lat, pos.lng);
          }
        },
      }}
      zIndexOffset={2000}
    >
      <Tooltip permanent direction="top" offset={[0, -38]} opacity={0.95}>
        <div className="marker-tooltip draft-tooltip">
          <strong>Selected Location</strong>
          <span>
            {formatCoordinate(draftLocation.lat)}, {formatCoordinate(draftLocation.lng)}
          </span>
          <span className="tooltip-hint">(Drag to adjust)</span>
        </div>
      </Tooltip>
    </Marker>
  );
}
