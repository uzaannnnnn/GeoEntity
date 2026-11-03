import { describe, it, expect, vi, beforeEach } from 'vitest';
import { geofencesApi } from './geofences';
import { apiClient } from './client';
import { GeofenceDefinition, GeofencePage, GeofenceEventPage, EntityGeofenceMembership } from '../types/geofence';

describe('Geofences API Client', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  const mockCircleGeofence: GeofenceDefinition = {
    id: 'geo-circle-1',
    name: 'Depot Alpha Zone',
    enabled: true,
    geometry: {
      type: 'Circle',
      center: [106.8272, -6.1754],
      radius_m: 500,
    },
    created_at: '2026-11-03T00:00:00Z',
    updated_at: '2026-11-03T00:00:00Z',
  };

  const mockPolygonGeofence: GeofenceDefinition = {
    id: 'geo-poly-1',
    name: 'Harbor Perimeter',
    enabled: false,
    geometry: {
      type: 'Polygon',
      coordinates: [
        [
          [106.8, -6.1],
          [106.9, -6.1],
          [106.9, -6.2],
          [106.8, -6.2],
          [106.8, -6.1],
        ],
      ],
    },
    created_at: '2026-11-03T00:00:00Z',
    updated_at: '2026-11-03T00:00:00Z',
  };

  describe('list', () => {
    it('requests geofences without query parameters by default', async () => {
      const page: GeofencePage = {
        data: [mockCircleGeofence],
        meta: { limit: 50, next_cursor: '' },
      };
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce(page);

      const res = await geofencesApi.list();
      expect(apiClient.get).toHaveBeenCalledWith('/geofences');
      expect(res).toEqual(page);
    });

    it('passes enabled filter, limit, and cursor properly', async () => {
      const page: GeofencePage = {
        data: [mockCircleGeofence],
        meta: { limit: 25, next_cursor: 'cur123' },
      };
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce(page);

      const res = await geofencesApi.list({ enabled: true, limit: 25, cursor: 'cur123' });
      expect(apiClient.get).toHaveBeenCalledWith('/geofences?enabled=true&limit=25&cursor=cur123');
      expect(res).toEqual(page);
    });

    it('supports enabled=false filter', async () => {
      const page: GeofencePage = {
        data: [mockPolygonGeofence],
        meta: { limit: 50, next_cursor: '' },
      };
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce(page);

      const res = await geofencesApi.list({ enabled: false });
      expect(apiClient.get).toHaveBeenCalledWith('/geofences?enabled=false');
      expect(res).toEqual(page);
    });
  });

  describe('get', () => {
    it('fetches a single geofence definition by ID', async () => {
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: mockCircleGeofence });

      const res = await geofencesApi.get('geo-circle-1');
      expect(apiClient.get).toHaveBeenCalledWith('/geofences/geo-circle-1');
      expect(res).toEqual(mockCircleGeofence);
    });
  });

  describe('create', () => {
    it('creates a new circle geofence', async () => {
      vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ data: mockCircleGeofence });

      const input = {
        name: 'Depot Alpha Zone',
        enabled: true,
        geometry: mockCircleGeofence.geometry,
      };
      const res = await geofencesApi.create(input);
      expect(apiClient.post).toHaveBeenCalledWith('/geofences', input);
      expect(res).toEqual(mockCircleGeofence);
    });

    it('creates a polygon geofence', async () => {
      vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ data: mockPolygonGeofence });

      const input = {
        name: 'Harbor Perimeter',
        enabled: false,
        geometry: mockPolygonGeofence.geometry,
      };
      const res = await geofencesApi.create(input);
      expect(apiClient.post).toHaveBeenCalledWith('/geofences', input);
      expect(res).toEqual(mockPolygonGeofence);
    });
  });

  describe('update', () => {
    it('updates an existing geofence definition', async () => {
      const updated = { ...mockCircleGeofence, name: 'Renamed Depot' };
      vi.spyOn(apiClient, 'put').mockResolvedValueOnce({ data: updated });

      const input = {
        name: 'Renamed Depot',
        enabled: true,
        geometry: mockCircleGeofence.geometry,
      };
      const res = await geofencesApi.update('geo-circle-1', input);
      expect(apiClient.put).toHaveBeenCalledWith('/geofences/geo-circle-1', input);
      expect(res).toEqual(updated);
    });
  });

  describe('delete', () => {
    it('deletes a geofence by ID', async () => {
      vi.spyOn(apiClient, 'delete').mockResolvedValueOnce(undefined);

      await geofencesApi.delete('geo-circle-1');
      expect(apiClient.delete).toHaveBeenCalledWith('/geofences/geo-circle-1');
    });
  });

  describe('getEntityGeofences', () => {
    it('fetches geofence memberships for an entity', async () => {
      const memberships: EntityGeofenceMembership[] = [
        {
          geofence_id: 'geo-circle-1',
          geofence_name: 'Depot Alpha Zone',
          state: 'inside',
          evaluated_latitude: -6.1754,
          evaluated_longitude: 106.8272,
          evaluated_at: '2026-11-03T01:00:00Z',
          definition_updated_at: '2026-11-03T00:00:00Z',
        },
      ];
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: memberships });

      const res = await geofencesApi.getEntityGeofences('ent-100');
      expect(apiClient.get).toHaveBeenCalledWith('/entities/ent-100/geofences');
      expect(res).toEqual(memberships);
    });

    it('returns empty array when data is empty or null', async () => {
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: [] });

      const res = await geofencesApi.getEntityGeofences('ent-200');
      expect(res).toEqual([]);
    });
  });

  describe('getGeofenceEvents', () => {
    it('fetches events with default parameters', async () => {
      const eventPage: GeofenceEventPage = {
        data: [
          {
            id: 'evt-1',
            entity_id: 'ent-1',
            entity_name: 'Vehicle Fleet 1',
            geofence_id: 'geo-circle-1',
            geofence_name: 'Depot Alpha Zone',
            event_type: 'enter',
            latitude: -6.1754,
            longitude: 106.8272,
            occurred_at: '2026-11-03T02:00:00Z',
            source: 'coordinate_update',
          },
        ],
        meta: { limit: 50, next_cursor: '' },
      };
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce(eventPage);

      const res = await geofencesApi.getGeofenceEvents('geo-circle-1');
      expect(apiClient.get).toHaveBeenCalledWith('/geofences/geo-circle-1/events');
      expect(res).toEqual(eventPage);
    });

    it('encodes entity_id, from, to, limit, and cursor filters properly', async () => {
      const eventPage: GeofenceEventPage = {
        data: [],
        meta: { limit: 10, next_cursor: 'cur999' },
      };
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce(eventPage);

      const res = await geofencesApi.getGeofenceEvents('geo-circle-1', {
        entity_id: '00000000-0000-0000-0000-000000000001',
        from: '2026-11-01T00:00:00Z',
        to: '2026-11-03T23:59:59Z',
        limit: 10,
        cursor: 'cur999',
      });

      expect(apiClient.get).toHaveBeenCalledWith(
        '/geofences/geo-circle-1/events?entity_id=00000000-0000-0000-0000-000000000001&from=2026-11-01T00%3A00%3A00Z&to=2026-11-03T23%3A59%3A59Z&limit=10&cursor=cur999'
      );
      expect(res).toEqual(eventPage);
    });
  });
});
