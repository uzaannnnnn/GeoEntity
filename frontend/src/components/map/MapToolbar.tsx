import { GeolocationErrorDetails } from '../../utils/geolocation';

interface MapToolbarProps {
  validPointsCount: number;
  isLocating: boolean;
  geolocationError: GeolocationErrorDetails | null;
  toolbarMessage: string | null;
  isLegendOpen: boolean;
  isInspectorOpen: boolean;
  onFitAll: () => void;
  onLocateMe: () => void;
  onToggleLegend: () => void;
  onToggleInspector: () => void;
  onDismissMessage?: () => void;
}

export function MapToolbar({
  validPointsCount,
  isLocating,
  geolocationError,
  toolbarMessage,
  isLegendOpen,
  isInspectorOpen,
  onFitAll,
  onLocateMe,
  onToggleLegend,
  onToggleInspector,
  onDismissMessage,
}: MapToolbarProps) {
  const activeMessage = geolocationError?.message || toolbarMessage;

  return (
    <div
      className="map-toolbar-container"
      role="toolbar"
      aria-label="Map Tools"
      onClick={(e) => e.stopPropagation()}
      onMouseDown={(e) => e.stopPropagation()}
      onDoubleClick={(e) => e.stopPropagation()}
    >
      <div className="map-toolbar">
        {/* Fit All Entities Button */}
        <button
          type="button"
          className="map-tool-btn"
          onClick={(e) => {
            e.stopPropagation();
            onFitAll();
          }}
          disabled={validPointsCount === 0}
          title={
            validPointsCount > 0
              ? `Fit all entities (${validPointsCount} valid)`
              : 'Fit all disabled: no valid entity coordinates'
          }
          aria-label={
            validPointsCount > 0
              ? `Fit all entities (${validPointsCount} valid)`
              : 'Fit all disabled (no valid entity coordinates)'
          }
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="15 3 21 3 21 9"/>
            <polyline points="9 21 3 21 3 15"/>
            <line x1="21" y1="3" x2="14" y2="10"/>
            <line x1="3" y1="21" x2="10" y2="14"/>
          </svg>
          <span className="btn-label">Fit All</span>
        </button>

        {/* Locate Me Button */}
        <button
          type="button"
          className={`map-tool-btn ${isLocating ? 'is-loading' : ''}`}
          onClick={(e) => {
            e.stopPropagation();
            onLocateMe();
          }}
          disabled={isLocating}
          title="Locate my position (ephemeral browser location)"
          aria-label="Locate my current position"
        >
          {isLocating ? (
            <span className="spinner-icon" aria-hidden="true" />
          ) : (
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="3"/>
              <line x1="12" y1="2" x2="12" y2="6"/>
              <line x1="12" y1="18" x2="12" y2="22"/>
              <line x1="2" y1="12" x2="6" y2="12"/>
              <line x1="18" y1="12" x2="22" y2="12"/>
            </svg>
          )}
          <span className="btn-label">{isLocating ? 'Locating...' : 'Locate Me'}</span>
        </button>

        {/* Legend Toggle Button */}
        <button
          type="button"
          className={`map-tool-btn ${isLegendOpen ? 'is-active' : ''}`}
          onClick={(e) => {
            e.stopPropagation();
            onToggleLegend();
          }}
          aria-expanded={isLegendOpen}
          aria-controls="map-legend-panel"
          title="Toggle map visual legend"
          aria-label="Toggle map visual legend"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="8" y1="6" x2="21" y2="6"/>
            <line x1="8" y1="12" x2="21" y2="12"/>
            <line x1="8" y1="18" x2="21" y2="18"/>
            <line x1="3" y1="6" x2="3.01" y2="6"/>
            <line x1="3" y1="12" x2="3.01" y2="12"/>
            <line x1="3" y1="18" x2="3.01" y2="18"/>
          </svg>
          <span className="btn-label">Legend</span>
        </button>

        {/* Coordinate Inspector Toggle Button */}
        <button
          type="button"
          className={`map-tool-btn ${isInspectorOpen ? 'is-active' : ''}`}
          onClick={(e) => {
            e.stopPropagation();
            onToggleInspector();
          }}
          aria-expanded={isInspectorOpen}
          aria-controls="map-inspector-panel"
          title="Toggle coordinate inspector"
          aria-label="Toggle coordinate inspector"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="8" x2="12" y2="12"/>
            <line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          <span className="btn-label">Inspector</span>
        </button>
      </div>

      {/* Ephemeral Feedback/Error Pill */}
      {activeMessage && (
        <div
          className={`map-toolbar-feedback animate-fade-in ${geolocationError ? 'is-error' : 'is-info'}`}
          role="status"
          aria-live="polite"
        >
          <span className="feedback-text">{activeMessage}</span>
          {onDismissMessage && (
            <button
              type="button"
              className="feedback-dismiss-btn"
              onClick={(e) => {
                e.stopPropagation();
                onDismissMessage();
              }}
              aria-label="Dismiss message"
            >
              &times;
            </button>
          )}
        </div>
      )}
    </div>
  );
}
