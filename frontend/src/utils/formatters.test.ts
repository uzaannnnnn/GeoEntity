import { describe, it, expect } from 'vitest';
import { formatCoordinate, formatTypeLabel, formatStatusLabel } from './formatters';

describe('formatters', () => {
  it('formats coordinates to 6 decimal places', () => {
    expect(formatCoordinate(-6.2087634)).toBe('-6.208763');
    expect(formatCoordinate(106.8456)).toBe('106.845600');
    expect(formatCoordinate(NaN)).toBe('—');
  });

  it('formats entity type labels correctly', () => {
    expect(formatTypeLabel('vehicle')).toBe('Vehicle');
    expect(formatTypeLabel('iot_device')).toBe('IoT Device');
    expect(formatTypeLabel('facility')).toBe('Facility');
    expect(formatTypeLabel('other')).toBe('Other');
  });

  it('formats entity status labels correctly', () => {
    expect(formatStatusLabel('active')).toBe('Active');
    expect(formatStatusLabel('inactive')).toBe('Inactive');
    expect(formatStatusLabel('maintenance')).toBe('Maintenance');
  });
});
