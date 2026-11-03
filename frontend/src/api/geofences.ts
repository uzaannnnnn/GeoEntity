import { apiClient } from './client';
import {
  GeofenceDefinition,
  GeofenceInput,
  GeofenceFilters,
  GeofencePage,
  EntityGeofenceMembership,
  GeofenceEventFilters,
  GeofenceEventPage,
} from '../types/geofence';
import { ApiDataResponse } from '../types/entity';

export const geofencesApi = {
  /**
   * List geofence definitions with optional enabled filter and keyset pagination.
   */
  async list(filters: GeofenceFilters = {}): Promise<GeofencePage> {
    const sp = new URLSearchParams();
    if (typeof filters.enabled === 'boolean') {
      sp.set('enabled', String(filters.enabled));
    }
    if (typeof filters.limit === 'number' && filters.limit > 0) {
      sp.set('limit', String(filters.limit));
    }
    const cursor = filters.cursor?.trim();
    if (cursor) {
      sp.set('cursor', cursor);
    }

    const qs = sp.toString();
    const path = qs ? `/geofences?${qs}` : '/geofences';
    return apiClient.get<GeofencePage>(path);
  },

  /**
   * Get a single geofence definition by ID.
   */
  async get(id: string): Promise<GeofenceDefinition> {
    const res = await apiClient.get<ApiDataResponse<GeofenceDefinition>>(`/geofences/${encodeURIComponent(id)}`);
    return res.data;
  },

  /**
   * Create a new geofence definition.
   */
  async create(input: GeofenceInput): Promise<GeofenceDefinition> {
    const res = await apiClient.post<ApiDataResponse<GeofenceDefinition>>('/geofences', input);
    return res.data;
  },

  /**
   * Replace/update an existing geofence definition.
   */
  async update(id: string, input: GeofenceInput): Promise<GeofenceDefinition> {
    const res = await apiClient.put<ApiDataResponse<GeofenceDefinition>>(
      `/geofences/${encodeURIComponent(id)}`,
      input
    );
    return res.data;
  },

  /**
   * Delete a geofence definition. Current memberships are removed; event history snapshots survive.
   */
  async delete(id: string): Promise<void> {
    await apiClient.delete<void>(`/geofences/${encodeURIComponent(id)}`);
  },

  /**
   * Get current geofence memberships for a specific entity.
   */
  async getEntityGeofences(entityId: string): Promise<EntityGeofenceMembership[]> {
    const res = await apiClient.get<ApiDataResponse<EntityGeofenceMembership[]>>(
      `/entities/${encodeURIComponent(entityId)}/geofences`
    );
    return res.data || [];
  },

  /**
   * Get append-only transition event history for a geofence with optional entity/time filters and keyset cursor.
   */
  async getGeofenceEvents(
    geofenceId: string,
    filters: GeofenceEventFilters = {}
  ): Promise<GeofenceEventPage> {
    const sp = new URLSearchParams();
    const entityId = filters.entity_id?.trim();
    if (entityId) {
      sp.set('entity_id', entityId);
    }
    const from = filters.from?.trim();
    if (from) {
      sp.set('from', from);
    }
    const to = filters.to?.trim();
    if (to) {
      sp.set('to', to);
    }
    if (typeof filters.limit === 'number' && filters.limit > 0) {
      sp.set('limit', String(filters.limit));
    }
    const cursor = filters.cursor?.trim();
    if (cursor) {
      sp.set('cursor', cursor);
    }

    const qs = sp.toString();
    const path = `/geofences/${encodeURIComponent(geofenceId)}/events${qs ? `?${qs}` : ''}`;
    return apiClient.get<GeofenceEventPage>(path);
  },
};
