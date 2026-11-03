import { useState, useMemo, useRef, useCallback } from 'react';
import { MapContainer, TileLayer, useMapEvents } from 'react-leaflet';
import L, { LatLngExpression } from 'leaflet';
import { Entity } from '../types/entity';
import { getMapTileConfig } from '../utils/mapConfig';
import { UserLocation, getCurrentUserLocation, GeolocationErrorDetails } from '../utils/geolocation';
import { getValidEntityPoints } from '../utils/mapBounds';
import { EntityMarkerLayer } from './map/EntityMarkerLayer';
import { DraftLocationMarker } from './map/DraftLocationMarker';
import { UserLocationLayer } from './map/UserLocationLayer';
import { MapController } from './map/MapController';
import { MapToolbar } from './map/MapToolbar';
import { MapLegend } from './map/MapLegend';
import { CoordinateInspector } from './map/CoordinateInspector';
import { updateMapCenter } from '../utils/mapCenter';

// Map event listener for location picking clicks
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

export interface MapViewProps {
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
  // Default map center (Jakarta coordinates from MVP baseline)
  const defaultCenterCoordinates = useMemo(() => ({ lat: -6.2088, lng: 106.8456 }), []);
  const defaultCenter: LatLngExpression = useMemo(
    () => [defaultCenterCoordinates.lat, defaultCenterCoordinates.lng],
    [defaultCenterCoordinates]
  );
  const defaultZoom = 5;

  const tileConfig = useMemo(() => {
    const base = getMapTileConfig();
    return {
      url: propTileUrl?.trim() || base.url,
      attribution: propTileAttribution?.trim() || base.attribution,
      maxZoom: propMaxZoom ?? base.maxZoom,
    };
  }, [propTileUrl, propTileAttribution, propMaxZoom]);

  // Ephemeral map-only state
  const [userLocation, setUserLocation] = useState<UserLocation | null>(null);
  const [userLocationFlyRequest, setUserLocationFlyRequest] = useState<{
    location: UserLocation;
    timestamp: number;
  } | null>(null);
  const [isLocating, setIsLocating] = useState<boolean>(false);
  const [geolocationError, setGeolocationError] = useState<GeolocationErrorDetails | null>(null);
  const [toolbarMessage, setToolbarMessage] = useState<string | null>(null);

  const [isLegendOpen, setIsLegendOpen] = useState<boolean>(false);
  const [isInspectorOpen, setIsInspectorOpen] = useState<boolean>(false);
  const [currentCenter, setCurrentCenter] = useState<{ lat: number; lng: number }>(defaultCenterCoordinates);
  const [fitAllRequest, setFitAllRequest] = useState<number>(0);

  const handleCenterChange = useCallback((lat: number, lng: number) => {
    setCurrentCenter((previous) => updateMapCenter(previous, { lat, lng }));
  }, []);

  // References for cluster group and marker instances
  const clusterGroupRef = useRef<L.MarkerClusterGroup | null>(null);
  const markerMapRef = useRef<Map<string, L.Marker>>(new Map());

  // Count valid entity points for Fit All disabled state
  const validPointsCount = useMemo(() => {
    return getValidEntityPoints(entities).length;
  }, [entities]);

  // Handle Fit All action
  const handleFitAll = useCallback(() => {
    if (validPointsCount === 0) {
      setToolbarMessage('No valid entity coordinates to fit.');
      setTimeout(() => setToolbarMessage(null), 4000);
      return;
    }
    setToolbarMessage(null);
    setFitAllRequest((prev) => prev + 1);
  }, [validPointsCount]);

  // Handle Locate Me action
  const handleLocateMe = useCallback(async () => {
    if (isLocating) return;
    setIsLocating(true);
    setGeolocationError(null);
    setToolbarMessage(null);

    try {
      const location = await getCurrentUserLocation();
      setUserLocation(location);
      setUserLocationFlyRequest({
        location,
        timestamp: Date.now(),
      });
    } catch (err) {
      setGeolocationError(err as GeolocationErrorDetails);
    } finally {
      setIsLocating(false);
    }
  }, [isLocating]);

  return (
    <div className={`map-wrapper ${isSelectingLocation ? 'picking-location' : ''}`}>
      {/* Location Picker Banner */}
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

      {/* Advanced Map Controls Toolbar (Top Right) */}
      <MapToolbar
        validPointsCount={validPointsCount}
        isLocating={isLocating}
        geolocationError={geolocationError}
        toolbarMessage={toolbarMessage}
        isLegendOpen={isLegendOpen}
        isInspectorOpen={isInspectorOpen}
        onFitAll={handleFitAll}
        onLocateMe={handleLocateMe}
        onToggleLegend={() => setIsLegendOpen((prev) => !prev)}
        onToggleInspector={() => setIsInspectorOpen((prev) => !prev)}
        onDismissMessage={() => {
          setGeolocationError(null);
          setToolbarMessage(null);
        }}
      />

      {/* Collapsible Visual Legend Panel */}
      <MapLegend
        isOpen={isLegendOpen}
        onClose={() => setIsLegendOpen(false)}
      />

      {/* Coordinate Inspector Panel */}
      <CoordinateInspector
        center={currentCenter}
        isOpen={isInspectorOpen}
        onClose={() => setIsInspectorOpen(false)}
      />

      {/* Leaflet Map Container */}
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
          fitAllRequest={fitAllRequest}
          shouldFitBounds={shouldFitBounds}
          onBoundsFitted={onBoundsFitted}
          draftLocation={draftLocation}
          userLocationFlyRequest={userLocationFlyRequest}
          clusterGroupRef={clusterGroupRef}
          markerMapRef={markerMapRef}
          onCenterChange={handleCenterChange}
          maxZoom={tileConfig.maxZoom}
        />

        <MapEvents
          isSelectingLocation={isSelectingLocation}
          onMapClick={onLocationSelected}
        />

        {/* Feature A: Clustered Persisted Entity Markers */}
        <EntityMarkerLayer
          entities={entities}
          selectedEntity={selectedEntity}
          onSelectEntity={onSelectEntity}
          onViewDetailsClick={onViewDetailsClick}
          onEditClick={onEditClick}
          clusterGroupRef={clusterGroupRef}
          markerMapRef={markerMapRef}
        />

        {/* Feature B: Transient Current User Location Layer (Outside cluster) */}
        <UserLocationLayer location={userLocation} />

        {/* Draft/Placement marker for Create or Update flow (Outside cluster) */}
        <DraftLocationMarker
          draftLocation={draftLocation}
          onDraftDragEnd={onDraftDragEnd}
        />
      </MapContainer>
    </div>
  );
}
