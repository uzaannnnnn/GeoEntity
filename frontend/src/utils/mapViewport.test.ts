import { describe, it, expect } from 'vitest';
import {
  normalizeLongitude,
  clampLatitude,
  normalizeLeafletBounds,
  areBoundsEqual,
  calculateExtentFitTarget,
} from './mapViewport';

describe('mapViewport utils', () => {
  it('normalizes longitude wrapping to [-180, 180]', () => {
    expect(normalizeLongitude(106.8)).toBe(106.8);
    expect(normalizeLongitude(185)).toBe(-175);
    expect(normalizeLongitude(-190)).toBe(170);
    expect(Math.abs(normalizeLongitude(540))).toBe(180);
    expect(Math.abs(normalizeLongitude(-540))).toBe(180);
    expect(normalizeLongitude(NaN)).toBe(0);
  });

  it('clamps latitude to [-90, 90]', () => {
    expect(clampLatitude(-6.2)).toBe(-6.2);
    expect(clampLatitude(95)).toBe(90);
    expect(clampLatitude(-105)).toBe(-90);
    expect(clampLatitude(NaN)).toBe(0);
  });

  it('normalizes standard Leaflet bounds', () => {
    const mockBounds = {
      getWest: () => 100.5,
      getEast: () => 110.2,
      getSouth: () => -8.5,
      getNorth: () => -5.1,
    };
    const result = normalizeLeafletBounds(mockBounds);
    expect(result).toEqual({
      west: 100.5,
      south: -8.5,
      east: 110.2,
      north: -5.1,
    });
  });

  it('normalizes antimeridian Leaflet bounds', () => {
    const mockBounds = {
      getWest: () => 175,
      getEast: () => 185, // 185 wraps to -175
      getSouth: () => -10,
      getNorth: () => -5,
    };
    const result = normalizeLeafletBounds(mockBounds);
    expect(result).toEqual({
      west: 175,
      south: -10,
      east: -175,
      north: -5,
    });
  });

  it('handles wide views spanning 360 degrees or more', () => {
    const mockBounds = {
      getWest: () => -200,
      getEast: () => 200, // span = 400 >= 360
      getSouth: () => -70,
      getNorth: () => 80,
    };
    const result = normalizeLeafletBounds(mockBounds);
    expect(result).toEqual({
      west: -180,
      south: -70,
      east: 180,
      north: 80,
    });
  });

  it('detects equal bounds within threshold', () => {
    const b1 = { west: 10, south: -5, east: 20, north: 5 };
    const b2 = { west: 10.00001, south: -5.00002, east: 20.00001, north: 5.00003 };
    const b3 = { west: 11, south: -5, east: 20, north: 5 };

    expect(areBoundsEqual(b1, b2)).toBe(true);
    expect(areBoundsEqual(b1, b3)).toBe(false);
    expect(areBoundsEqual(null, null)).toBe(true);
    expect(areBoundsEqual(b1, null)).toBe(false);
  });

  it('calculates fit target from backend extent', () => {
    // Empty extent
    expect(calculateExtentFitTarget({ count: 0, bounds: null })).toEqual({
      type: 'none',
      count: 0,
    });

    // Single point extent
    expect(
      calculateExtentFitTarget({
        count: 1,
        bounds: { west: 106.8, south: -6.2, east: 106.8, north: -6.2 },
      })
    ).toEqual({
      type: 'center',
      center: [-6.2, 106.8],
      zoom: 13,
      count: 1,
    });

    // Multiple points standard bounds
    const standard = calculateExtentFitTarget({
      count: 5,
      bounds: { west: 100, south: -10, east: 110, north: 0 },
    });
    expect(standard.type).toBe('bounds');
    if (standard.type === 'bounds') {
      expect(standard.bounds).toEqual([
        [-10, 100],
        [0, 110],
      ]);
      expect(standard.count).toBe(5);
    }

    // Antimeridian bounds
    const antimeridian = calculateExtentFitTarget({
      count: 3,
      bounds: { west: 175, south: -10, east: -175, north: 0 },
    });
    expect(antimeridian.type).toBe('bounds');
    if (antimeridian.type === 'bounds') {
      expect(antimeridian.bounds).toEqual([
        [-10, -180],
        [0, 180],
      ]);
      expect(antimeridian.count).toBe(3);
    }
  });
});
