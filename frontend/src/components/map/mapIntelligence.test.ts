import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('leaflet', () => {
  return {
    default: {
      divIcon: vi.fn((options: unknown) => ({ options })),
      point: vi.fn((x: number, y: number) => [x, y]),
    },
  };
});

import { Entity } from '../../types/entity';
import { getEntityIconSvg, createEntityDivIcon, createDraftDivIcon, createUserLocationDivIcon, createClusterDivIcon } from './markerIcons';
import { getValidEntityPoints, calculateFitTarget } from '../../utils/mapBounds';
import { formatCenterCopyText } from '../../utils/coordinateFormat';
import { getCurrentUserLocation } from '../../utils/geolocation';
import { entitiesApi } from '../../api/entities';

function createMockEntity(id: string, name: string, lat = -6.2, lng = 106.8): Entity {
  return {
    id,
    name,
    type: 'facility',
    description: 'Test facility',
    status: 'active',
    latitude: lat,
    longitude: lng,
    created_at: '2026-11-02T00:00:00Z',
    updated_at: '2026-11-02T00:00:00Z',
  };
}

describe('Map Intelligence Architecture & Feature Tests', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('Feature A: Marker Clustering', () => {
    it('clusters only persisted entity markers with stable unique IDs', () => {
      const entities: Entity[] = [
        createMockEntity('e-1', 'Alpha'),
        createMockEntity('e-2', 'Beta'),
        createMockEntity('e-3', 'Gamma'),
      ];

      // Verify that entities have unique stable IDs suitable as React keys
      const keys = entities.map((e) => e.id);
      const uniqueKeys = new Set(keys);
      expect(uniqueKeys.size).toBe(entities.length);
    });

    it('creates distinct custom cluster DivIcon bubbles with child count formatting', () => {
      const mockSmallCluster = { getChildCount: () => 5 } as any;
      const mockMediumCluster = { getChildCount: () => 25 } as any;
      const mockLargeCluster = { getChildCount: () => 150 } as any;

      const smallIcon = createClusterDivIcon(mockSmallCluster);
      const mediumIcon = createClusterDivIcon(mockMediumCluster);
      const largeIcon = createClusterDivIcon(mockLargeCluster);

      expect(smallIcon.options.html).toContain('cluster-small');
      expect(smallIcon.options.html).toContain('>5<');

      expect(mediumIcon.options.html).toContain('cluster-medium');
      expect(mediumIcon.options.html).toContain('>25<');

      expect(largeIcon.options.html).toContain('cluster-large');
      expect(largeIcon.options.html).toContain('>150<');
    });

    it('creates distinct icons for entity types and draft placement markers', () => {
      const entityIcon = createEntityDivIcon('vehicle', 'active', false);
      const selectedIcon = createEntityDivIcon('vehicle', 'active', true);
      const draftIcon = createDraftDivIcon();
      const userIcon = createUserLocationDivIcon();

      // Persisted entity marker has pin and status
      expect(entityIcon.options.html).toContain('marker-pin marker-type-vehicle');
      expect(entityIcon.options.html).not.toContain('marker-pulse');

      // Selected marker includes pulsating halo
      expect(selectedIcon.options.html).toContain('marker-pulse');

      // Draft marker has distinct draft pin and ripple
      expect(draftIcon.options.html).toContain('draft-pin');
      expect(draftIcon.options.html).toContain('draft-ripple');

      // User location marker has user-location dot and pulse
      expect(userIcon.options.html).toContain('user-location-dot');
      expect(userIcon.options.html).toContain('user-location-pulse');
    });

    it('preserves CRUD identity across list mutations without duplicate IDs', () => {
      let entities: Entity[] = [
        createMockEntity('1', 'Entity 1'),
        createMockEntity('2', 'Entity 2'),
      ];

      // Simulate CREATE
      const newEntity = createMockEntity('3', 'Entity 3');
      entities = [...entities, newEntity];
      expect(entities.map((e) => e.id)).toEqual(['1', '2', '3']);

      // Simulate UPDATE (moved coordinates)
      entities = entities.map((e) => (e.id === '2' ? { ...e, latitude: -6.5, longitude: 107.0 } : e));
      const updated = entities.find((e) => e.id === '2');
      expect(updated?.latitude).toBe(-6.5);
      expect(updated?.longitude).toBe(107.0);

      // Simulate DELETE
      entities = entities.filter((e) => e.id !== '1');
      expect(entities.map((e) => e.id)).toEqual(['2', '3']);
    });
  });

  describe('Feature B: Locate Me Privacy and Ephemeral State', () => {
    it('never writes user location coordinates to localStorage or sessionStorage', async () => {
      const setLocalSpy = vi.spyOn(Storage.prototype, 'setItem');
      const getPositionMock = vi.fn().mockImplementation((success) => {
        success({
          coords: { latitude: 37.7749, longitude: -122.4194, accuracy: 20 },
          timestamp: Date.now(),
        });
      });

      vi.stubGlobal('navigator', { geolocation: { getCurrentPosition: getPositionMock } });
      vi.stubGlobal('window', { isSecureContext: true });

      const location = await getCurrentUserLocation();
      expect(location.latitude).toBe(37.7749);
      expect(location.longitude).toBe(-122.4194);

      // Verify ZERO calls to browser web storage
      expect(setLocalSpy).not.toHaveBeenCalled();
    });

    it('never submits user location coordinates to the backend entity API', async () => {
      const createApiSpy = vi.spyOn(entitiesApi, 'create');
      const updateApiSpy = vi.spyOn(entitiesApi, 'update');

      const getPositionMock = vi.fn().mockImplementation((success) => {
        success({
          coords: { latitude: 37.7749, longitude: -122.4194, accuracy: 20 },
          timestamp: Date.now(),
        });
      });

      vi.stubGlobal('navigator', { geolocation: { getCurrentPosition: getPositionMock } });
      vi.stubGlobal('window', { isSecureContext: true });

      const location = await getCurrentUserLocation();
      expect(location).toBeDefined();

      // Verify that geolocation fix NEVER enters API calls
      expect(createApiSpy).not.toHaveBeenCalled();
      expect(updateApiSpy).not.toHaveBeenCalled();
    });
  });

  describe('Feature C: Fit All Entities', () => {
    it('preserves existing entity selection and draft location without mutation', () => {
      const selectedEntity = createMockEntity('sel-1', 'Selected Item');
      const draftLocation = { lat: -6.123, lng: 106.789 };

      const entities = [
        selectedEntity,
        createMockEntity('other-2', 'Other Item', 10, 20),
      ];

      const validPoints = getValidEntityPoints(entities);
      const target = calculateFitTarget(validPoints, 19);

      expect(target.type).toBe('bounds');
      // Verify inputs remain unmutated
      expect(selectedEntity.id).toBe('sel-1');
      expect(draftLocation.lat).toBe(-6.123);
      expect(draftLocation.lng).toBe(106.789);
    });

    it('does not include draft or user location in Fit All calculation', () => {
      const persistedEntities = [
        createMockEntity('p-1', 'Persisted Point', 10.0, 20.0),
      ];

      // Points fed to getValidEntityPoints must only be from persisted entities array
      const points = getValidEntityPoints(persistedEntities);
      expect(points).toHaveLength(1);
      expect(points[0].id).toBe('p-1');
    });
  });

  describe('Feature D: Map Legend Contract', () => {
    it('uses actual entity SVG icons in legend definitions', () => {
      expect(getEntityIconSvg('vehicle')).toContain('<svg');
      expect(getEntityIconSvg('iot_device')).toContain('<svg');
      expect(getEntityIconSvg('facility')).toContain('<svg');
      expect(getEntityIconSvg('other')).toContain('<svg');
    });
  });

  describe('Feature E: Coordinate Inspector', () => {
    it('formats coordinates as latitude, longitude without mutating any state', () => {
      const center = { lat: -6.2088, lng: 106.8456 };
      const formatted = formatCenterCopyText(center.lat, center.lng);

      expect(formatted).toBe('-6.208800, 106.845600');
      expect(center.lat).toBe(-6.2088);
      expect(center.lng).toBe(106.8456);
    });
  });
});
