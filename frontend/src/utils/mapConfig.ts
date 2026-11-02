export const DEFAULT_MAP_TILE_URL = 'https://tile.openstreetmap.org/{z}/{x}/{y}.png';

export const DEFAULT_MAP_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">OpenStreetMap</a> contributors';

export const DEFAULT_MAP_MAX_ZOOM = 19;

export interface MapTileConfig {
  url: string;
  attribution: string;
  maxZoom: number;
}

/**
 * Resolves map tile configuration from environment variables or sensible OSM defaults.
 *
 * Operational notes:
 * - Default tile provider: OpenStreetMap standard tile layer (tile.openstreetmap.org).
 * - The legacy `{s}.tile.openstreetmap.org` template is deprecated and replaced by `tile.openstreetmap.org`.
 * - The OSM Foundation operates standard tile servers as a best-effort public service without an SLA.
 * - Browsers supply standard request identification via the HTTP Referer header; do not introduce
 *   a restrictive referrer policy (such as `no-referrer`) that removes the Referer header.
 * - Browsers handle tile caching natively via standard HTTP caching headers; custom prefetching,
 *   scraping, or bulk offline downloading of standard OSM tiles is strictly prohibited by OSM policy.
 * - For public production or higher-volume use, configure a commercial or self-hosted tile provider
 *   with an appropriate SLA and terms of service via VITE_MAP_TILE_URL.
 */
export function getMapTileConfig(
  env: Record<string, string | undefined> = import.meta.env
): MapTileConfig {
  const rawUrl = env?.VITE_MAP_TILE_URL?.trim();
  const rawAttribution = env?.VITE_MAP_ATTRIBUTION?.trim();
  const rawMaxZoom = env?.VITE_MAP_MAX_ZOOM?.trim();

  const url = rawUrl && rawUrl.length > 0 ? rawUrl : DEFAULT_MAP_TILE_URL;
  const attribution = rawAttribution && rawAttribution.length > 0 ? rawAttribution : DEFAULT_MAP_ATTRIBUTION;

  const parsedZoom = rawMaxZoom ? parseInt(rawMaxZoom, 10) : NaN;
  const maxZoom = Number.isFinite(parsedZoom) && parsedZoom > 0 ? parsedZoom : DEFAULT_MAP_MAX_ZOOM;

  return {
    url,
    attribution,
    maxZoom,
  };
}
