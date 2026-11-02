import { describe, expect, it, vi } from 'vitest';
import type { Map as LeafletMap } from 'leaflet';
import { Entity } from '../types/entity';
import { flyToEntity } from './entityNavigation';

const selectedEntity: Entity = {
  id: 'entity-one',
  name: 'North Depot',
  type: 'facility',
  description: '',
  status: 'active',
  latitude: -6.2,
  longitude: 106.8,
  created_at: '2026-11-02T00:00:00Z',
  updated_at: '2026-11-02T00:00:00Z',
};

describe('flyToEntity', () => {
  it('recenters to the selected entity on every request, including repeated requests', () => {
    const flyTo = vi.fn();
    const map = { flyTo } as unknown as Pick<LeafletMap, 'flyTo'>;

    flyToEntity(map, selectedEntity);
    flyToEntity(map, selectedEntity);

    expect(flyTo).toHaveBeenCalledTimes(2);
    expect(flyTo).toHaveBeenLastCalledWith([-6.2, 106.8], 13, { duration: 1.2 });
  });

  it('does not navigate to invalid coordinates', () => {
    const flyTo = vi.fn();
    const map = { flyTo } as unknown as Pick<LeafletMap, 'flyTo'>;

    flyToEntity(map, { ...selectedEntity, latitude: Number.NaN });

    expect(flyTo).not.toHaveBeenCalled();
  });
});
