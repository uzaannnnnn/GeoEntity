import { describe, expect, it } from 'vitest';
import { validateGeofenceGeometry } from './geofenceFormValidation';

const validValues = {
  name: 'North Depot',
  circleLat: '-6.2088',
  circleLon: '106.8456',
  circleRadius: '500',
  polyCoordsJson: '[[[106.81,-6.21],[106.83,-6.21],[106.83,-6.23],[106.81,-6.21]]]',
};

describe('validateGeofenceGeometry', () => {
  it('accepts circle values and preserves longitude-latitude ordering', () => {
    expect(validateGeofenceGeometry('Circle', validValues)).toEqual({
      geometry: { type: 'Circle', center: [106.8456, -6.2088], radius_m: 500 },
      errors: {},
    });
  });

  it('reports invalid circle controls without constructing geometry', () => {
    const result = validateGeofenceGeometry('Circle', {
      ...validValues,
      name: ' ',
      circleLat: '91',
      circleLon: '181',
      circleRadius: '0',
    });

    expect(result.geometry).toBeNull();
    expect(result.errors).toMatchObject({
      name: 'Geofence name is required (1 to 100 characters).',
      circleLat: 'Latitude must be a valid number between -90 and 90.',
      circleLon: 'Longitude must be a valid number between -180 and 180.',
      circleRadius: 'Radius must be a positive number up to 10,000,000 meters.',
    });
  });

  it('accepts a closed polygon and rejects malformed or unclosed rings', () => {
    expect(validateGeofenceGeometry('Polygon', validValues).geometry).toEqual({
      type: 'Polygon',
      coordinates: [[[106.81, -6.21], [106.83, -6.21], [106.83, -6.23], [106.81, -6.21]]],
    });
    expect(
      validateGeofenceGeometry('Polygon', { ...validValues, polyCoordsJson: '{' }).errors.polygon
    ).toBe('Invalid JSON syntax for polygon coordinates.');
    expect(
      validateGeofenceGeometry('Polygon', {
        ...validValues,
        polyCoordsJson: '[[[0,0],[1,0],[1,1],[0,1]]]'
      }).errors.polygon
    ).toBe('Exterior ring must be closed (first and last positions must be identical).');
  });
});
