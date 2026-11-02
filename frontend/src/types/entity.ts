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
  };
}

export interface ApiDataResponse<T> {
  data: T;
}

export type SidePanelMode = 'list' | 'detail' | 'create' | 'edit';
