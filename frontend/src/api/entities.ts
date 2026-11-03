import { apiClient } from './client';
import {
  ApiDataResponse,
  Entity,
  EntityInput,
  EntityPageResponse,
  EntityQueryParams,
  EntityType,
  EntityStatus,
  MapEntity,
  MapQueryParams,
  MapExtentData,
} from '../types/entity';

export interface HealthStatus {
  status: string;
}

export const entitiesApi = {
  async getHealth(): Promise<HealthStatus> {
    const res = await apiClient.get<ApiDataResponse<HealthStatus>>('/health');
    return res.data;
  },

  async getAll(): Promise<Entity[]> {
    const res = await apiClient.get<ApiDataResponse<Entity[]>>('/entities');
    return res.data;
  },

  async query(params: EntityQueryParams = {}): Promise<EntityPageResponse> {
    const sp = new URLSearchParams();
    const q = params.q?.trim();
    if (q) sp.set('q', q);
    if (params.type && params.type !== 'all') sp.set('type', params.type);
    if (params.status && params.status !== 'all') sp.set('status', params.status);
    if (typeof params.limit === 'number' && params.limit > 0) sp.set('limit', String(params.limit));
    const cursor = params.cursor?.trim();
    if (cursor) sp.set('cursor', cursor);

    const qs = sp.toString();
    const path = qs ? `/entities/query?${qs}` : '/entities/query';
    return apiClient.get<EntityPageResponse>(path);
  },

  async getMapEntities(params: MapQueryParams): Promise<MapEntity[]> {
    const sp = new URLSearchParams();
    sp.set('west', String(params.west));
    sp.set('south', String(params.south));
    sp.set('east', String(params.east));
    sp.set('north', String(params.north));

    const q = params.q?.trim();
    if (q) sp.set('q', q);
    if (params.type && params.type !== 'all') sp.set('type', params.type);
    if (params.status && params.status !== 'all') sp.set('status', params.status);

    const res = await apiClient.get<ApiDataResponse<MapEntity[]>>(`/entities/map?${sp.toString()}`);
    return res.data;
  },

  async getMapExtent(filters?: {
    q?: string;
    type?: EntityType | 'all';
    status?: EntityStatus | 'all';
  }): Promise<MapExtentData> {
    const sp = new URLSearchParams();
    const q = filters?.q?.trim();
    if (q) sp.set('q', q);
    if (filters?.type && filters.type !== 'all') sp.set('type', filters.type);
    if (filters?.status && filters.status !== 'all') sp.set('status', filters.status);

    const qs = sp.toString();
    const path = qs ? `/entities/map/extent?${qs}` : '/entities/map/extent';
    const res = await apiClient.get<ApiDataResponse<MapExtentData>>(path);
    return res.data;
  },

  async getById(id: string): Promise<Entity> {
    const res = await apiClient.get<ApiDataResponse<Entity>>(`/entities/${id}`);
    return res.data;
  },

  async create(input: EntityInput): Promise<Entity> {
    // Only send the strict input fields accepted by backend
    const payload: EntityInput = {
      name: input.name.trim(),
      type: input.type,
      description: input.description ?? '',
      status: input.status,
      latitude: Number(input.latitude),
      longitude: Number(input.longitude),
    };
    const res = await apiClient.post<ApiDataResponse<Entity>>('/entities', payload);
    return res.data;
  },

  async update(id: string, input: EntityInput): Promise<Entity> {
    // Only send the strict input fields accepted by backend (disallowUnknownFields)
    const payload: EntityInput = {
      name: input.name.trim(),
      type: input.type,
      description: input.description ?? '',
      status: input.status,
      latitude: Number(input.latitude),
      longitude: Number(input.longitude),
    };
    const res = await apiClient.put<ApiDataResponse<Entity>>(`/entities/${id}`, payload);
    return res.data;
  },

  async delete(id: string): Promise<void> {
    await apiClient.delete(`/entities/${id}`);
  },
};
