import { useState, useCallback } from 'react';
import { formatCoordinateSixDecimals, formatCenterCopyText, copyTextToClipboard } from '../../utils/coordinateFormat';

interface CoordinateInspectorProps {
  center: { lat: number; lng: number };
  isOpen: boolean;
  onClose: () => void;
}

export function CoordinateInspector({
  center,
  isOpen,
  onClose,
}: CoordinateInspectorProps) {
  const [copyStatus, setCopyStatus] = useState<'idle' | 'success' | 'error'>('idle');
  const [announcement, setAnnouncement] = useState('');

  const handleCopy = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    const textToCopy = formatCenterCopyText(center.lat, center.lng);
    const success = await copyTextToClipboard(textToCopy);

    if (success) {
      setCopyStatus('success');
      setAnnouncement(`Copied coordinates ${textToCopy} to clipboard`);
      setTimeout(() => {
        setCopyStatus('idle');
        setAnnouncement('');
      }, 2500);
    } else {
      setCopyStatus('error');
      setAnnouncement('Failed to copy coordinates automatically. Please copy manually.');
      setTimeout(() => {
        setCopyStatus('idle');
      }, 3000);
    }
  }, [center.lat, center.lng]);

  if (!isOpen) {
    return null;
  }

  const formattedLat = formatCoordinateSixDecimals(center.lat);
  const formattedLng = formatCoordinateSixDecimals(center.lng);

  return (
    <div
      id="map-inspector-panel"
      className="coordinate-inspector-panel animate-fade-in"
      role="region"
      aria-label="Map Center Coordinate Inspector"
      onClick={(e) => e.stopPropagation()}
      onMouseDown={(e) => e.stopPropagation()}
      onDoubleClick={(e) => e.stopPropagation()}
    >
      <div className="inspector-header">
        <div className="inspector-title-row">
          <svg className="inspector-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="2" x2="12" y2="6"/>
            <line x1="12" y1="18" x2="12" y2="22"/>
            <line x1="2" y1="12" x2="6" y2="12"/>
            <line x1="18" y1="12" x2="22" y2="12"/>
          </svg>
          <span className="inspector-title">Map Center Coordinates</span>
        </div>
        <button
          type="button"
          className="inspector-close-btn"
          onClick={(e) => {
            e.stopPropagation();
            onClose();
          }}
          aria-label="Close Coordinate Inspector"
        >
          &times;
        </button>
      </div>

      <div className="inspector-body">
        <div className="coord-field">
          <span className="coord-label">Latitude:</span>
          <span className="coord-value" user-select="all">{formattedLat}</span>
        </div>
        <div className="coord-field">
          <span className="coord-label">Longitude:</span>
          <span className="coord-value" user-select="all">{formattedLng}</span>
        </div>
      </div>

      <div className="inspector-footer">
        <button
          type="button"
          className={`btn btn-xs ${copyStatus === 'success' ? 'btn-success' : 'btn-primary'} copy-coords-btn`}
          onClick={handleCopy}
          aria-label="Copy map center coordinates to clipboard"
        >
          {copyStatus === 'success' ? (
            <>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
              Copied!
            </>
          ) : copyStatus === 'error' ? (
            'Copy failed'
          ) : (
            <>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
              </svg>
              Copy Coordinates
            </>
          )}
        </button>
        <span className="inspector-hint">Format: latitude, longitude</span>
      </div>

      {/* Screen reader live region */}
      <span className="sr-only" role="status" aria-live="polite">
        {announcement}
      </span>
    </div>
  );
}
