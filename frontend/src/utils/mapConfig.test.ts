import { describe, it, expect } from 'vitest';
import {
  DEFAULT_MAP_TILE_URL,
  DEFAULT_MAP_ATTRIBUTION,
  DEFAULT_MAP_MAX_ZOOM,
  getMapTileConfig,
} from './mapConfig';

describe('mapConfig', () => {
  it('uses official OpenStreetMap standard tile URL without subdomain template by default', () => {
    expect(DEFAULT_MAP_TILE_URL).toBe('https://tile.openstreetmap.org/{z}/{x}/{y}.png');
    expect(DEFAULT_MAP_TILE_URL).not.toContain('{s}');
  });

  it('includes required visible OpenStreetMap contributors attribution', () => {
    expect(DEFAULT_MAP_ATTRIBUTION).toContain('OpenStreetMap');
    expect(DEFAULT_MAP_ATTRIBUTION).toContain('contributors');
    expect(DEFAULT_MAP_ATTRIBUTION).toContain('https://www.openstreetmap.org/copyright');
  });

  it('resolves defaults when environment variables are empty or missing', () => {
    const config = getMapTileConfig({});

    expect(config.url).toBe(DEFAULT_MAP_TILE_URL);
    expect(config.attribution).toBe(DEFAULT_MAP_ATTRIBUTION);
    expect(config.maxZoom).toBe(DEFAULT_MAP_MAX_ZOOM);
  });

  it('handles whitespace-only environment variables by falling back to defaults', () => {
    const config = getMapTileConfig({
      VITE_MAP_TILE_URL: '   ',
      VITE_MAP_ATTRIBUTION: '  ',
      VITE_MAP_MAX_ZOOM: '  ',
    });

    expect(config.url).toBe(DEFAULT_MAP_TILE_URL);
    expect(config.attribution).toBe(DEFAULT_MAP_ATTRIBUTION);
    expect(config.maxZoom).toBe(DEFAULT_MAP_MAX_ZOOM);
  });

  it('allows custom tile URL override via environment variables', () => {
    const customUrl = 'https://tiles.example.com/{z}/{x}/{y}.png';
    const config = getMapTileConfig({
      VITE_MAP_TILE_URL: customUrl,
    });

    expect(config.url).toBe(customUrl);
    expect(config.attribution).toBe(DEFAULT_MAP_ATTRIBUTION);
    expect(config.maxZoom).toBe(DEFAULT_MAP_MAX_ZOOM);
  });

  it('allows custom attribution override via environment variables', () => {
    const customAttribution = '&copy; <a href="https://custom.provider">Custom Provider</a>';
    const config = getMapTileConfig({
      VITE_MAP_ATTRIBUTION: customAttribution,
    });

    expect(config.attribution).toBe(customAttribution);
  });

  it('parses valid numeric VITE_MAP_MAX_ZOOM and falls back for invalid values', () => {
    const validConfig = getMapTileConfig({
      VITE_MAP_MAX_ZOOM: '18',
    });
    expect(validConfig.maxZoom).toBe(18);

    const invalidConfig = getMapTileConfig({
      VITE_MAP_MAX_ZOOM: 'invalid-number',
    });
    expect(invalidConfig.maxZoom).toBe(DEFAULT_MAP_MAX_ZOOM);

    const zeroConfig = getMapTileConfig({
      VITE_MAP_MAX_ZOOM: '0',
    });
    expect(zeroConfig.maxZoom).toBe(DEFAULT_MAP_MAX_ZOOM);
  });
});
