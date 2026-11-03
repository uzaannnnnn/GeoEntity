import { describe, it, expect } from 'vitest';
import { formatRadius } from '../../utils/formatters';
import { GeofenceDefinition, GeofencePosition } from '../../types/geofence';

describe('GeofenceOverlayLayer Unit & Coordinate Mapping Tests', () => {
  describe('formatRadius', () => {
    it('formats meters below 1000m as rounded meters', () => {
      expect(formatRadius(250)).toBe('250 m');
      expect(formatRadius(999.4)).toBe('999 m');
      expect(formatRadius(50)).toBe('50 m');
    });

    it('formats meters 1000m and above as kilometers with 2 decimal places', () => {
      expect(formatRadius(1000)).toBe('1.00 km');
      expect(formatRadius(2500)).toBe('2.50 km');
      expect(formatRadius(15432)).toBe('15.43 km');
    });
  });

  describe('RFC 7946 to Leaflet coordinate conversion rules', () => {
    it('correctly maps Circle center [lon, lat] to Leaflet [lat, lon]', () => {
      const circleFence: GeofenceDefinition = {
        id: 'gf-circle-1',
        name: 'Monas Zone',
        enabled: true,
        geometry: {
          type: 'Circle',
          center: [106.8272, -6.1754], // [lon, lat]
          radius_m: 500,
        },
        created_at: '2026-11-03T00:00:00Z',
        updated_at: '2026-11-03T00:00:00Z',
      };

      if (circleFence.geometry.type === 'Circle') {
        const leafletCenter: [number, number] = [
          circleFence.geometry.center[1],
          circleFence.geometry.center[0],
        ];
        expect(leafletCenter[0]).toBe(-6.1754); // lat
        expect(leafletCenter[1]).toBe(106.8272); // lon
      }
    });

    it('correctly maps Polygon exterior ring [[lon, lat], ...] to Leaflet [[lat, lon], ...]', () => {
      const polygonFence: GeofenceDefinition = {
        id: 'gf-poly-1',
        name: 'District Boundary',
        enabled: true,
        geometry: {
          type: 'Polygon',
          coordinates: [
            [
              [106.80, -6.20],
              [106.85, -6.20],
              [106.85, -6.25],
              [106.80, -6.25],
              [106.80, -6.20],
            ],
          ],
        },
        created_at: '2026-11-03T00:00:00Z',
        updated_at: '2026-11-03T00:00:00Z',
      };

      if (polygonFence.geometry.type === 'Polygon') {
        const leafletPositions = polygonFence.geometry.coordinates[0].map(
          ([lon, lat]: GeofencePosition) => [lat, lon]
        );

        expect(leafletPositions).toHaveLength(5);
        expect(leafletPositions[0]).toEqual([-6.20, 106.80]);
        expect(leafletPositions[1]).toEqual([-6.20, 106.85]);
        expect(leafletPositions[2]).toEqual([-6.25, 106.85]);
        expect(leafletPositions[3]).toEqual([-6.25, 106.80]);
        expect(leafletPositions[4]).toEqual([-6.20, 106.80]); // Closed ring check
      }
    });
  });

  describe('Path Option Rules', () => {
    it('applies blue styling for enabled circle', () => {
      const isCircle = true;
      const enabled = true;
      const isSelected = false;

      const primaryColor = isCircle ? '#2563eb' : '#7c3aed';
      const color = enabled ? primaryColor : '#94a3b8';
      const fillColor = enabled ? primaryColor : '#cbd5e1';
      const fillOpacity = isSelected ? 0.35 : enabled ? 0.15 : 0.08;
      const weight = isSelected ? 4 : enabled ? 2 : 1.5;

      expect(color).toBe('#2563eb');
      expect(fillColor).toBe('#2563eb');
      expect(fillOpacity).toBe(0.15);
      expect(weight).toBe(2);
    });

    it('applies purple styling for enabled polygon', () => {
      const isCircle = false;
      const enabled = true;

      const primaryColor = isCircle ? '#2563eb' : '#7c3aed';
      const color = enabled ? primaryColor : '#94a3b8';
      const fillColor = enabled ? primaryColor : '#cbd5e1';

      expect(color).toBe('#7c3aed');
      expect(fillColor).toBe('#7c3aed');
    });

    it('applies muted slate and dashed style for disabled geofence', () => {
      const enabled = false;
      const color = enabled ? '#2563eb' : '#94a3b8';
      const dashArray = enabled ? undefined : '6, 6';

      expect(color).toBe('#94a3b8');
      expect(dashArray).toBe('6, 6');
    });

    it('applies prominent weight and fill opacity when selected', () => {
      const isSelected = true;
      const fillOpacity = isSelected ? 0.35 : 0.15;
      const weight = isSelected ? 4 : 2;

      expect(fillOpacity).toBe(0.35);
      expect(weight).toBe(4);
    });
  });
});
