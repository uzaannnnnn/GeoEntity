import { EntityStatus } from '../types/entity';
import { formatStatusLabel } from '../utils/formatters';

interface StatusBadgeProps {
  status: EntityStatus;
  showDot?: boolean;
  className?: string;
}

export function StatusBadge({ status, showDot = true, className = '' }: StatusBadgeProps) {
  return (
    <span className={`badge badge-status badge-status-${status} ${className}`}>
      {showDot && <span className="status-dot" />}
      <span className="badge-text">{formatStatusLabel(status)}</span>
    </span>
  );
}
