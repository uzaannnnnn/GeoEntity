import { useState, useMemo, useRef, useCallback, useEffect } from 'react';
import { MapContainer, TileLayer, useMap, useMapEvents } from 'react-leaflet';
import L, { LatLngExpression } from 'leaflet';
import { Entity, MapBoundsParams } from '../types/entity';
import { getMapTileConfig } from '../utils/mapConfig';
import { UserLocation, getCurrentUserLocation, GeolocationErrorDetails } from '../utils/geolocation';
import { FitTarget } from '../utils/mapBounds';
import { normalizeLeafletBounds, areBoundsEqual } from '../utils/mapViewport';
import { EntityMarkerLayer, AnyMarkerEntity } from './map/EntityMarkerLayer';
import { DraftLocationMarker } from './map/DraftLocationMarker';
import { UserLocationLayer } from './map/UserLocationLayer';
import { GeofenceOverlayLayer } from './map/GeofenceOverlayLayer';
import { GeofenceDefinition } from '../types/geofence';
import { MapController } from './map/MapController';
import { MapToolbar } from './map/MapToolbar';
import { MapLegend } from './map/MapLegend';
import { CoordinateInspector } from './map/CoordinateInspector';
import { AlertIcon } from './Icons';
import { updateMapCenter } from '../utils/mapCenter';
import { createCoalescedFrameTask } from '../utils/coalescedFrameTask';

function MapSizeObserver() {
  const map = useMap();

  useEffect(() => {
    const resizeTask = createCoalescedFrameTask(() => {
      map.invalidateSize({ pan: false, debounceMoveend: true });
    });
    const container = map.getContainer();
    const observer = typeof ResizeObserver === 'undefined'
      ? null
      : new ResizeObserver(() => resizeTask.request());
    observer?.observe(container);
    window.addEventListener('resize', resizeTask.request);
    resizeTask.request();

    return () => {
      observer?.disconnect();
      window.removeEventListener('resize', resizeTask.request);
      resizeTask.cancel();
    };
  }, [map]);

  return null;
}

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

// Map event listener for debounced viewport bounds reporting
function MapViewportListener({
  onBoundsChange,
}: {
  onBoundsChange?: (bounds: MapBoundsParams) => void;
}) {
  const onBoundsChangeRef = useRef(onBoundsChange);
  onBoundsChangeRef.current = onBoundsChange;
  const prevBoundsRef = useRef<MapBoundsParams | null>(null);

  const notifyBounds = useCallback((mapInstance: L.Map) => {
    if (!onBoundsChangeRef.current) return;
    const b = mapInstance.getBounds();
    const normalized = normalizeLeafletBounds(b);
    if (!areBoundsEqual(prevBoundsRef.current, normalized)) {
      prevBoundsRef.current = normalized;
      onBoundsChangeRef.current(normalized);
    }
  }, []);

  const map = useMapEvents({
    moveend() {
      notifyBounds(map);
    },
    zoomend() {
      notifyBounds(map);
    },
  });

  useEffect(() => {
    notifyBounds(map);
  }, [map, notifyBounds]);

  return null;
}

export interface MapViewProps {
  mapMarkers: AnyMarkerEntity[];
  selectedEntity: Entity | null;
  centerRequest: number;
  draftLocation: { lat: number; lng: number } | null;
  isSelectingLocation: boolean;
  onSelectEntity: (entity: AnyMarkerEntity) => void;
  onLocationSelected: (lat: number, lng: number) => void;
  onDraftDragEnd?: (lat: number, lng: number) => void;
  onBoundsChange?: (bounds: MapBoundsParams) => void;
  onFitAllClick?: () => void;
  matchingCount?: number;
  isMarkersLoading?: boolean;
  mapLimitExceeded?: boolean;
  mapError?: string | null;
  fitTargetRequest?: { target: FitTarget; timestamp: number } | null;
  onViewDetailsClick?: (entity: AnyMarkerEntity) => void;
  onEditClick?: (entity: AnyMarkerEntity) => void;
  geofences?: GeofenceDefinition[];
  selectedGeofenceId?: string | null;
  onSelectGeofence?: (geofence: GeofenceDefinition) => void;
  onViewGeofenceEvents?: (geofence: GeofenceDefinition) => void;
  tileUrl?: string;
  tileAttribution?: string;
  maxZoom?: number;
}

