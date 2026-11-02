import { useEffect, useMemo, useRef } from 'react';
import { MapContainer, TileLayer, Marker, Popup, Tooltip, useMap, useMapEvents } from 'react-leaflet';
import L, { LatLngExpression } from 'leaflet';
import { Entity, EntityStatus, EntityType } from '../types/entity';
import { formatCoordinate } from '../utils/formatters';
import { StatusBadge } from './StatusBadge';
import { TypeBadge } from './TypeBadge';
import { flyToEntity } from '../utils/entityNavigation';
import { getMapTileConfig } from '../utils/mapConfig';

// Helper to generate inline SVG strings for divIcon
function getIconSvg(type: EntityType): string {
  switch (type) {
    case 'vehicle':
      return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 17h2c.6 0 1-.4 1-1v-3c0-.9-.7-1.7-1.5-1.9C18.7 10.6 16 10 16 10s-1.3-1.4-2.2-2.3c-.5-.4-1.1-.7-1.8-.7H5c-.6 0-1.1.4-1.4.9l-1.4 2.9A3.7 3.7 0 0 0 2 12v4c0 .6.4 1 1 1h2"/><circle cx="7" cy="17" r="2"/><path d="M9 17h6"/><circle cx="17" cy="17" r="2"/></svg>`;
    case 'iot_device':
      return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 1v3M15 1v3M9 20v3M15 20v3M20 9h3M20 14h3M1 9h3M1 14h3"/></svg>`;
    case 'facility':
      return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z"/><path d="M6 12H4a2 2 0 0 0-2 2v8h4"/><path d="M18 9h2a2 2 0 0 1 2 2v11h-4"/><path d="M10 6h4M10 10h4M10 14h4M10 18h4"/></svg>`;
    case 'other':
    default:
      return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>`;
  }
}

function createEntityDivIcon(type: EntityType, status: EntityStatus, isSelected: boolean) {
  return L.divIcon({
    className: `geo-marker-wrapper ${isSelected ? 'marker-selected' : ''}`,
    html: `
      <div class="marker-pin marker-type-${type} status-border-${status}">
        <span class="marker-icon">${getIconSvg(type)}</span>
        <span class="marker-status-dot status-dot-${status}"></span>
      </div>
      <div class="marker-tip"></div>
      ${isSelected ? '<div class="marker-pulse"></div>' : ''}
    `,
    iconSize: [36, 42],
    iconAnchor: [18, 42],
    popupAnchor: [0, -42],
    tooltipAnchor: [18, -21],
  });
}

function createDraftDivIcon() {
  return L.divIcon({
    className: 'geo-marker-wrapper marker-draft-wrapper',
    html: `
      <div class="marker-pin draft-pin">
        <span class="marker-icon">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><line x1="22" y1="12" x2="18" y2="12"/><line x1="6" y1="12" x2="2" y2="12"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="12" y1="22" x2="12" y2="18"/></svg>
        </span>
      </div>
      <div class="marker-tip draft-tip"></div>
      <div class="draft-ripple"></div>
    `,
    iconSize: [38, 44],
    iconAnchor: [19, 44],
    popupAnchor: [0, -44],
  });
}

// Controller component to smoothly manage bounds and pan/fly actions
function MapController({
  entities,
  selectedEntity,
  centerRequest,
  draftLocation,
  shouldFitBounds,
  onBoundsFitted,
}: {
  entities: Entity[];
  selectedEntity: Entity | null;
  centerRequest: number;
  draftLocation: { lat: number; lng: number } | null;
  shouldFitBounds: boolean;
  onBoundsFitted: () => void;
}) {
  const map = useMap();
  const prevSelectedIdRef = useRef<string | null>(null);
  const handledCenterRequestRef = useRef(0);

  // Fit bounds when requested (initial load or manual trigger)
  useEffect(() => {
    if (shouldFitBounds && entities.length > 0) {
      const validPoints = entities
        .filter((e) => Number.isFinite(e.latitude) && Number.isFinite(e.longitude))
        .map((e) => [e.latitude, e.longitude] as [number, number]);

      if (validPoints.length === 1) {
        map.flyTo(validPoints[0], 12, { duration: 1 });
      } else if (validPoints.length > 1) {
        const bounds = L.latLngBounds(validPoints);
        map.fitBounds(bounds, { padding: [50, 50], maxZoom: 14 });
      }
      onBoundsFitted();
    }
  }, [shouldFitBounds, entities, map, onBoundsFitted]);

  // Fly to selected entity when selection changes
  useEffect(() => {
    if (selectedEntity && selectedEntity.id !== prevSelectedIdRef.current) {
      prevSelectedIdRef.current = selectedEntity.id;
      flyToEntity(map, selectedEntity);
    } else if (!selectedEntity) {
      prevSelectedIdRef.current = null;
    }
  }, [selectedEntity, map]);

  // Explicit Center actions must work even when the selected ID is unchanged.
  useEffect(() => {
    if (!centerRequest || centerRequest === handledCenterRequestRef.current || !selectedEntity) return;
    handledCenterRequestRef.current = centerRequest;
    flyToEntity(map, selectedEntity);
  }, [centerRequest, selectedEntity, map]);

  // Pan to draft location when placed
  useEffect(() => {
    if (draftLocation && Number.isFinite(draftLocation.lat) && Number.isFinite(draftLocation.lng)) {
      map.panTo([draftLocation.lat, draftLocation.lng]);
    }
  }, [draftLocation, map]);

  return null;
}

// Map event listener for clicks
function MapEvents({
  isSelectingLocation,
  onMapClick,
}: {
  isSelectingLocation: boolean;
  onMapClick: (lat: number, lng: number) => void;
}) {
  useMapEvents({
    click(e) {
      if (isSelectingLocation) {
        onMapClick(e.latlng.lat, e.latlng.lng);
      }
    },
  });
  return null;
}

interface MapViewProps {
  entities: Entity[];
  selectedEntity: Entity | null;
  centerRequest: number;
  draftLocation: { lat: number; lng: number } | null;
  isSelectingLocation: boolean;
  onSelectEntity: (entity: Entity) => void;
  onLocationSelected: (lat: number, lng: number) => void;
  onDraftDragEnd?: (lat: number, lng: number) => void;
  shouldFitBounds: boolean;
  onBoundsFitted: () => void;
  onViewDetailsClick?: (entity: Entity) => void;
  onEditClick?: (entity: Entity) => void;
  tileUrl?: string;
  tileAttribution?: string;
  maxZoom?: number;
}

export function MapView({
  entities,
  selectedEntity,
  centerRequest,
  draftLocation,
  isSelectingLocation,
  onSelectEntity,
  onLocationSelected,
  onDraftDragEnd,
  shouldFitBounds,
  onBoundsFitted,
  onViewDetailsClick,
  onEditClick,
  tileUrl: propTileUrl,
  tileAttribution: propTileAttribution,
  maxZoom: propMaxZoom,
}: MapViewProps) {
  // Default map center (Jakarta coordinates from MVP examples or world view)
  const defaultCenter: LatLngExpression = useMemo(() => [-6.2088, 106.8456], []);
  const defaultZoom = 5;

  const tileConfig = useMemo(() => {
    const base = getMapTileConfig();
    return {
      url: propTileUrl?.trim() || base.url,
      attribution: propTileAttribution?.trim() || base.attribution,
      maxZoom: propMaxZoom ?? base.maxZoom,
    };
  }, [propTileUrl, propTileAttribution, propMaxZoom]);

  return (
    <div className={`map-wrapper ${isSelectingLocation ? 'picking-location' : ''}`}>
      {isSelectingLocation && (
        <div className="map-picking-banner animate-fade-in">
          <div className="banner-content">
            <span className="banner-pulse-dot" />
            <span className="banner-text">
              <strong>Location Picker Active:</strong> Click anywhere on the map to set coordinates.
            </span>
          </div>
        </div>
      )}

      <MapContainer
        center={defaultCenter}
        zoom={defaultZoom}
        scrollWheelZoom={true}
        className="leaflet-map-container"
      >
        <TileLayer
          attribution={tileConfig.attribution}
          url={tileConfig.url}
          maxZoom={tileConfig.maxZoom}
        />

        <MapController
          entities={entities}
          selectedEntity={selectedEntity}
          centerRequest={centerRequest}
          draftLocation={draftLocation}
          shouldFitBounds={shouldFitBounds}
          onBoundsFitted={onBoundsFitted}
        />

        <MapEvents
          isSelectingLocation={isSelectingLocation}
          onMapClick={onLocationSelected}
        />

        {/* Saved entity markers */}
        {entities.map((entity) => {
          const isSelected = selectedEntity?.id === entity.id;
          const position: LatLngExpression = [entity.latitude, entity.longitude];
          const icon = createEntityDivIcon(entity.type, entity.status, isSelected);

          return (
            <Marker
              key={entity.id}
              position={position}
              icon={icon}
              eventHandlers={{
                click: () => onSelectEntity(entity),
              }}
              zIndexOffset={isSelected ? 1000 : 100}
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

        {/* Draft/Placement marker for Create or Update flow */}
        {draftLocation && (
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
        )}
      </MapContainer>
    </div>
  );
}
