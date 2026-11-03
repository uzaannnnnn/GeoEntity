import type { ReactNode } from 'react';
import { PlusIcon } from '../Icons';

interface GeofenceEmptyStateProps {
  icon: ReactNode;
  title: string;
  description: string;
  actionLabel?: string;
  onAction?: () => void;
}

export function GeofenceEmptyState({
  icon,
  title,
  description,
  actionLabel,
  onAction,
}: GeofenceEmptyStateProps) {
  return (
    <div className="geofence-empty-state">
      <div className="empty-icon-wrap" aria-hidden="true">{icon}</div>
      <h4>{title}</h4>
      <p>{description}</p>
      {actionLabel && onAction && (
        <button type="button" className="btn btn-primary" onClick={onAction}>
          <PlusIcon size={16} />
          <span>{actionLabel}</span>
        </button>
      )}
    </div>
  );
}