export function MapView({
  mapMarkers,
  selectedEntity,
  centerRequest,
  draftLocation,
  isSelectingLocation,
  onSelectEntity,
  onLocationSelected,
  onDraftDragEnd,
  onBoundsChange,
  onFitAllClick,
  matchingCount = 0,
  isMarkersLoading = false,
  mapLimitExceeded = false,
  mapError,
  fitTargetRequest,
  onViewDetailsClick,
  onEditClick,
  geofences = [],
  selectedGeofenceId,
  onSelectGeofence,
  onViewGeofenceEvents,
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

  const handleCenterChange = useCallback((lat: number, lng: number) => {
    setCurrentCenter((previous) => updateMapCenter(previous, { lat, lng }));
  }, []);

  // References for cluster group and marker instances
  const clusterGroupRef = useRef<L.MarkerClusterGroup | null>(null);
  const markerMapRef = useRef<Map<string, L.Marker>>(new Map());

  // Handle Fit All action
  const handleFitAll = useCallback(() => {
    if (matchingCount === 0) {
      setToolbarMessage('No matching entities to fit.');
      setTimeout(() => setToolbarMessage(null), 4000);
      return;
    }
    setToolbarMessage(null);
    onFitAllClick?.();
  }, [matchingCount, onFitAllClick]);

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
      <div className="map-top-controls">
        {/* Location Picker Banner */}
        {isSelectingLocation && (
          <div className="map-picking-banner animate-fade-in" role="status">
            <div className="banner-content">
              <span className="banner-pulse-dot" aria-hidden="true" />
              <span className="banner-text">
                <strong>Location Picker Active:</strong> Click anywhere on the map to set coordinates.
              </span>
            </div>
          </div>
        )}

        {/* Advanced Map Controls Toolbar */}
        <MapToolbar
          validPointsCount={matchingCount}
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
      </div>

      {/* HTTP 413 Map Result Limit Warning Banner */}
      {mapLimitExceeded && (
        <div className="map-limit-banner animate-fade-in" role="alert">
          <AlertIcon size={16} className="text-warning" />
          <span className="banner-text">
            <strong>Area contains too many entities (&gt; 5,000):</strong> Please zoom in or apply
            filters to view map markers.
          </span>
        </div>
      )}

      {/* Map Error Banner */}
      {mapError && !mapLimitExceeded && (
        <div className="map-limit-banner map-error-banner animate-fade-in" role="alert">
          <AlertIcon size={16} className="text-danger" />
          <span className="banner-text">{mapError}</span>
        </div>
      )}

      {/* Loading indicator pill */}
      {isMarkersLoading && (
        <div className="map-loading-indicator animate-fade-in" role="status">
          <span className="spinner-icon spinner-sm" aria-hidden="true" />
          <span>Updating markers...</span>
        </div>
      )}

      {/* Collapsible Visual Legend Panel */}
      <MapLegend isOpen={isLegendOpen} onClose={() => setIsLegendOpen(false)} />

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
        <MapSizeObserver />
        <TileLayer
          attribution={tileConfig.attribution}
          url={tileConfig.url}
          maxZoom={tileConfig.maxZoom}
        />

        <MapController
          entities={mapMarkers}
          selectedEntity={selectedEntity}
          centerRequest={centerRequest}
          fitTargetRequest={fitTargetRequest}
          draftLocation={draftLocation}
          userLocationFlyRequest={userLocationFlyRequest}
          clusterGroupRef={clusterGroupRef}
          markerMapRef={markerMapRef}
          onCenterChange={handleCenterChange}
          maxZoom={tileConfig.maxZoom}
        />

        <MapEvents isSelectingLocation={isSelectingLocation} onMapClick={onLocationSelected} />

        <MapViewportListener onBoundsChange={onBoundsChange} />

        {/* Clustered Persisted Entity Markers from separate Map API */}
        <EntityMarkerLayer
          entities={mapMarkers}
          selectedEntity={selectedEntity}
          onSelectEntity={onSelectEntity}
          onViewDetailsClick={onViewDetailsClick}
          onEditClick={onEditClick}
          clusterGroupRef={clusterGroupRef}
          markerMapRef={markerMapRef}
        />

        {/* Transient Current User Location Layer (Outside cluster) */}
        <UserLocationLayer location={userLocation} />

        {/* Draft/Placement marker for Create or Update flow (Outside cluster) */}
        <DraftLocationMarker draftLocation={draftLocation} onDraftDragEnd={onDraftDragEnd} />

        {/* Authoritative Geofence Spatial Overlay Layer (Circles & Polygons) */}
        <GeofenceOverlayLayer
          geofences={geofences}
          selectedGeofenceId={selectedGeofenceId}
          onSelectGeofence={onSelectGeofence}
          onViewEvents={onViewGeofenceEvents}
        />
      </MapContainer>
    </div>
  );
}
