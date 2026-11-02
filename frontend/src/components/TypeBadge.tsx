import { EntityType } from '../types/entity';
import { VehicleIcon, IoTDeviceIcon, FacilityIcon, OtherIcon } from './Icons';
import { formatTypeLabel } from '../utils/formatters';

interface TypeBadgeProps {
  type: EntityType;
  showIcon?: boolean;
  className?: string;
}

export function TypeBadge({ type, showIcon = true, className = '' }: TypeBadgeProps) {
  const renderIcon = () => {
    switch (type) {
      case 'vehicle':
        return <VehicleIcon size={14} />;
      case 'iot_device':
        return <IoTDeviceIcon size={14} />;
      case 'facility':
        return <FacilityIcon size={14} />;
      case 'other':
      default:
        return <OtherIcon size={14} />;
    }
  };

  return (
    <span className={`badge badge-type badge-type-${type} ${className}`}>
      {showIcon && <span className="badge-icon">{renderIcon()}</span>}
      <span className="badge-text">{formatTypeLabel(type)}</span>
    </span>
  );
}
