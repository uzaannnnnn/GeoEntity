import { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import L from 'leaflet';
import { Entity } from '../../types/entity';
import { UserLocation } from '../../utils/geolocation';
import { getValidEntityPoints, calculateFitTarget } from '../../utils/mapBounds';
import { flyToEntity } from '../../utils/entityNavigation';
import { subscribeToMapCenter } from '../../utils/mapCenter';
import { canZoomToShowMarker } from '../../utils/mapMarkerReveal';

interface MapControllerProps {
  entities: Entity[];
  selectedEntity: Entity | null;
  centerRequest: number;
  fitAllRequest: number;
  shouldFitBounds: boolean;
  onBoundsFitted: () => void;
  draftLocation: { lat: number; lng: number } | null;
  userLocationFlyRequest: { location: UserLocation; timestamp: number } | null;
  clusterGroupRef?: React.RefObject<L.MarkerClusterGroup | null>;
  markerMapRef?: React.MutableRefObject<Map<string, L.Marker>>;
  onCenterChange?: (lat: number, lng: number) => void;
  maxZoom?: number;
}

export function MapController({
  entities,
  selectedEntity,
  centerRequest,
  fitAllRequest,
  shouldFitBounds,
  onBoundsFitted,
  draftLocation,
  userLocationFlyRequest,
  clusterGroupRef,
  markerMapRef,
  onCenterChange,
  maxZoom = 19,
}: MapControllerProps) {
  const map = useMap();
  const prevSelectedIdRef = useRef<string | null>(null);
  const handledCenterRequestRef = useRef(0);
  const handledFitAllRequestRef = useRef(0);
  const handledUserFlyRequestRef = useRef(0);
  const onCenterChangeRef = useRef(onCenterChange);
  onCenterChangeRef.current = onCenterChange;

  // Keep one map listener for this map instance and report its initial center once.
  useEffect(() => {
    return subscribeToMapCenter(map, onCenterChangeRef);
  }, [map]);

  // Fit bounds helper for valid entity points
  const executeFit = (fitMaxZoom = maxZoom) => {
    const validPoints = getValidEntityPoints(entities);
    const target = calculateFitTarget(validPoints, fitMaxZoom);

    if (target.type === 'center') {
      map.flyTo(target.center, target.zoom, { duration: 1 });
    } else if (target.type === 'bounds') {
      const bounds = L.latLngBounds(target.bounds);
      map.fitBounds(bounds, { padding: target.padding, maxZoom: target.maxZoom });
    }
  };

  // Initial load or manual refresh fit bounds
  useEffect(() => {
    if (shouldFitBounds && entities.length > 0) {
      executeFit(maxZoom);
      onBoundsFitted();
    }
  }, [shouldFitBounds, entities, map, onBoundsFitted, maxZoom]);

  // Explicit Fit All action triggered from toolbar
  useEffect(() => {
    if (fitAllRequest && fitAllRequest !== handledFitAllRequestRef.current) {
      handledFitAllRequestRef.current = fitAllRequest;
      executeFit(maxZoom);
    }
  }, [fitAllRequest, entities, map, maxZoom]);

  // Fly to selected entity when selection changes or center is explicitly requested
  const revealAndFlyToSelected = (entity: Entity) => {
    const marker = markerMapRef?.current.get(entity.id);
    const clusterGroup = clusterGroupRef?.current;

    if (marker && clusterGroup && canZoomToShowMarker(clusterGroup, marker)) {
      clusterGroup.zoomToShowLayer(marker, () => {
        flyToEntity(map, entity);
      });
    } else {
      flyToEntity(map, entity);
    }
  };

  // Selection change
  useEffect(() => {
    if (selectedEntity && selectedEntity.id !== prevSelectedIdRef.current) {
      prevSelectedIdRef.current = selectedEntity.id;
      revealAndFlyToSelected(selectedEntity);
    } else if (!selectedEntity) {
      prevSelectedIdRef.current = null;
    }
  }, [selectedEntity, map]);

  // Center Request (even when selected ID is unchanged)
  useEffect(() => {
    if (!centerRequest || centerRequest === handledCenterRequestRef.current || !selectedEntity) {
      return;
    }
    handledCenterRequestRef.current = centerRequest;
    revealAndFlyToSelected(selectedEntity);
  }, [centerRequest, selectedEntity, map]);

  // Fly to user location upon explicit Locate Me success
  useEffect(() => {
    if (
      userLocationFlyRequest &&
      userLocationFlyRequest.timestamp !== handledUserFlyRequestRef.current
    ) {
      handledUserFlyRequestRef.current = userLocationFlyRequest.timestamp;
      const { latitude, longitude } = userLocationFlyRequest.location;
      const targetZoom = Math.min(15, maxZoom);
      map.flyTo([latitude, longitude], targetZoom, { duration: 1.2 });
    }
  }, [userLocationFlyRequest, map, maxZoom]);

  // Pan to draft location when placed
  useEffect(() => {
    if (draftLocation && Number.isFinite(draftLocation.lat) && Number.isFinite(draftLocation.lng)) {
      map.panTo([draftLocation.lat, draftLocation.lng]);
    }
  }, [draftLocation, map]);

  return null;
}
