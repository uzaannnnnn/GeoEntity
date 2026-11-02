import { Entity } from '../types/entity';

export interface ValidPoint {
  lat: number;
  lng: number;
  id?: string;
}

export type FitTarget =
  | { type: 'none'; count: 0 }
  | { type: 'center'; center: [number, number]; zoom: number; count: number }
  | {
      type: 'bounds';
      bounds: [[number, number], [number, number]];
      padding: [number, number];
      maxZoom: number;
      count: number;
    };

/**
 * Validates that latitude and longitude are finite numbers within standard geographic boundaries.
 */
export function isValidCoordinate(lat: unknown, lng: unknown): lat is number {
  if (typeof lat !== 'number' || typeof lng !== 'number') {
    return false;
  }
  if (!Number.isFinite(lat) || !Number.isFinite(lng)) {
    return false;
  }
  if (lat < -90 || lat > 90) {
    return false;
  }
  if (lng < -180 || lng > 180) {
    return false;
  }
  return true;
}

/**
 * Filters an entity array to only those with valid geographic coordinates.
 * Ignores non-finite, missing, out-of-range, NaN, or Infinity coordinates.
 */
export function getValidEntityPoints(entities: Entity[]): ValidPoint[] {
  if (!Array.isArray(entities)) {
    return [];
  }
  const valid: ValidPoint[] = [];
  for (const entity of entities) {
    if (isValidCoordinate(entity.latitude, entity.longitude)) {
      valid.push({
        lat: entity.latitude,
        lng: entity.longitude,
        id: entity.id,
      });
    }
  }
  return valid;
}

/**
 * Calculates the appropriate camera fit target based on valid coordinates.
 * Handles zero points, single points, identical points, and multiple distinct points.
 */
export function calculateFitTarget(
  points: ValidPoint[],
  configuredMaxZoom = 19
): FitTarget {
  if (!points || points.length === 0) {
    return { type: 'none', count: 0 };
  }

  const effectiveMaxZoom = Number.isFinite(configuredMaxZoom) && configuredMaxZoom > 0
    ? configuredMaxZoom
    : 19;

  // Single point case
  if (points.length === 1) {
    const targetZoom = Math.min(13, effectiveMaxZoom);
    return {
      type: 'center',
      center: [points[0].lat, points[0].lng],
      zoom: targetZoom,
      count: 1,
    };
  }

  // Check if all points share identical coordinates
  const first = points[0];
  let allIdentical = true;
  let minLat = first.lat;
  let maxLat = first.lat;
  let minLng = first.lng;
  let maxLng = first.lng;

  for (let i = 1; i < points.length; i++) {
    const p = points[i];
    if (p.lat !== first.lat || p.lng !== first.lng) {
      allIdentical = false;
    }
    if (p.lat < minLat) minLat = p.lat;
    if (p.lat > maxLat) maxLat = p.lat;
    if (p.lng < minLng) minLng = p.lng;
    if (p.lng > maxLng) maxLng = p.lng;
  }

  if (allIdentical) {
    const targetZoom = Math.min(13, effectiveMaxZoom);
    return {
      type: 'center',
      center: [first.lat, first.lng],
      zoom: targetZoom,
      count: points.length,
    };
  }

  const targetMaxZoom = Math.min(14, effectiveMaxZoom);
  return {
    type: 'bounds',
    bounds: [
      [minLat, minLng],
      [maxLat, maxLng],
    ],
    padding: [44, 44],
    maxZoom: targetMaxZoom,
    count: points.length,
  };
}
