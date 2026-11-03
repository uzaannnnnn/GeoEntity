import { describe, it, expect, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { GeofenceList } from './GeofenceList';
import { GeofenceDefinition } from '../../types/geofence';

const sampleGeofences: GeofenceDefinition[] = [
  {
    id: 'gf-1',
    name: 'Logistics Depot North',
    enabled: true,
    geometry: {
      type: 'Circle',
      center: [106.8456, -6.2088],
      radius_m: 1500,
    },
    created_at: '2026-11-03T01:00:00Z',
    updated_at: '2026-11-03T01:30:00Z',
  },
  {
    id: 'gf-2',
    name: 'Restricted Industrial Zone',
    enabled: false,
    geometry: {
      type: 'Polygon',
      coordinates: [
        [
          [106.80, -6.20],
          [106.85, -6.20],
          [106.85, -6.25],
          [106.80, -6.25],
          [106.80, -6.20],
        ],
      ],
    },
    created_at: '2026-11-02T10:00:00Z',
    updated_at: '2026-11-02T12:00:00Z',
  },
];

describe('GeofenceList component', () => {
  it('renders geofence cards with names, radius, and vertex count', () => {
    const markup = renderToStaticMarkup(
      <GeofenceList
        geofences={sampleGeofences}
        onSelectGeofence={vi.fn()}
        onToggleEnabled={vi.fn()}
        onEditGeofence={vi.fn()}
        onDeleteGeofence={vi.fn()}
        onViewEvents={vi.fn()}
        onCreateClick={vi.fn()}
      />
    );

    expect(markup).toContain('Logistics Depot North');
    expect(markup).toContain('Circle • 1.50 km');
    expect(markup).toContain('Restricted Industrial Zone');
    expect(markup).toContain('Polygon • 5 vertices');
    expect(markup).toContain('1 Active');
    expect(markup).toContain('1 Disabled');
  });

  it('indicates selected geofence with appropriate CSS class', () => {
    const markup = renderToStaticMarkup(
      <GeofenceList
        geofences={sampleGeofences}
        selectedGeofenceId="gf-1"
        onSelectGeofence={vi.fn()}
        onToggleEnabled={vi.fn()}
        onEditGeofence={vi.fn()}
        onDeleteGeofence={vi.fn()}
        onViewEvents={vi.fn()}
        onCreateClick={vi.fn()}
      />
    );

    expect(markup).toContain('geofence-card selected');
  });

  it('renders empty state when no geofences exist', () => {
    const markup = renderToStaticMarkup(
      <GeofenceList
        geofences={[]}
        onSelectGeofence={vi.fn()}
        onToggleEnabled={vi.fn()}
        onEditGeofence={vi.fn()}
        onDeleteGeofence={vi.fn()}
        onViewEvents={vi.fn()}
        onCreateClick={vi.fn()}
      />
    );

    expect(markup).toContain('No Geofences Created');
    expect(markup).toContain('Create Geofence');
    expect(markup).toContain('class="geofence-empty-state"');
  });
});
