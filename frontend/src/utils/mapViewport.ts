import { MapBoundsParams, MapExtentData } from '../types/entity';
import { FitTarget } from './mapBounds';

/**
 * Normalizes a longitude coordinate into the standard [-180, 180] range.
 */
export function normalizeLongitude(lng: number): number {
  if (!Number.isFinite(lng)) return 0;
  if (lng >= -180 && lng <= 180) return Number(lng.toFixed(6));
  // Wrap to [-180, 180]
  const wrapped = ((((lng + 180) % 360) + 360) % 360) - 180;
  return Number(wrapped.toFixed(6));
}

/**
 * Normalizes a latitude coordinate into the standard [-90, 90] range.
 */
export function clampLatitude(lat: number): number {
  if (!Number.isFinite(lat)) return 0;
  return Number(Math.max(-90, Math.min(90, lat)).toFixed(6));
}

export interface MinimalLeafletBounds {
  getWest(): number;
  getSouth(): number;
  getEast(): number;
  getNorth(): number;
}

/**
 * Converts Leaflet LatLngBounds into standard MapBoundsParams compliant with the backend.
 * Handles antimeridian wrapping and global zoom-out (> 360 deg span).
 */
export function normalizeLeafletBounds(bounds: MinimalLeafletBounds): MapBoundsParams {
  const rawWest = bounds.getWest();
  const rawEast = bounds.getEast();
  let south = clampLatitude(bounds.getSouth());
  let north = clampLatitude(bounds.getNorth());

  if (south > north) {
    const temp = south;
    south = north;
    north = temp;
  }

  // If the view spans 360 degrees or more (e.g. at low zoom levels), span the full globe
  if (rawEast - rawWest >= 360) {
    return {
      west: -180,
      south,
      east: 180,
      north,
    };
  }

  const west = normalizeLongitude(rawWest);
  const east = normalizeLongitude(rawEast);

  return {
    west,
    south,
    east,
    north,
  };
}

/**
 * Checks whether two MapBoundsParams are meaningfully different beyond a delta threshold.
 */
export function areBoundsEqual(
  a: MapBoundsParams | null,
  b: MapBoundsParams | null,
  threshold = 1e-4
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  return (
    Math.abs(a.west - b.west) < threshold &&
    Math.abs(a.south - b.south) < threshold &&
    Math.abs(a.east - b.east) < threshold &&
    Math.abs(a.north - b.north) < threshold
  );
}

/**
 * Calculates a FitTarget for MapController from backend extent data.
 */
export function calculateExtentFitTarget(
  extent: MapExtentData,
  configuredMaxZoom = 19
): FitTarget {
  if (!extent || extent.count === 0 || !extent.bounds) {
    return { type: 'none', count: 0 };
  }

  const effectiveMaxZoom =
    Number.isFinite(configuredMaxZoom) && configuredMaxZoom > 0 ? configuredMaxZoom : 19;
  const { west, south, east, north } = extent.bounds;

  // Single point or identical coordinates
  if (west === east && south === north) {
    return {
      type: 'center',
      center: [south, west],
      zoom: Math.min(13, effectiveMaxZoom),
      count: extent.count,
    };
  }

  // Standard non-antimeridian bounds
  if (west <= east) {
    const targetMaxZoom = Math.min(14, effectiveMaxZoom);
    return {
      type: 'bounds',
      bounds: [
        [south, west],
        [north, east],
      ],
      padding: [44, 44],
      maxZoom: targetMaxZoom,
      count: extent.count,
    };
  }

  // Antimeridian wrap: bounds cross the 180th meridian.
  // Fit standard world or center between the antimeridian spans
  const targetMaxZoom = Math.min(12, effectiveMaxZoom);
  return {
    type: 'bounds',
    bounds: [
      [south, -180],
      [north, 180],
    ],
    padding: [44, 44],
    maxZoom: targetMaxZoom,
    count: extent.count,
  };
}
