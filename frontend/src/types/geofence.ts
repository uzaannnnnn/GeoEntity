// Position follows RFC 7946 / GeoEntity convention: [longitude, latitude]
export type GeofencePosition = [number, number];

export interface CircleGeometry {
  type: 'Circle';
  center: GeofencePosition; // [longitude, latitude]
  radius_m: number;
}

export interface PolygonGeometry {
  type: 'Polygon';
  coordinates: GeofencePosition[][]; // GeoJSON format: [exterior_ring], ring has >= 4 points, first == last
}

export type GeofenceGeometry = CircleGeometry | PolygonGeometry;

export interface GeofenceDefinition {
  id: string;
  name: string;
  enabled: boolean;
  geometry: GeofenceGeometry;
  created_at: string;
  updated_at: string;
}

export interface GeofenceInput {
  name: string;
  enabled?: boolean;
  geometry: GeofenceGeometry;
}

export interface GeofenceFilters {
  enabled?: boolean;
  limit?: number;
  cursor?: string;
}

export interface GeofencePageMeta {
  limit: number;
  next_cursor: string;
}

export interface GeofencePage {
  data: GeofenceDefinition[];
  meta: GeofencePageMeta;
}

export type GeofenceMembershipState = 'inside' | 'outside' | 'inactive';

export interface EntityGeofenceMembership {
  geofence_id: string;
  geofence_name: string;
  state: GeofenceMembershipState;
  evaluated_latitude: number;
  evaluated_longitude: number;
  evaluated_at: string;
  definition_updated_at: string;
}

export type GeofenceEventType = 'enter' | 'exit';

export interface GeofenceEvent {
  id: string;
  entity_id: string;
  entity_name: string;
  geofence_id: string;
  geofence_name: string;
  event_type: GeofenceEventType;
  latitude: number;
  longitude: number;
  occurred_at: string;
  source: string; // 'coordinate_update'
}

export interface GeofenceEventFilters {
  entity_id?: string;
  from?: string; // RFC3339 timestamp
  to?: string;   // RFC3339 timestamp
  limit?: number;
  cursor?: string;
}

export interface GeofenceEventPageMeta {
  limit: number;
  next_cursor: string;
}

export interface GeofenceEventPage {
  data: GeofenceEvent[];
  meta: GeofenceEventPageMeta;
}
