import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { GeofenceEventTimeline } from './GeofenceEventTimeline';
import { GeofenceDefinition } from '../../types/geofence';
import { geofencesApi } from '../../api/geofences';

vi.mock('../../api/geofences', () => {
  return {
    geofencesApi: {
      getGeofenceEvents: vi.fn(),
    },
  };
});

const sampleGeofence: GeofenceDefinition = {
  id: 'gf-circle-99',
  name: 'Central Warehouse Compound',
  enabled: true,
  geometry: {
    type: 'Circle',
    center: [106.8456, -6.2088],
    radius_m: 800,
  },
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};

describe('GeofenceEventTimeline component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders header with geofence name and semantic disclaimer notice', () => {
    vi.mocked(geofencesApi.getGeofenceEvents).mockReturnValue(new Promise(() => {}));

    const markup = renderToStaticMarkup(
      <GeofenceEventTimeline geofence={sampleGeofence} />
    );

    expect(markup).toContain('Historical Transition Log');
    expect(markup).toContain('Events represent spatial transitions computed on saved entity coordinate changes');
    expect(markup).toContain('not live GPS or real-time device telemetry');
  });

  it('renders filter controls for entity uuid, start date, and end date', () => {
    vi.mocked(geofencesApi.getGeofenceEvents).mockReturnValue(new Promise(() => {}));

    const markup = renderToStaticMarkup(
      <GeofenceEventTimeline geofence={sampleGeofence} />
    );

    expect(markup).toContain('Entity UUID');
    expect(markup).toContain('filter-entity-id');
    expect(markup).toContain('From Date');
    expect(markup).toContain('To Date');
    expect(markup).toContain('Apply Filters');
  });
});
