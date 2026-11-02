import { describe, expect, it } from 'vitest';
import { Entity } from '../types/entity';
import { removeEntity, upsertEntity } from './entityState';

const entity = (id: string, latitude: number, longitude: number): Entity => ({
  id,
  name: `Entity ${id}`,
  type: 'facility',
  description: '',
  status: 'active',
  latitude,
  longitude,
  created_at: '2026-11-02T00:00:00Z',
  updated_at: '2026-11-02T00:00:00Z',
});

describe('entity state updates after successful mutations', () => {
  it('inserts a created entity without duplicating it', () => {
    const first = entity('one', 1, 2);
    const created = entity('two', 3, 4);
    const once = upsertEntity([first], created);
    const twice = upsertEntity(once, created);

    expect(once).toEqual([first, created]);
    expect(twice).toEqual([first, created]);
  });

  it('replaces an updated entity and its coordinates, collapsing duplicate IDs', () => {
    const before = entity('one', 1, 2);
    const other = entity('two', 5, 6);
    const updated = entity('one', -7.1, 107.2);

    expect(upsertEntity([before, other, before], updated)).toEqual([updated, other]);
  });

  it('removes a deleted entity and leaves unrelated entries intact', () => {
    const keep = entity('keep', 0, 0);
    const remove = entity('remove', 1, 1);

    expect(removeEntity([keep, remove], remove.id)).toEqual([keep]);
  });
});
