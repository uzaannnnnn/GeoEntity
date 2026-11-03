import L from 'leaflet';

export interface MapCenter {
  lat: number;
  lng: number;
}

export const MAP_CENTER_EPSILON = 1e-6;

/** Return the existing state object when map movement is below the useful precision. */
export function updateMapCenter(
  previous: MapCenter,
  next: MapCenter,
  epsilon = MAP_CENTER_EPSILON
): MapCenter {
  if (
    Math.abs(previous.lat - next.lat) <= epsilon &&
    Math.abs(previous.lng - next.lng) <= epsilon
  ) {
    return previous;
  }
  return next;
}

export type MapCenterCallback = (lat: number, lng: number) => void;

/** Subscribe once to map center changes while always calling the latest React callback. */
export function subscribeToMapCenter(
  map: L.Map,
  callbackRef: { current: MapCenterCallback | undefined }
): () => void {
  const reportCenter = () => {
    const center = map.getCenter();
    callbackRef.current?.(center.lat, center.lng);
  };

  map.on('moveend', reportCenter);
  reportCenter();

  return () => {
    map.off('moveend', reportCenter);
  };
}
