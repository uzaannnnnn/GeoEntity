import { describe, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { ActivityIcon, GeofenceIcon } from '../Icons';
import { GeofenceEmptyState } from './GeofenceEmptyState';

describe('GeofenceEmptyState', () => {
  it('renders matching compact empty-state structure with an optional action', () => {
    const markup = renderToStaticMarkup(
      <GeofenceEmptyState
        icon={<GeofenceIcon size={32} />}
        title="No Geofences Created"
        description="Create a boundary to monitor entity containment."
        actionLabel="Create Geofence"
        onAction={vi.fn()}
      />
    );

    expect(markup).toContain('class="geofence-empty-state"');
    expect(markup).toContain('class="empty-icon-wrap" aria-hidden="true"');
    expect(markup).toContain('No Geofences Created');
    expect(markup).toContain('Create Geofence');
  });

  it('renders event history empty state without a create action', () => {
    const markup = renderToStaticMarkup(
      <GeofenceEmptyState
        icon={<ActivityIcon size={32} />}
        title="No Transition Events"
        description="No spatial transition events have been recorded."
      />
    );

    expect(markup).toContain('class="geofence-empty-state"');
    expect(markup).toContain('No Transition Events');
    expect(markup).not.toContain('<button');
  });
});
