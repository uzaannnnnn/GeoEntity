import { useState, useEffect, useRef } from 'react';
import { EntityListFilters } from '../EntityList';
import { ImportResult } from '../../types/dataExchange';
import { ExportPanel } from './ExportPanel';
import { ImportPanel } from './ImportPanel';
import { DownloadIcon, UploadIcon, CloseIcon, DataExchangeIcon } from '../Icons';

interface DataExchangeModalProps {
  isOpen: boolean;
  onClose: () => void;
  activeFilters: EntityListFilters;
  matchingCount: number;
  onImportSuccess: (result: ImportResult) => void;
}

export function DataExchangeModal({
  isOpen,
  onClose,
  activeFilters,
  matchingCount,
  onImportSuccess,
}: DataExchangeModalProps) {
  const [activeTab, setActiveTab] = useState<'export' | 'import'>('export');
  const modalRef = useRef<HTMLDivElement | null>(null);

  // Close on Escape key
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return (
    <div
      className="modal-overlay data-exchange-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="data-exchange-modal-title"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          onClose();
        }
      }}
    >
      <div className="modal-dialog data-exchange-dialog" ref={modalRef}>
        {/* Modal Header */}
        <div className="modal-header">
          <div className="modal-title-group">
            <div className="modal-title-icon">
              <DataExchangeIcon size={20} />
            </div>
            <div>
              <h2 id="data-exchange-modal-title" className="modal-title">
                Data Operations
              </h2>
              <span className="modal-subtitle">
                Export and import spatial entities via CSV or RFC 7946 GeoJSON
              </span>
            </div>
          </div>

          <button
            type="button"
            className="btn btn-ghost btn-icon-only modal-close-btn"
            onClick={onClose}
            aria-label="Close modal"
            title="Close dialog (Esc)"
          >
            <CloseIcon size={18} />
          </button>
        </div>

        {/* Tab Navigation */}
        <div className="exchange-tab-nav" role="tablist" aria-label="Data Operations Tabs">
          <button
            type="button"
            className={`exchange-tab-btn ${activeTab === 'export' ? 'active' : ''}`}
            onClick={() => setActiveTab('export')}
            role="tab"
            aria-selected={activeTab === 'export'}
            id="tab-export"
            aria-controls="panel-export"
          >
            <DownloadIcon size={16} />
            <span>Export Data</span>
          </button>
          <button
            type="button"
            className={`exchange-tab-btn ${activeTab === 'import' ? 'active' : ''}`}
            onClick={() => setActiveTab('import')}
            role="tab"
            aria-selected={activeTab === 'import'}
            id="tab-import"
            aria-controls="panel-import"
          >
            <UploadIcon size={16} />
            <span>Import Data</span>
          </button>
        </div>

        {/* Modal Body */}
        <div className="modal-body data-exchange-body">
          {activeTab === 'export' && (
            <div
              id="panel-export"
              role="tabpanel"
              aria-labelledby="tab-export"
              tabIndex={0}
            >
              <ExportPanel
                activeFilters={activeFilters}
                matchingCount={matchingCount}
              />
            </div>
          )}

          {activeTab === 'import' && (
            <div
              id="panel-import"
              role="tabpanel"
              aria-labelledby="tab-import"
              tabIndex={0}
            >
              <ImportPanel
                onImportSuccess={onImportSuccess}
                onDone={onClose}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
