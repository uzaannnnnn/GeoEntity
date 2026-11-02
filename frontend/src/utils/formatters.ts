import { EntityType, EntityStatus } from '../types/entity';

export function formatCoordinate(val: number): string {
  return Number.isFinite(val) ? val.toFixed(6) : '—';
}

export function formatDateTime(isoString: string): string {
  if (!isoString) return '—';
  try {
    const date = new Date(isoString);
    if (Number.isNaN(date.getTime())) return isoString;
    return date.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      timeZoneName: 'short',
    });
  } catch {
    return isoString;
  }
}

export function formatTypeLabel(type: EntityType): string {
  switch (type) {
    case 'vehicle':
      return 'Vehicle';
    case 'iot_device':
      return 'IoT Device';
    case 'facility':
      return 'Facility';
    case 'other':
      return 'Other';
    default:
      return type;
  }
}

export function formatStatusLabel(status: EntityStatus): string {
  switch (status) {
    case 'active':
      return 'Active';
    case 'inactive':
      return 'Inactive';
    case 'maintenance':
      return 'Maintenance';
    default:
      return status;
  }
}
