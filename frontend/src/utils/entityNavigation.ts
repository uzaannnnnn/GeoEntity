import type { Map as LeafletMap } from 'leaflet';
import { Entity } from '../types/entity';

export function flyToEntity(map: Pick<LeafletMap, 'flyTo'>, entity: Entity): void {
  if (!Number.isFinite(entity.latitude) || !Number.isFinite(entity.longitude)) return;
  map.flyTo([entity.latitude, entity.longitude], 13, { duration: 1.2 });
}
