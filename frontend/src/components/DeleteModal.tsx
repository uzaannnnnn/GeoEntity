import { Entity } from '../types/entity';
import { TrashIcon, CloseIcon, AlertIcon } from './Icons';

interface DeleteModalProps {
  entity: Entity | null;
  isOpen: boolean;
  isDeleting: boolean;
  error: string | null;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
}

export function DeleteModal({
  entity,
  isOpen,
  isDeleting,
  error,
  onConfirm,
  onCancel,
}: DeleteModalProps) {
  if (!isOpen || !entity) return null;

  return (
    <div className="modal-backdrop animate-fade-in" role="dialog" aria-modal="true">
      <div className="modal-dialog animate-scale-up">
        <div className="modal-header">
          <div className="modal-icon-wrapper danger-bg">
            <TrashIcon size={20} className="text-danger" />
          </div>
          <button
            type="button"
            className="modal-close-btn"
            onClick={onCancel}
            disabled={isDeleting}
            aria-label="Close dialog"
          >
            <CloseIcon size={16} />
          </button>
        </div>

        <div className="modal-body">
          <h3 className="modal-title">Delete Entity?</h3>
          <p className="modal-message">
            Are you sure you want to permanently delete{' '}
            <strong className="entity-highlight-name">&ldquo;{entity.name}&rdquo;</strong>?
          </p>
          <p className="modal-warning">
            This action will remove the record from SQLite and remove its marker from the map. This
            action cannot be undone.
          </p>

          {error && (
            <div className="form-alert form-alert-danger mt-3">
              <AlertIcon size={16} className="alert-icon" />
              <div className="alert-text">{error}</div>
            </div>
          )}
        </div>

        <div className="modal-footer">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onCancel}
            disabled={isDeleting}
          >
            Cancel
          </button>
          <button
            type="button"
            className="btn btn-danger"
            onClick={onConfirm}
            disabled={isDeleting}
          >
            {isDeleting ? (
              <span className="btn-loading-content">
                <span className="spinner-sm" />
                <span>Deleting...</span>
              </span>
            ) : (
              'Confirm Delete'
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
