import type { GeofenceGeometry, GeofencePosition } from '../types/geofence';

export type GeofenceGeometryType = 'Circle' | 'Polygon';
export type GeofenceFormErrors = Record<string, string>;

interface GeofenceFormValues {
  name: string;
  circleLat: string;
  circleLon: string;
  circleRadius: string;
  polyCoordsJson: string;
}

export function validateGeofenceGeometry(
  geomType: GeofenceGeometryType,
  values: GeofenceFormValues
): { geometry: GeofenceGeometry | null; errors: GeofenceFormErrors } {
  const errors: GeofenceFormErrors = {};
  const name = values.name.trim();

  if (!name) {
    errors.name = 'Geofence name is required (1 to 100 characters).';
  } else if (name.length > 100) {
    errors.name = 'Geofence name cannot exceed 100 characters.';
  }

  if (geomType === 'Circle') {
    const lat = Number.parseFloat(values.circleLat);
    const lon = Number.parseFloat(values.circleLon);
    const radius = Number.parseFloat(values.circleRadius);

    if (!Number.isFinite(lat) || lat < -90 || lat > 90) {
      errors.circleLat = 'Latitude must be a valid number between -90 and 90.';
    }
    if (!Number.isFinite(lon) || lon < -180 || lon > 180) {
      errors.circleLon = 'Longitude must be a valid number between -180 and 180.';
    }
    if (!Number.isFinite(radius) || radius <= 0 || radius > 10000000) {
      errors.circleRadius = 'Radius must be a positive number up to 10,000,000 meters.';
    }

    if (Object.keys(errors).length > 0) return { geometry: null, errors };
    return {
      geometry: { type: 'Circle', center: [lon, lat], radius_m: radius },
      errors,
    };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(values.polyCoordsJson);
  } catch {
    errors.polygon = 'Invalid JSON syntax for polygon coordinates.';
    return { geometry: null, errors };
  }

  if (!Array.isArray(parsed) || parsed.length === 0 || !Array.isArray(parsed[0])) {
    errors.polygon = 'Coordinates must be an array containing an exterior ring of [lon, lat] points.';
    return { geometry: null, errors };
  }

  const ring = parsed[0] as unknown[];
  if (ring.length < 4) {
    errors.polygon = 'Exterior ring must have at least 4 positions (minimum 3 distinct vertices plus closed end).';
  } else {
    const first = ring[0];
    const last = ring[ring.length - 1];
    const isClosed = Array.isArray(first) && Array.isArray(last) && first[0] === last[0] && first[1] === last[1];
    if (!isClosed) {
      errors.polygon = 'Exterior ring must be closed (first and last positions must be identical).';
    }
  }

  if (Object.keys(errors).length > 0) return { geometry: null, errors };
  return {
    geometry: { type: 'Polygon', coordinates: parsed as GeofencePosition[][] },
    errors,
  };
}
