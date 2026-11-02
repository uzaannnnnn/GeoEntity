import { RefObject, MutableRefObject } from 'react';
import { Marker, Popup, Tooltip } from 'react-leaflet';
import L, { LatLngExpression } from 'leaflet';
import MarkerClusterGroup from 'react-leaflet-cluster';
import { Entity } from '../../types/entity';
import { formatCoordinate } from '../../utils/formatters';
import { StatusBadge } from '../StatusBadge';
import { TypeBadge } from '../TypeBadge';
import { createEntityDivIcon, createClusterDivIcon } from './markerIcons';

interface EntityMarkerLayerProps {
  entities: Entity[];
  selectedEntity: Entity | null;
  onSelectEntity: (entity: Entity) => void;
  onViewDetailsClick?: (entity: Entity) => void;
  onEditClick?: (entity: Entity) => void;
  clusterGroupRef?: RefObject<L.MarkerClusterGroup | null>;
  markerMapRef?: MutableRefObject<Map<string, L.Marker>>;
}

export function EntityMarkerLayer({
  entities,
  selectedEntity,
  onSelectEntity,
  onViewDetailsClick,
  onEditClick,
  clusterGroupRef,
  markerMapRef,
}: EntityMarkerLayerProps) {
  return (
    <MarkerClusterGroup
      ref={clusterGroupRef as unknown as React.Ref<L.MarkerClusterGroup>}
      chunkedLoading={true}
      showCoverageOnHover={false}
      spiderfyOnMaxZoom={true}
      zoomToBoundsOnClick={true}
      maxClusterRadius={45}
      iconCreateFunction={createClusterDivIcon}
    >
      {entities.map((entity) => {
        const isSelected = selectedEntity?.id === entity.id;
        const position: LatLngExpression = [entity.latitude, entity.longitude];
        const icon = createEntityDivIcon(entity.type, entity.status, isSelected);

        return (
          <Marker
            key={entity.id}
            position={position}
            icon={icon}
            zIndexOffset={isSelected ? 1000 : 100}
            ref={(markerInstance) => {
              if (markerMapRef) {
                if (markerInstance) {
                  markerMapRef.current.set(entity.id, markerInstance);
                } else {
                  markerMapRef.current.delete(entity.id);
                }
              }
            }}
            eventHandlers={{
              click: () => onSelectEntity(entity),
            }}
          >
            <Tooltip direction="top" offset={[0, -36]} opacity={0.95}>
              <div className="marker-tooltip">
                <strong>{entity.name}</strong>
                <span className="tooltip-sub">
                  {entity.type} &bull; {entity.status}
                </span>
              </div>
            </Tooltip>

            <Popup className="geo-popup" offset={[0, -34]}>
              <div className="popup-card">
                <div className="popup-header">
                  <h4 className="popup-title">{entity.name}</h4>
                </div>
                <div className="popup-badges">
                  <TypeBadge type={entity.type} />
                  <StatusBadge status={entity.status} />
                </div>
                {entity.description && (
                  <p className="popup-desc">{entity.description}</p>
                )}
                <div className="popup-coords">
                  <span>Lat: {formatCoordinate(entity.latitude)}</span>
                  <span>Lng: {formatCoordinate(entity.longitude)}</span>
                </div>
                <div className="popup-actions">
                  {onViewDetailsClick && (
                    <button
                      type="button"
                      className="btn btn-xs btn-primary"
                      onClick={() => onViewDetailsClick(entity)}
                    >
                      View Details
                    </button>
                  )}
                  {onEditClick && (
                    <button
                      type="button"
                      className="btn btn-xs btn-secondary"
                      onClick={() => onEditClick(entity)}
                    >
                      Edit
                    </button>
                  )}
                </div>
              </div>
            </Popup>
          </Marker>
        );
      })}
    </MarkerClusterGroup>
  );
}
