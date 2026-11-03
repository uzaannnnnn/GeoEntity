export type EntityType = 'vehicle' | 'iot_device' | 'facility' | 'other';

export type EntityStatus = 'active' | 'inactive' | 'maintenance';

export interface Entity {
  id: string;
  name: string;
  type: EntityType;
  description: string;
  status: EntityStatus;
  latitude: number;
  longitude: number;
  created_at: string;
  updated_at: string;
}

export interface EntityInput {
  name: string;
  type: EntityType;
  description: string;
  status: EntityStatus;
  latitude: number;
  longitude: number;
}

export interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
    field?: string;
    details?: any[];
  };
}

export interface ApiDataResponse<T> {
  data: T;
}

export type SidePanelMode = 'list' | 'detail' | 'create' | 'edit';

export interface MapEntity {
  id: string;
  name: string;
  type: EntityType;
  status: EntityStatus;
  latitude: number;
  longitude: number;
}

export interface EntityQueryParams {
  q?: string;
  type?: EntityType | 'all';
  status?: EntityStatus | 'all';
  limit?: number;
  cursor?: string;
}

export interface EntityPageMeta {
  limit: number;
  next_cursor: string;
  matching_count: number;
  status_counts: Record<EntityStatus, number>;
}

export interface EntityPageResponse {
  data: Entity[];
  meta: EntityPageMeta;
}

export interface MapBoundsParams {
  west: number;
  south: number;
  east: number;
  north: number;
}

export interface MapQueryParams extends MapBoundsParams {
  q?: string;
  type?: EntityType | 'all';
  status?: EntityStatus | 'all';
}

export interface MapExtentBounds {
  west: number;
  south: number;
  east: number;
  north: number;
}

export interface MapExtentData {
  count: number;
  bounds: MapExtentBounds | null;
}
