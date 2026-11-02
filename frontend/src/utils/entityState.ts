import { Entity } from '../types/entity';

/** Insert or replace one entity while collapsing any existing duplicate IDs. */
export function upsertEntity(entities: Entity[], updated: Entity): Entity[] {
  const index = entities.findIndex((entity) => entity.id === updated.id);
  if (index === -1) return [...entities, updated];

  const result = entities.filter((entity) => entity.id !== updated.id);
  result.splice(Math.min(index, result.length), 0, updated);
  return result;
}

export function removeEntity(entities: Entity[], id: string): Entity[] {
  return entities.filter((entity) => entity.id !== id);
}
