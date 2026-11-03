import { describe, expect, it, vi } from 'vitest';
import L from 'leaflet';
import { MapCenterCallback, subscribeToMapCenter, updateMapCenter } from './mapCenter';

describe('map center reporting', () => {
  it('preserves state identity when coordinates are unchanged or within the threshold', () => {
    const previous = { lat: -6.2, lng: 106.8 };

    expect(updateMapCenter(previous, { ...previous })).toBe(previous);
    expect(updateMapCenter(previous, { lat: -6.2 + 0.5e-6, lng: 106.8 - 0.5e-6 })).toBe(previous);
  });

  it('returns a new center for meaningful coordinate changes', () => {
    const previous = { lat: -6.2, lng: 106.8 };
    const next = { lat: -6.19, lng: 106.81 };

    expect(updateMapCenter(previous, next)).toBe(next);
  });

  it('reports the initial center and subsequent moveend centers using the latest callback', () => {
    const listeners = new Set<() => void>();
    let center = { lat: -6.2, lng: 106.8 };
    const map = {
      getCenter: () => center,
      on: vi.fn((_event: string, listener: () => void) => listeners.add(listener)),
      off: vi.fn((_event: string, listener: () => void) => listeners.delete(listener)),
    } as unknown as L.Map;
    const initialCallback = vi.fn();
    const latestCallback = vi.fn();
    const callbackRef: { current: MapCenterCallback | undefined } = { current: initialCallback };

    const cleanup = subscribeToMapCenter(map, callbackRef);
    expect(initialCallback).toHaveBeenCalledTimes(1);
    expect(initialCallback).toHaveBeenCalledWith(-6.2, 106.8);
    expect(listeners.size).toBe(1);

    // A React rerender updates the ref, not the map subscription.
    callbackRef.current = latestCallback;
    center = { lat: -6.19, lng: 106.81 };
    for (const listener of listeners) listener();
    expect(latestCallback).toHaveBeenCalledTimes(1);
    expect(latestCallback).toHaveBeenCalledWith(-6.19, 106.81);
    expect(map.on).toHaveBeenCalledTimes(1);

    cleanup();
    expect(map.off).toHaveBeenCalledTimes(1);
    expect(listeners.size).toBe(0);

    // A later mount registers exactly one fresh listener.
    const remountCleanup = subscribeToMapCenter(map, callbackRef);
    expect(listeners.size).toBe(1);
    remountCleanup();
    expect(listeners.size).toBe(0);
  });
});
