import L from 'leaflet';

/** zoomToShowLayer requires the marker to have been registered in the cluster group. */
export function canZoomToShowMarker(
  clusterGroup: L.MarkerClusterGroup | null | undefined,
  marker: L.Marker | null | undefined
): boolean {
  return Boolean(
    clusterGroup &&
    marker &&
    typeof clusterGroup.zoomToShowLayer === 'function' &&
    clusterGroup.hasLayer(marker)
  );
}
