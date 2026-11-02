import { describe, it, expect } from 'vitest';
import {
  isValidCoordinate,
  getValidEntityPoints,
  calculateFitTarget,
} from './mapBounds';
import { Entity } from '../types/entity';

function mockEntity(id: string, lat: number, lng: number): Entity {
  return {
    id,
    name: `Entity ${id}`,
    type: 'facility',
    description: '',
    status: 'active',
    latitude: lat,
    longitude: lng,
    created_at: '2026-11-02T00:00:00Z',
    updated_at: '2026-11-02T00:00:00Z',
  };
}

describe('mapBounds', () => {
  describe('isValidCoordinate', () => {
    it('accepts valid coordinates within geographic ranges', () => {
      expect(isValidCoordinate(0, 0)).toBe(true);
      expect(isValidCoordinate(-90, -180)).toBe(true);
      expect(isValidCoordinate(90, 180)).toBe(true);
      expect(isValidCoordinate(-6.2088, 106.8456)).toBe(true);
    });

    it('rejects coordinates outside geographic ranges', () => {
      expect(isValidCoordinate(-90.001, 0)).toBe(false);
      expect(isValidCoordinate(90.001, 0)).toBe(false);
      expect(isValidCoordinate(0, -180.001)).toBe(false);
      expect(isValidCoordinate(0, 180.001)).toBe(false);
    });

    it('rejects non-finite values (NaN, Infinity, -Infinity)', () => {
      expect(isValidCoordinate(NaN, 100)).toBe(false);
      expect(isValidCoordinate(10, NaN)).toBe(false);
      expect(isValidCoordinate(Infinity, 100)).toBe(false);
      expect(isValidCoordinate(10, -Infinity)).toBe(false);
    });

    it('rejects non-numeric types', () => {
      expect(isValidCoordinate(null, 10)).toBe(false);
      expect(isValidCoordinate(undefined, 10)).toBe(false);
      expect(isValidCoordinate('10' as unknown as number, 10)).toBe(false);
    });
  });

  describe('getValidEntityPoints', () => {
    it('returns empty array when entities array is empty or invalid', () => {
      expect(getValidEntityPoints([])).toEqual([]);
      expect(getValidEntityPoints(null as unknown as Entity[])).toEqual([]);
    });

    it('extracts valid points and filters out invalid points', () => {
      const entities = [
        mockEntity('1', -6.2, 106.8),
        mockEntity('2', 95.0, 50.0), // invalid lat
        mockEntity('3', NaN, 20.0), // NaN lat
        mockEntity('4', 10.0, 200.0), // invalid lng
        mockEntity('5', 12.0, 77.0), // valid
      ];

      const points = getValidEntityPoints(entities);
      expect(points).toHaveLength(2);
      expect(points[0]).toEqual({ lat: -6.2, lng: 106.8, id: '1' });
      expect(points[1]).toEqual({ lat: 12.0, lng: 77.0, id: '5' });
    });
  });

  describe('calculateFitTarget', () => {
    it('returns none for empty points array', () => {
      expect(calculateFitTarget([])).toEqual({ type: 'none', count: 0 });
    });

    it('returns center with zoom 13 for a single point', () => {
      const result = calculateFitTarget([{ lat: -6.2, lng: 106.8, id: '1' }]);
      expect(result).toEqual({
        type: 'center',
        center: [-6.2, 106.8],
        zoom: 13,
        count: 1,
      });
    });

    it('caps single-point zoom by configuredMaxZoom', () => {
      const result = calculateFitTarget([{ lat: -6.2, lng: 106.8, id: '1' }], 10);
      expect(result).toEqual({
        type: 'center',
        center: [-6.2, 106.8],
        zoom: 10,
        count: 1,
      });
    });

    it('treats multiple identical points as a single spatial extent', () => {
      const points = [
        { lat: 10.5, lng: 20.5, id: '1' },
        { lat: 10.5, lng: 20.5, id: '2' },
        { lat: 10.5, lng: 20.5, id: '3' },
      ];
      const result = calculateFitTarget(points);
      expect(result).toEqual({
        type: 'center',
        center: [10.5, 20.5],
        zoom: 13,
        count: 3,
      });
    });

    it('returns bounding box with padding and maxZoom for multiple distinct points', () => {
      const points = [
        { lat: -6.2, lng: 106.8, id: '1' },
        { lat: -7.0, lng: 110.0, id: '2' },
        { lat: 0.0, lng: 100.0, id: '3' },
      ];
      const result = calculateFitTarget(points);
      expect(result).toEqual({
        type: 'bounds',
        bounds: [
          [-7.0, 100.0],
          [0.0, 110.0],
        ],
        padding: [44, 44],
        maxZoom: 14,
        count: 3,
      });
    });

    it('caps multiple-point maxZoom by configuredMaxZoom', () => {
      const points = [
        { lat: 10, lng: 10, id: '1' },
        { lat: 20, lng: 20, id: '2' },
      ];
      const result = calculateFitTarget(points, 12);
      expect(result).toEqual({
        type: 'bounds',
        bounds: [
          [10, 10],
          [20, 20],
        ],
        padding: [44, 44],
        maxZoom: 12,
        count: 2,
      });
    });
  });
});
