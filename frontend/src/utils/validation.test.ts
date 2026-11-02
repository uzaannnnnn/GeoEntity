import { describe, it, expect } from 'vitest';
import { validateEntityInput, hasErrors } from './validation';
import { EntityInput } from '../types/entity';

describe('validateEntityInput', () => {
  const validPayload: EntityInput = {
    name: 'Vehicle Alpha',
    type: 'vehicle',
    description: 'Fleet car',
    status: 'active',
    latitude: -6.2088,
    longitude: 106.8456,
  };

  it('passes for a valid entity input', () => {
    const errors = validateEntityInput(validPayload);
    expect(hasErrors(errors)).toBe(false);
    expect(Object.keys(errors)).toHaveLength(0);
  });

  it('rejects empty or whitespace-only name', () => {
    const err1 = validateEntityInput({ ...validPayload, name: '' });
    expect(err1.name).toBe('Name is required');

    const err2 = validateEntityInput({ ...validPayload, name: '   ' });
    expect(err2.name).toBe('Name is required');
  });

  it('rejects unsupported types', () => {
    // @ts-expect-error testing invalid type
    const err = validateEntityInput({ ...validPayload, type: 'airplane' });
    expect(err.type).toBe('Type must be one of: vehicle, iot_device, facility, other');
  });

  it('accepts all allowed types', () => {
    const types = ['vehicle', 'iot_device', 'facility', 'other'] as const;
    for (const t of types) {
      const err = validateEntityInput({ ...validPayload, type: t });
      expect(err.type).toBeUndefined();
    }
  });

  it('rejects unsupported status', () => {
    // @ts-expect-error testing invalid status
    const err = validateEntityInput({ ...validPayload, status: 'unknown' });
    expect(err.status).toBe('Status must be one of: active, inactive, maintenance');
  });

  it('accepts all allowed statuses', () => {
    const statuses = ['active', 'inactive', 'maintenance'] as const;
    for (const s of statuses) {
      const err = validateEntityInput({ ...validPayload, status: s });
      expect(err.status).toBeUndefined();
    }
  });

  it('validates latitude bounds [-90, 90]', () => {
    expect(validateEntityInput({ ...validPayload, latitude: -90 }).latitude).toBeUndefined();
    expect(validateEntityInput({ ...validPayload, latitude: 90 }).latitude).toBeUndefined();
    expect(validateEntityInput({ ...validPayload, latitude: 0 }).latitude).toBeUndefined();

    const errUnder = validateEntityInput({ ...validPayload, latitude: -90.0001 });
    expect(errUnder.latitude).toBe('Latitude must be a finite number between -90 and 90');

    const errOver = validateEntityInput({ ...validPayload, latitude: 90.0001 });
    expect(errOver.latitude).toBe('Latitude must be a finite number between -90 and 90');

    const errMissing = validateEntityInput({ ...validPayload, latitude: undefined });
    expect(errMissing.latitude).toBe('Latitude is required');

    const errNaN = validateEntityInput({ ...validPayload, latitude: NaN });
    expect(errNaN.latitude).toBe('Latitude is required');

    const errInf = validateEntityInput({ ...validPayload, latitude: Infinity });
    expect(errInf.latitude).toBe('Latitude must be a finite number between -90 and 90');
  });

  it('validates longitude bounds [-180, 180]', () => {
    expect(validateEntityInput({ ...validPayload, longitude: -180 }).longitude).toBeUndefined();
    expect(validateEntityInput({ ...validPayload, longitude: 180 }).longitude).toBeUndefined();
    expect(validateEntityInput({ ...validPayload, longitude: 0 }).longitude).toBeUndefined();

    const errUnder = validateEntityInput({ ...validPayload, longitude: -180.0001 });
    expect(errUnder.longitude).toBe('Longitude must be a finite number between -180 and 180');

    const errOver = validateEntityInput({ ...validPayload, longitude: 180.0001 });
    expect(errOver.longitude).toBe('Longitude must be a finite number between -180 and 180');

    const errMissing = validateEntityInput({ ...validPayload, longitude: undefined });
    expect(errMissing.longitude).toBe('Longitude is required');

    const errNaN = validateEntityInput({ ...validPayload, longitude: NaN });
    expect(errNaN.longitude).toBe('Longitude is required');

    const errInf = validateEntityInput({ ...validPayload, longitude: -Infinity });
    expect(errInf.longitude).toBe('Longitude must be a finite number between -180 and 180');
  });
});
