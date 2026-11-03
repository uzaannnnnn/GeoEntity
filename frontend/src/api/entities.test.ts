import { describe, it, expect, vi, beforeEach } from 'vitest';
import { entitiesApi } from './entities';
import { EntityInput } from '../types/entity';

describe('entitiesApi', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches all entities correctly', async () => {
    const mockEntities = [
      {
        id: '11111111-1111-1111-1111-111111111111',
        name: 'Device A',
        type: 'iot_device',
        description: 'Sensor',
        status: 'active',
        latitude: 10,
        longitude: 20,
        created_at: '2026-11-02T00:00:00Z',
        updated_at: '2026-11-02T00:00:00Z',
      },
    ];

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ data: mockEntities }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const result = await entitiesApi.getAll();
    expect(result).toEqual(mockEntities);
    expect(fetch).toHaveBeenCalledWith('/api/entities', {
      method: 'GET',
      headers: { Accept: 'application/json' },
    });
  });

  it('creates an entity sending only allowed payload fields', async () => {
    const input: EntityInput = {
      name: '  Van 1  ',
      type: 'vehicle',
      description: 'Logistics',
      status: 'active',
      latitude: -6.2,
      longitude: 106.8,
    };

    const mockCreated = {
      id: '22222222-2222-2222-2222-222222222222',
      name: 'Van 1',
      type: 'vehicle',
      description: 'Logistics',
      status: 'active',
      latitude: -6.2,
      longitude: 106.8,
      created_at: '2026-11-02T00:00:00Z',
      updated_at: '2026-11-02T00:00:00Z',
    };

    let sentBody: string | undefined;
    vi.spyOn(globalThis, 'fetch').mockImplementationOnce(async (_url, init) => {
      sentBody = init?.body as string;
      return new Response(JSON.stringify({ data: mockCreated }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      });
    });

    const result = await entitiesApi.create(input);
    expect(result).toEqual(mockCreated);
    expect(sentBody).toBeDefined();

    const parsed = JSON.parse(sentBody!);
    expect(parsed).toEqual({
      name: 'Van 1', // trimmed
      type: 'vehicle',
      description: 'Logistics',
      status: 'active',
      latitude: -6.2,
      longitude: 106.8,
    });
    // Ensure no unexpected fields are present
    expect(Object.keys(parsed)).toEqual([
      'name',
      'type',
      'description',
      'status',
      'latitude',
      'longitude',
    ]);
  });

  it('updates an entity sending only allowed payload fields', async () => {
    const id = '22222222-2222-2222-2222-222222222222';
    const input: EntityInput = {
      name: 'Updated Van',
      type: 'vehicle',
      description: 'Updated',
      status: 'maintenance',
      latitude: -6.5,
      longitude: 107.0,
    };

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ data: { id, ...input } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const result = await entitiesApi.update(id, input);
    expect(result.id).toBe(id);
    expect(fetch).toHaveBeenCalledWith(`/api/entities/${id}`, expect.objectContaining({
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
    }));
  });

  it('deletes an entity handling 204 No Content', async () => {
    const id = '22222222-2222-2222-2222-222222222222';

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(null, {
        status: 204,
      })
    );

    await expect(entitiesApi.delete(id)).resolves.toBeUndefined();
    expect(fetch).toHaveBeenCalledWith(`/api/entities/${id}`, {
      method: 'DELETE',
      headers: { Accept: 'application/json' },
    });
  });

  it('throws ApiError with field and code on 400 validation error', async () => {
    const input: EntityInput = {
      name: 'Test',
      type: 'vehicle',
      description: '',
      status: 'active',
      latitude: 95, // invalid
      longitude: 100,
    };

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: 'VALIDATION_ERROR',
            message: 'latitude must be a finite number between -90 and 90',
            field: 'latitude',
          },
        }),
        {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }
      )
    );

    await expect(entitiesApi.create(input)).rejects.toMatchObject({
      name: 'ApiError',
      status: 400,
      code: 'VALIDATION_ERROR',
      message: 'latitude must be a finite number between -90 and 90',
      field: 'latitude',
    });
  });

  it('queries paginated entities with correct query string construction', async () => {
    const mockPage = {
      data: [
        {
          id: '11111111-1111-1111-1111-111111111111',
          name: 'Device A',
          type: 'iot_device',
          description: 'Sensor',
          status: 'active',
          latitude: 10,
          longitude: 20,
          created_at: '2026-11-02T00:00:00Z',
          updated_at: '2026-11-02T00:00:00Z',
        },
      ],
      meta: {
        limit: 25,
        next_cursor: 'cursor123',
        matching_count: 1,
        status_counts: { active: 1, inactive: 0, maintenance: 0 },
      },
    };

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify(mockPage), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const result = await entitiesApi.query({
      q: '  Sensor  ',
      type: 'iot_device',
      status: 'active',
      limit: 25,
      cursor: 'cur0',
    });

    expect(result).toEqual(mockPage);
    expect(fetch).toHaveBeenCalledWith(
      '/api/entities/query?q=Sensor&type=iot_device&status=active&limit=25&cursor=cur0',
      {
        method: 'GET',
        headers: { Accept: 'application/json' },
      }
    );
  });

  it('queries map markers with bounds and handles 413 limit', async () => {
    const mockMarkers = [
      {
        id: '11111111-1111-1111-1111-111111111111',
        name: 'Device A',
        type: 'iot_device',
        status: 'active',
        latitude: 10,
        longitude: 20,
      },
    ];

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ data: mockMarkers }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const result = await entitiesApi.getMapEntities({
      west: 10,
      south: -5,
      east: 30,
      north: 15,
      q: 'Device',
      type: 'iot_device',
    });

    expect(result).toEqual(mockMarkers);
    expect(fetch).toHaveBeenCalledWith(
      '/api/entities/map?west=10&south=-5&east=30&north=15&q=Device&type=iot_device',
      {
        method: 'GET',
        headers: { Accept: 'application/json' },
      }
    );

    // 413 MAP_RESULT_LIMIT
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          error: {
            code: 'MAP_RESULT_LIMIT',
            message: 'More than 5000 entities match this map view; zoom in to see markers',
          },
        }),
        {
          status: 413,
          headers: { 'Content-Type': 'application/json' },
        }
      )
    );

    await expect(
      entitiesApi.getMapEntities({ west: -180, south: -90, east: 180, north: 90 })
    ).rejects.toMatchObject({
      name: 'ApiError',
      status: 413,
      code: 'MAP_RESULT_LIMIT',
      message: 'More than 5000 entities match this map view; zoom in to see markers',
    });
  });

  it('fetches map extent across all filtered entities', async () => {
    const mockExtent = {
      count: 2,
      bounds: { west: 10, south: -5, east: 25, north: 12 },
    };

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ data: mockExtent }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const result = await entitiesApi.getMapExtent({ type: 'vehicle', status: 'active' });
    expect(result).toEqual(mockExtent);
    expect(fetch).toHaveBeenCalledWith('/api/entities/map/extent?type=vehicle&status=active', {
      method: 'GET',
      headers: { Accept: 'application/json' },
    });
  });
});
