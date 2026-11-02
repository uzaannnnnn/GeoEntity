import { apiClient } from './client';
import { ApiDataResponse, Entity, EntityInput } from '../types/entity';

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
