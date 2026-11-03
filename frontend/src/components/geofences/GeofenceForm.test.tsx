import { describe, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { GeofenceDefinition } from '../../types/geofence';
import { GeofenceForm } from './GeofenceForm';

const polygonFence: GeofenceDefinition = {
  id: 'gf-polygon',
  name: 'North Polygon',
  enabled: true,
  geometry: {
    type: 'Polygon',
    coordinates: [[[106.81, -6.21], [106.83, -6.21], [106.83, -6.23], [106.81, -6.21]]],
  },
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};

describe('GeofenceForm', () => {
  it('renders shared input styling, aligned geometry choices, and circle controls', () => {
    const markup = renderToStaticMarkup(
      <GeofenceForm onSave={vi.fn()} onCancel={vi.fn()} mapCenter={{ lat: -6.2, lng: 106.8 }} />
    );

    expect(markup).toContain('Create New Geofence');
    expect(markup).toContain('class="form-input ');
    expect(markup).toContain('class="checkbox-toggle-label geofence-enabled-toggle"');
    expect(markup).toContain('class="coord-inputs-row"');
    expect(markup).toContain('aria-checked="true"');
    expect(markup).toContain('Center Latitude');
    expect(markup).toContain('Radius (meters)');
    expect(markup).toContain('Cancel');
  });

  it('renders polygon selection and coordinate editor for polygon edits', () => {
    const markup = renderToStaticMarkup(
      <GeofenceForm initialGeofence={polygonFence} onSave={vi.fn()} onCancel={vi.fn()} />
    );

    expect(markup).toContain('aria-checked="true"');
    expect(markup).toContain('Polygon Coordinates');
    expect(markup).toContain('class="form-textarea geofence-polygon-input ');
  });
});
