import { useState, useEffect, useRef } from 'react';
import { GeofenceDefinition, GeofenceInput } from '../../types/geofence';
import { geofencesApi } from '../../api/geofences';
import { ApiError } from '../../api/client';
import { GeofenceList } from './GeofenceList';
import { GeofenceForm } from './GeofenceForm';
import { GeofenceEventTimeline } from './GeofenceEventTimeline';
import { GeofenceEmptyState } from './GeofenceEmptyState';
import {
  GeofenceIcon,
  ActivityIcon,
  CloseIcon,
  PlusIcon,
  AlertIcon,
  TrashIcon,
} from '../Icons';

export type GeofenceModalTab = 'list' | 'events' | 'create' | 'edit';

interface GeofenceManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  geofences: GeofenceDefinition[];
  selectedGeofence: GeofenceDefinition | null;
  onSelectGeofence: (geofence: GeofenceDefinition | null) => void;
  onRefreshGeofences: () => Promise<void>;
  mapCenter?: { lat: number; lng: number };
  onSelectEntity?: (entityId: string) => void;
  onToast?: (type: 'success' | 'error' | 'info', message: string, title?: string) => void;
  initialTab?: GeofenceModalTab;
}

export function GeofenceManagerModal({
  isOpen,
  onClose,
  geofences,
  selectedGeofence,
  onSelectGeofence,
  onRefreshGeofences,
  mapCenter,
  onSelectEntity,
  onToast = () => {},
  initialTab = 'list',
}: GeofenceManagerModalProps) {
  const [activeTab, setActiveTab] = useState<GeofenceModalTab>(initialTab);
  const [editingGeofence, setEditingGeofence] = useState<GeofenceDefinition | null>(null);

  // Deletion modal state
  const [geofenceToDelete, setGeofenceToDelete] = useState<GeofenceDefinition | null>(null);
  const [isDeleting, setIsDeleting] = useState<boolean>(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const modalRef = useRef<HTMLDivElement | null>(null);

  // Reset tab when modal opens or initialTab changes
  useEffect(() => {
    if (isOpen) {
      setActiveTab(initialTab);
    }
  }, [isOpen, initialTab]);

  // Handle Escape key
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (geofenceToDelete) {
          setGeofenceToDelete(null);
        } else {
          onClose();
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose, geofenceToDelete]);

  const handleCreateSave = async (input: GeofenceInput) => {
    const created = await geofencesApi.create(input);
    onToast('success', `Geofence "${created.name}" created successfully.`, 'Geofence Created');
    await onRefreshGeofences();
    onSelectGeofence(created);
    setActiveTab('list');
  };

  const handleEditSave = async (input: GeofenceInput) => {
    if (!editingGeofence) return;
    const updated = await geofencesApi.update(editingGeofence.id, input);
    onToast('success', `Geofence "${updated.name}" updated successfully.`, 'Geofence Updated');
    await onRefreshGeofences();
    onSelectGeofence(updated);
    setEditingGeofence(null);
    setActiveTab('list');
  };

  const handleToggleEnabled = async (g: GeofenceDefinition) => {
    try {
      const updated = await geofencesApi.update(g.id, {
        name: g.name,
        enabled: !g.enabled,
        geometry: g.geometry,
      });
      onToast(
        'info',
        `Geofence "${g.name}" ${updated.enabled ? 'enabled' : 'disabled'}.`,
        'Status Changed'
      );
      await onRefreshGeofences();
      if (selectedGeofence?.id === g.id) {
        onSelectGeofence(updated);
      }
    } catch (err) {
      if (err instanceof ApiError) {
        onToast('error', err.message, 'Failed to Change Status');
      } else {
        onToast('error', 'Network error while updating geofence status.', 'Error');
      }
    }
  };

  const handleConfirmDelete = async () => {
    if (!geofenceToDelete) return;

    setIsDeleting(true);
    setDeleteError(null);

    try {
      await geofencesApi.delete(geofenceToDelete.id);
      onToast(
        'success',
        `Geofence "${geofenceToDelete.name}" was deleted. Current memberships removed; historical event snapshots preserved.`,
        'Geofence Deleted'
      );
      if (selectedGeofence?.id === geofenceToDelete.id) {
        onSelectGeofence(null);
      }
      setGeofenceToDelete(null);
      await onRefreshGeofences();
    } catch (err) {
      if (err instanceof ApiError) {
        setDeleteError(err.message);
      } else {
        setDeleteError('Failed to delete geofence.');
      }
    } finally {
      setIsDeleting(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div
      className="modal-overlay geofence-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="geofence-modal-title"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          onClose();
        }
      }}
    >
      <div className="modal-dialog geofence-modal-dialog" ref={modalRef}>
        {/* Modal Header */}
        <div className="modal-header">
          <div className="modal-title-group">
            <div className="modal-title-icon">
              <GeofenceIcon size={20} />
            </div>
            <div>
              <h2 id="geofence-modal-title" className="modal-title">
                Geofence Operations &amp; Spatial History
              </h2>
              <span className="modal-subtitle">
                Configure spatial containment perimeters and inspect historical transition events
              </span>
            </div>
          </div>

          <button
            type="button"
            className="btn btn-ghost btn-icon-only modal-close-btn"
            onClick={onClose}
            aria-label="Close geofence operations dialog"
            title="Close dialog (Esc)"
          >
            <CloseIcon size={18} />
          </button>
        </div>

        {/* Tab Navigation */}
        <div className="geofence-modal-tabs" role="tablist" aria-label="Geofence Sections">
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'list'}
            id="tab-geofence-list"
            className={`geofence-tab-btn ${activeTab === 'list' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('list');
              setEditingGeofence(null);
            }}
          >
            <GeofenceIcon size={16} />
            <span>Geofences ({geofences.length})</span>
          </button>

          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'events'}
            id="tab-geofence-events"
            className={`geofence-tab-btn ${activeTab === 'events' ? 'active' : ''}`}
            onClick={() => setActiveTab('events')}
          >
            <ActivityIcon size={16} />
            <span>
              Event History
              {selectedGeofence ? `: ${selectedGeofence.name}` : ''}
            </span>
          </button>

          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'create' || activeTab === 'edit'}
            id="tab-geofence-form"
            className={`geofence-tab-btn ${activeTab === 'create' || activeTab === 'edit' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('create');
              setEditingGeofence(null);
            }}
          >
            <PlusIcon size={16} />
            <span>{activeTab === 'edit' ? 'Edit Geofence' : 'New Geofence'}</span>
          </button>
        </div>

        {/* Modal Body */}
        <div className="modal-body geofence-modal-body">
          {activeTab === 'list' && (
            <GeofenceList
              geofences={geofences}
              selectedGeofenceId={selectedGeofence?.id}
              onSelectGeofence={(g) => {
                onSelectGeofence(g);
              }}
              onToggleEnabled={handleToggleEnabled}
              onEditGeofence={(g) => {
                setEditingGeofence(g);
                setActiveTab('edit');
              }}
              onDeleteGeofence={(g) => {
                setGeofenceToDelete(g);
              }}
              onViewEvents={(g) => {
                onSelectGeofence(g);
                setActiveTab('events');
              }}
              onCreateClick={() => {
                setEditingGeofence(null);
                setActiveTab('create');
              }}
            />
          )}

          {activeTab === 'events' && (
            <div className="geofence-events-view">
              {geofences.length > 0 && (
                <div className="event-geofence-selector-bar">
                  <label htmlFor="select-geofence-event" className="form-label-sm">
                    Selected Geofence:
                  </label>
                  <select
                    id="select-geofence-event"
                    className="form-control-sm select-geofence-dropdown"
                    value={selectedGeofence?.id || geofences[0]?.id || ''}
                    onChange={(e) => {
                      const found = geofences.find((g) => g.id === e.target.value);
                      if (found) onSelectGeofence(found);
                    }}
                  >
                    {geofences.map((g) => (
                      <option key={g.id} value={g.id}>
                        {g.name} ({g.geometry.type}, {g.enabled ? 'Enabled' : 'Disabled'})
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {selectedGeofence || geofences[0] ? (
                <GeofenceEventTimeline
                  key={selectedGeofence?.id || geofences[0]?.id}
                  geofence={selectedGeofence || geofences[0]}
                  onSelectEntity={(entityId) => {
                    onSelectEntity?.(entityId);
                    onClose();
                  }}
                />
              ) : (
                <GeofenceEmptyState
                  icon={<ActivityIcon size={32} />}
                  title="No Geofences Available"
                  description="Create a geofence first to record and inspect transition events."
                  actionLabel="Create Geofence"
                  onAction={() => setActiveTab('create')}
                />
              )}
            </div>
          )}

          {(activeTab === 'create' || activeTab === 'edit') && (
            <GeofenceForm
              key={editingGeofence?.id || 'new'}
              initialGeofence={editingGeofence}
              mapCenter={mapCenter}
              onSave={editingGeofence ? handleEditSave : handleCreateSave}
              onCancel={() => {
                setEditingGeofence(null);
                setActiveTab('list');
              }}
              activeCount={geofences.filter((g) => g.enabled).length}
            />
          )}
        </div>
      </div>

      {/* Deletion Confirmation Dialog */}
      {geofenceToDelete && (
        <div
          className="modal-overlay delete-confirm-overlay"
          role="dialog"
          aria-modal="true"
          aria-labelledby="delete-geofence-title"
          onClick={(e) => {
            if (e.target === e.currentTarget && !isDeleting) {
              setGeofenceToDelete(null);
            }
          }}
        >
          <div className="modal-dialog modal-dialog-sm delete-confirm-dialog">
            <div className="modal-header">
              <div className="modal-title-group">
                <div className="modal-title-icon text-danger">
                  <TrashIcon size={20} />
                </div>
                <h3 id="delete-geofence-title" className="modal-title">
                  Delete Geofence
                </h3>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-icon-only"
                onClick={() => setGeofenceToDelete(null)}
                disabled={isDeleting}
                aria-label="Cancel delete"
              >
                <CloseIcon size={18} />
              </button>
            </div>

            <div className="modal-body">
              <p>
                Are you sure you want to permanently delete geofence{' '}
                <strong>"{geofenceToDelete.name}"</strong>?
              </p>

              <div className="alert-box alert-warning">
                <AlertIcon size={16} />
                <div className="alert-content">
                  <strong>Lifecycle Guarantee:</strong> Current entity containment memberships
                  for this geofence will be removed. Historical transition event snapshots will
                  survive in the append-only event log.
                </div>
              </div>

              {deleteError && (
                <div className="alert-box alert-error" role="alert">
                  <AlertIcon size={16} />
                  <div className="alert-content">{deleteError}</div>
                </div>
              )}
            </div>

            <div className="modal-footer">
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => setGeofenceToDelete(null)}
                disabled={isDeleting}
              >
                Cancel
              </button>
              <button
                type="button"
                className="btn btn-danger"
                onClick={handleConfirmDelete}
                disabled={isDeleting}
              >
                {isDeleting ? 'Deleting Geofence...' : 'Delete Geofence'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
