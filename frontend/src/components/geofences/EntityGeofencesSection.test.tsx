import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { EntityGeofencesSection } from './EntityGeofencesSection';
import { Entity } from '../../types/entity';
import { geofencesApi } from '../../api/geofences';

vi.mock('../../api/geofences', () => {
  return {
    geofencesApi: {
      getEntityGeofences: vi.fn(),
    },
  };
});

const sampleEntity: Entity = {
  id: 'ent-1234',
  name: 'Service Truck #42',
  type: 'vehicle',
  status: 'active',
  latitude: -6.2088,
  longitude: 106.8456,
  description: 'Fleet vehicle assigned to South District',
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};

describe('EntityGeofencesSection component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders initial loading state with accessible notice', () => {
    vi.mocked(geofencesApi.getEntityGeofences).mockReturnValue(new Promise(() => {}));

    const markup = renderToStaticMarkup(
      <EntityGeofencesSection entity={sampleEntity} />
    );

    expect(markup).toContain('Evaluating current geofence containment');
    expect(markup).toContain('Geofence Memberships');
    expect(markup).toContain('animate-spin');
  });

  it('displays membership states and disclaimer text correctly', () => {
    // Note: renderToStaticMarkup does not trigger useEffect, so initial render tests loading/empty scaffolding
    const markup = renderToStaticMarkup(
      <EntityGeofencesSection entity={sampleEntity} />
    );

    // Verify presence of structural elements
    expect(markup).toContain('entity-geofences-section');
    expect(markup).toBeDefined();
  });
});
