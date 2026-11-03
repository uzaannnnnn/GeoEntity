import { describe, it, expect, vi, beforeEach } from 'vitest';
import { entitiesApi } from './entities';
import { calculateExtentFitTarget } from '../utils/mapViewport';
import { RequestGuard } from '../utils/requestGuard';

describe('V2.4C Server Data Operations Integration', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('Server-Side Entity Query API', () => {
    it('constructs correct URL omitting empty and "all" filters', async () => {
      let requestedUrl = '';
      vi.spyOn(globalThis, 'fetch').mockImplementationOnce(async (url) => {
        requestedUrl = String(url);
        return new Response(
          JSON.stringify({
            data: [],
            meta: {
              limit: 50,
              next_cursor: '',
              matching_count: 0,
              status_counts: { active: 0, inactive: 0, maintenance: 0 },
            },
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        );
      });

      await entitiesApi.query({
        q: '  Fleet Alpha  ',
        type: 'all',
        status: 'active',
        limit: 50,
      });

      expect(requestedUrl).toBe('/api/entities/query?q=Fleet+Alpha&status=active&limit=50');
    });

    it('passes cursor for pagination continuation', async () => {
      let requestedUrl = '';
      vi.spyOn(globalThis, 'fetch').mockImplementationOnce(async (url) => {
        requestedUrl = String(url);
        return new Response(
          JSON.stringify({
            data: [],
            meta: {
              limit: 50,
              next_cursor: '',
              matching_count: 5,
              status_counts: { active: 3, inactive: 2, maintenance: 0 },
            },
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        );
      });

      await entitiesApi.query({
        cursor: 'eyJ2IjoxLCJpZCI6IjAwMDEifQ',
      });

      expect(requestedUrl).toBe('/api/entities/query?cursor=eyJ2IjoxLCJpZCI6IjAwMDEifQ');
    });
  });

  describe('Independent Map Dataset API', () => {
    it('requests markers using exact viewport bounds and active filters', async () => {
      let requestedUrl = '';
      vi.spyOn(globalThis, 'fetch').mockImplementationOnce(async (url) => {
        requestedUrl = String(url);
        return new Response(JSON.stringify({ data: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      });

      await entitiesApi.getMapEntities({
        west: 106.5,
        south: -6.5,
        east: 107.0,
        north: -6.0,
        q: 'depot',
        type: 'facility',
      });

      expect(requestedUrl).toBe(
        '/api/entities/map?west=106.5&south=-6.5&east=107&north=-6&q=depot&type=facility'
      );
    });

    it('handles antimeridian wrap where west > east', async () => {
      let requestedUrl = '';
      vi.spyOn(globalThis, 'fetch').mockImplementationOnce(async (url) => {
        requestedUrl = String(url);
        return new Response(JSON.stringify({ data: [] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      });

      await entitiesApi.getMapEntities({
        west: 178,
        south: -18,
        east: -178,
        north: -16,
      });

      expect(requestedUrl).toBe('/api/entities/map?west=178&south=-18&east=-178&north=-16');
    });

    it('throws ApiError with MAP_RESULT_LIMIT on HTTP 413 response', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            error: {
              code: 'MAP_RESULT_LIMIT',
              message: 'More than 5000 entities match this map view; zoom in to see markers',
            },
          }),
          { status: 413, headers: { 'Content-Type': 'application/json' } }
        )
      );

      await expect(
        entitiesApi.getMapEntities({
          west: -180,
          south: -90,
          east: 180,
          north: 90,
        })
      ).rejects.toMatchObject({
        name: 'ApiError',
        status: 413,
        code: 'MAP_RESULT_LIMIT',
        message: 'More than 5000 entities match this map view; zoom in to see markers',
      });
    });
  });

  describe('Map Extent & Fit All Calculation', () => {
    it('queries extent endpoint with filters', async () => {
      let requestedUrl = '';
      vi.spyOn(globalThis, 'fetch').mockImplementationOnce(async (url) => {
        requestedUrl = String(url);
        return new Response(
          JSON.stringify({
            data: {
              count: 3,
              bounds: { west: 106.2, south: -6.4, east: 106.9, north: -6.1 },
            },
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        );
      });

      const res = await entitiesApi.getMapExtent({ q: 'Truck', status: 'active' });
      expect(requestedUrl).toBe('/api/entities/map/extent?q=Truck&status=active');
      expect(res.count).toBe(3);
      expect(res.bounds).toEqual({ west: 106.2, south: -6.4, east: 106.9, north: -6.1 });
    });

    it('derives correct fit target for MapController from extent data', () => {
      const extent = {
        count: 2,
        bounds: { west: 106.2, south: -6.4, east: 106.9, north: -6.1 },
      };
      const target = calculateExtentFitTarget(extent, 19);
      expect(target.type).toBe('bounds');
      if (target.type === 'bounds') {
        expect(target.bounds).toEqual([
          [-6.4, 106.2],
          [-6.1, 106.9],
        ]);
        expect(target.count).toBe(2);
      }
    });
  });

  describe('Race Condition and Stale Request Prevention', () => {
    it('guards against older slower requests replacing newer state', () => {
      const guard = new RequestGuard();

      const reqA = guard.begin();
      expect(reqA).toBe(1);
      expect(guard.isCurrent(reqA)).toBe(true);

      const reqB = guard.begin();
      expect(reqB).toBe(2);
      expect(guard.isCurrent(reqB)).toBe(true);
      // Older request A is no longer current
      expect(guard.isCurrent(reqA)).toBe(false);

      // Invalidation discards both
      guard.invalidate();
      expect(guard.isCurrent(reqB)).toBe(false);
    });
  });
});
