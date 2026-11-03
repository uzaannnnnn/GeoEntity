import { describe, it, expect, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { GeofenceManagerModal } from './GeofenceManagerModal';
import { GeofenceDefinition } from '../../types/geofence';

const sampleGeofences: GeofenceDefinition[] = [
  {
    id: 'gf-circle-1',
    name: 'North Port',
    enabled: true,
    geometry: {
      type: 'Circle',
      center: [106.8456, -6.2088],
      radius_m: 1000,
    },
    created_at: '2026-11-03T00:00:00Z',
    updated_at: '2026-11-03T00:00:00Z',
  },
];

describe('GeofenceManagerModal component', () => {
  it('renders modal dialog when isOpen is true', () => {
    const markup = renderToStaticMarkup(
      <GeofenceManagerModal
        isOpen={true}
        onClose={vi.fn()}
        geofences={sampleGeofences}
        selectedGeofence={sampleGeofences[0]}
        onSelectGeofence={vi.fn()}
        onRefreshGeofences={vi.fn().mockResolvedValue(undefined)}
      />
    );

    expect(markup).toContain('Geofence Operations &amp; Spatial History');
    expect(markup).toContain('Geofences (1)');
    expect(markup).toContain('Event History: North Port');
    expect(markup).toContain('New Geofence');
    expect(markup).toContain('North Port');
  });

  it('renders nothing when isOpen is false', () => {
    const markup = renderToStaticMarkup(
      <GeofenceManagerModal
        isOpen={false}
        onClose={vi.fn()}
        geofences={sampleGeofences}
        selectedGeofence={null}
        onSelectGeofence={vi.fn()}
        onRefreshGeofences={vi.fn().mockResolvedValue(undefined)}
      />
    );

    expect(markup).toBe('');
  });

  it('uses the shared Geofence empty state when Event History has no fences', () => {
    const markup = renderToStaticMarkup(
      <GeofenceManagerModal
        isOpen={true}
        onClose={vi.fn()}
        geofences={[]}
        selectedGeofence={null}
        onSelectGeofence={vi.fn()}
        onRefreshGeofences={vi.fn().mockResolvedValue(undefined)}
        initialTab="events"
      />
    );

    expect(markup).toContain('class="geofence-empty-state"');
    expect(markup).toContain('No Geofences Available');
  });
});
