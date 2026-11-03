import { describe, expect, it, vi } from 'vitest';
import L from 'leaflet';
import { canZoomToShowMarker } from './mapMarkerReveal';

describe('canZoomToShowMarker', () => {
  const marker = {} as L.Marker;

  it('rejects missing groups, markers, or unregistered marker layers', () => {
    const group = {
      zoomToShowLayer: vi.fn(),
      hasLayer: vi.fn(() => false),
    } as unknown as L.MarkerClusterGroup;

    expect(canZoomToShowMarker(null, marker)).toBe(false);
    expect(canZoomToShowMarker(group, null)).toBe(false);
    expect(canZoomToShowMarker(group, marker)).toBe(false);
  });

  it('allows zoomToShowLayer only after the cluster group owns the marker', () => {
    const group = {
      zoomToShowLayer: vi.fn(),
      hasLayer: vi.fn(() => true),
    } as unknown as L.MarkerClusterGroup;

    expect(canZoomToShowMarker(group, marker)).toBe(true);
  });
});
