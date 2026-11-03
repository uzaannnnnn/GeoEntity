import { describe, expect, it } from 'vitest';
import {
  appNavigationReducer,
  initialAppNavigationState,
  PrimaryOverlay,
} from './appNavigation';

describe('app navigation overlay transitions', () => {
  it.each<PrimaryOverlay>(['attribute-definitions', 'data-operations', 'geofences'])(
    'opens only the %s primary overlay',
    (overlay) => {
      expect(appNavigationReducer(initialAppNavigationState, { type: 'open-overlay', overlay })).toEqual({
        view: 'map',
        overlay,
      });
    }
  );

  it('replaces the current primary overlay when another opens', () => {
    const definitions = appNavigationReducer(initialAppNavigationState, {
      type: 'open-overlay',
      overlay: 'attribute-definitions',
    });

    expect(appNavigationReducer(definitions, { type: 'open-overlay', overlay: 'data-operations' })).toEqual({
      view: 'map',
      overlay: 'data-operations',
    });
    expect(appNavigationReducer(definitions, { type: 'open-overlay', overlay: 'geofences' })).toEqual({
      view: 'map',
      overlay: 'geofences',
    });
  });

  it.each<PrimaryOverlay>(['attribute-definitions', 'data-operations']) (
    'replaces an open geofence overlay with %s',
    (overlay) => {
      const geofences = appNavigationReducer(initialAppNavigationState, {
        type: 'open-overlay',
        overlay: 'geofences',
      });

      expect(appNavigationReducer(geofences, { type: 'open-overlay', overlay })).toEqual({
        view: 'map',
        overlay,
      });
    }
  );

  it('closes the active overlay and restores the map layout', () => {
    const geofences = appNavigationReducer(initialAppNavigationState, {
      type: 'open-overlay',
      overlay: 'geofences',
    });

    expect(appNavigationReducer(geofences, { type: 'close-overlay' })).toEqual({
      view: 'map',
      overlay: null,
    });
  });

  it('closes a primary overlay when navigating to Monitoring', () => {
    const geofences = appNavigationReducer(initialAppNavigationState, {
      type: 'open-overlay',
      overlay: 'geofences',
    });

    expect(appNavigationReducer(geofences, { type: 'toggle-monitoring' })).toEqual({
      view: 'monitoring',
      overlay: null,
    });
  });

  it('returns to the map without restoring a stale overlay', () => {
    const monitoring = { view: 'monitoring', overlay: null } as const;

    expect(appNavigationReducer(monitoring, { type: 'toggle-monitoring' })).toEqual({
      view: 'map',
      overlay: null,
    });
  });

  it('returns to the map when an overlay is opened from Monitoring', () => {
    const monitoring = { view: 'monitoring', overlay: null } as const;

    expect(
      appNavigationReducer(monitoring, { type: 'open-overlay', overlay: 'data-operations' })
    ).toEqual({ view: 'map', overlay: 'data-operations' });
  });
});
