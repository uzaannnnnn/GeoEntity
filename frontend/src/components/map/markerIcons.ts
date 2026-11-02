import L from 'leaflet';
import { EntityStatus, EntityType } from '../../types/entity';

/**
 * Returns inline SVG icon markup for each entity type.
 */
export function getEntityIconSvg(type: EntityType): string {
  switch (type) {
    case 'vehicle':
      return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 17h2c.6 0 1-.4 1-1v-3c0-.9-.7-1.7-1.5-1.9C18.7 10.6 16 10 16 10s-1.3-1.4-2.2-2.3c-.5-.4-1.1-.7-1.8-.7H5c-.6 0-1.1.4-1.4.9l-1.4 2.9A3.7 3.7 0 0 0 2 12v4c0 .6.4 1 1 1h2"/><circle cx="7" cy="17" r="2"/><path d="M9 17h6"/><circle cx="17" cy="17" r="2"/></svg>`;
    case 'iot_device':
      return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 1v3M15 1v3M9 20v3M15 20v3M20 9h3M20 14h3M1 9h3M1 14h3"/></svg>`;
    case 'facility':
      return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z"/><path d="M6 12H4a2 2 0 0 0-2 2v8h4"/><path d="M18 9h2a2 2 0 0 1 2 2v11h-4"/><path d="M10 6h4M10 10h4M10 14h4M10 18h4"/></svg>`;
    case 'other':
    default:
      return `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>`;
  }
}

/**
 * Creates custom DivIcon for persisted entities with status border, type icon, and selection pulse.
 */
export function createEntityDivIcon(type: EntityType, status: EntityStatus, isSelected: boolean): L.DivIcon {
  return L.divIcon({
    className: `geo-marker-wrapper ${isSelected ? 'marker-selected' : ''}`,
    html: `
      <div class="marker-pin marker-type-${type} status-border-${status}">
        <span class="marker-icon">${getEntityIconSvg(type)}</span>
        <span class="marker-status-dot status-dot-${status}"></span>
      </div>
      <div class="marker-tip"></div>
      ${isSelected ? '<div class="marker-pulse"></div>' : ''}
    `,
    iconSize: [36, 42],
    iconAnchor: [18, 42],
    popupAnchor: [0, -42],
    tooltipAnchor: [18, -21],
  });
}

/**
 * Creates custom DivIcon for draft location picker with crosshair icon and pulsating ripple.
 */
export function createDraftDivIcon(): L.DivIcon {
  return L.divIcon({
    className: 'geo-marker-wrapper marker-draft-wrapper',
    html: `
      <div class="marker-pin draft-pin">
        <span class="marker-icon">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><line x1="22" y1="12" x2="18" y2="12"/><line x1="6" y1="12" x2="2" y2="12"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="12" y1="22" x2="12" y2="18"/></svg>
        </span>
      </div>
      <div class="marker-tip draft-tip"></div>
      <div class="draft-ripple"></div>
    `,
    iconSize: [38, 44],
    iconAnchor: [19, 44],
    popupAnchor: [0, -44],
  });
}

/**
 * Creates custom DivIcon for current user geolocation dot with concentric pulse waves.
 */
export function createUserLocationDivIcon(): L.DivIcon {
  return L.divIcon({
    className: 'user-location-marker-wrapper',
    html: `
      <div class="user-location-marker" aria-label="Current location">
        <div class="user-location-pulse"></div>
        <div class="user-location-dot"></div>
      </div>
    `,
    iconSize: [28, 28],
    iconAnchor: [14, 14],
    tooltipAnchor: [0, -16],
  });
}

/**
 * Creates custom cluster bubble DivIcon with count formatting and size classification.
 */
export function createClusterDivIcon(cluster: L.MarkerCluster): L.DivIcon {
  const count = cluster.getChildCount();
  let sizeClass = 'cluster-small';
  if (count >= 100) {
    sizeClass = 'cluster-large';
  } else if (count >= 10) {
    sizeClass = 'cluster-medium';
  }

  return L.divIcon({
    html: `
      <div class="geo-cluster-bubble ${sizeClass}" aria-label="Cluster of ${count} entities">
        <span class="cluster-count">${count}</span>
      </div>
    `,
    className: 'geo-cluster-wrapper',
    iconSize: [40, 40],
    iconAnchor: [20, 20],
  });
}
