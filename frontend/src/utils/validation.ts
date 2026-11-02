import { EntityInput, EntityType, EntityStatus } from '../types/entity';

export const ALLOWED_TYPES: readonly EntityType[] = ['vehicle', 'iot_device', 'facility', 'other'] as const;
export const ALLOWED_STATUSES: readonly EntityStatus[] = ['active', 'inactive', 'maintenance'] as const;

export interface ValidationErrors {
  name?: string;
  type?: string;
  status?: string;
  description?: string;
  latitude?: string;
  longitude?: string;
  general?: string;
}

export function validateEntityInput(input: Partial<EntityInput>): ValidationErrors {
  const errors: ValidationErrors = {};

  // Name validation
  const trimmedName = (input.name ?? '').trim();
  if (!trimmedName) {
    errors.name = 'Name is required';
  }

  // Type validation
  if (!input.type || !ALLOWED_TYPES.includes(input.type)) {
    errors.type = 'Type must be one of: vehicle, iot_device, facility, other';
  }

  // Status validation
  if (!input.status || !ALLOWED_STATUSES.includes(input.status)) {
    errors.status = 'Status must be one of: active, inactive, maintenance';
  }

  // Latitude validation
  if (input.latitude === undefined || input.latitude === null || Number.isNaN(Number(input.latitude))) {
    errors.latitude = 'Latitude is required';
  } else {
    const lat = Number(input.latitude);
    if (!Number.isFinite(lat) || lat < -90 || lat > 90) {
      errors.latitude = 'Latitude must be a finite number between -90 and 90';
    }
  }

  // Longitude validation
  if (input.longitude === undefined || input.longitude === null || Number.isNaN(Number(input.longitude))) {
    errors.longitude = 'Longitude is required';
  } else {
    const lng = Number(input.longitude);
    if (!Number.isFinite(lng) || lng < -180 || lng > 180) {
      errors.longitude = 'Longitude must be a finite number between -180 and 180';
    }
  }

  return errors;
}

export function hasErrors(errors: ValidationErrors): boolean {
  return Object.values(errors).some((msg) => Boolean(msg));
}
