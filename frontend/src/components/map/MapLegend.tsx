import { getEntityIconSvg } from './markerIcons';

interface MapLegendProps {
  isOpen: boolean;
  onClose: () => void;
}

export function MapLegend({ isOpen, onClose }: MapLegendProps) {
  if (!isOpen) {
    return null;
  }

  return (
    <div
      id="map-legend-panel"
      className="map-legend-panel animate-fade-in"
      role="region"
      aria-label="Map Visual Legend"
      onClick={(e) => e.stopPropagation()}
      onMouseDown={(e) => e.stopPropagation()}
      onDoubleClick={(e) => e.stopPropagation()}
    >
      <div className="legend-header">
        <div className="legend-title-row">
          <svg className="legend-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="8" y1="6" x2="21" y2="6"/>
            <line x1="8" y1="12" x2="21" y2="12"/>
            <line x1="8" y1="18" x2="21" y2="18"/>
            <line x1="3" y1="6" x2="3.01" y2="6"/>
            <line x1="3" y1="12" x2="3.01" y2="12"/>
            <line x1="3" y1="18" x2="3.01" y2="18"/>
          </svg>
          <span className="legend-title">Map Legend</span>
        </div>
        <button
          type="button"
          className="legend-close-btn"
          onClick={(e) => {
            e.stopPropagation();
            onClose();
          }}
          aria-label="Close Map Legend"
        >
          &times;
        </button>
      </div>

      <div className="legend-content">
        {/* Entity Types */}
        <div className="legend-section">
          <h5 className="legend-section-title">Entity Types</h5>
          <div className="legend-grid">
            <div className="legend-item">
              <span className="legend-symbol type-symbol" dangerouslySetInnerHTML={{ __html: getEntityIconSvg('vehicle') }} />
              <span className="legend-label">Vehicle</span>
            </div>
            <div className="legend-item">
              <span className="legend-symbol type-symbol" dangerouslySetInnerHTML={{ __html: getEntityIconSvg('iot_device') }} />
              <span className="legend-label">IoT Device</span>
            </div>
            <div className="legend-item">
              <span className="legend-symbol type-symbol" dangerouslySetInnerHTML={{ __html: getEntityIconSvg('facility') }} />
              <span className="legend-label">Facility</span>
            </div>
            <div className="legend-item">
              <span className="legend-symbol type-symbol" dangerouslySetInnerHTML={{ __html: getEntityIconSvg('other') }} />
              <span className="legend-label">Other</span>
            </div>
          </div>
        </div>

        {/* Entity Statuses */}
        <div className="legend-section">
          <h5 className="legend-section-title">Status Indicators</h5>
          <div className="legend-grid">
            <div className="legend-item">
              <span className="legend-symbol status-symbol status-dot-active" />
              <span className="legend-label">Active</span>
            </div>
            <div className="legend-item">
              <span className="legend-symbol status-symbol status-dot-maintenance" />
              <span className="legend-label">Maintenance</span>
            </div>
            <div className="legend-item">
              <span className="legend-symbol status-symbol status-dot-inactive" />
              <span className="legend-label">Inactive</span>
            </div>
          </div>
        </div>

        {/* Map Symbols & Layers */}
        <div className="legend-section">
          <h5 className="legend-section-title">Map Symbols</h5>
          <div className="legend-list">
            <div className="legend-item">
              <span className="legend-symbol halo-preview" />
              <div className="legend-text-col">
                <span className="legend-label">Selected Entity</span>
                <span className="legend-desc">Highlighted with blue pulsating halo</span>
              </div>
            </div>

            <div className="legend-item">
              <div className="geo-cluster-bubble cluster-small legend-cluster-preview">
                <span className="cluster-count">4</span>
              </div>
              <div className="legend-text-col">
                <span className="legend-label">Entity Cluster</span>
                <span className="legend-desc">Group of nearby saved entities; click to expand</span>
              </div>
            </div>

            <div className="legend-item">
              <div className="user-location-marker legend-user-preview">
                <div className="user-location-dot" />
              </div>
              <div className="legend-text-col">
                <span className="legend-label">Your Location</span>
                <span className="legend-desc">Ephemeral browser location with accuracy circle</span>
              </div>
            </div>

            <div className="legend-item">
              <div className="marker-pin draft-pin legend-draft-preview">
                <span className="marker-icon">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                    <circle cx="12" cy="12" r="10"/><line x1="22" y1="12" x2="18" y2="12"/><line x1="6" y1="12" x2="2" y2="12"/>
                  </svg>
                </span>
              </div>
              <div className="legend-text-col">
                <span className="legend-label">Draft Location</span>
                <span className="legend-desc">Unsaved coordinate placement (drag to adjust)</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
