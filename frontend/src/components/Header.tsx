import { LogoIcon, PlusIcon, RefreshIcon, AttributesIcon, DataExchangeIcon } from './Icons';

interface HeaderProps {
  totalCount: number;
  activeCount: number;
  maintenanceCount: number;
  inactiveCount: number;
  isHealthOk: boolean | null;
  isLoading: boolean;
  onAddClick: () => void;
  onRefreshClick: () => void;
  onManageDefinitionsClick?: () => void;
  onDataExchangeClick?: () => void;
  isCreateActive: boolean;
}

export function Header({
  totalCount,
  activeCount,
  maintenanceCount,
  inactiveCount,
  isHealthOk,
  isLoading,
  onAddClick,
  onRefreshClick,
  onManageDefinitionsClick,
  onDataExchangeClick,
  isCreateActive,
}: HeaderProps) {
  return (
    <header className="app-header">
      <div className="header-brand">
        <div className="header-logo-badge">
          <LogoIcon size={22} className="header-logo-icon" />
        </div>
        <div className="header-brand-text">
          <h1 className="header-title">GeoEntity</h1>
          <span className="header-tagline">Management System</span>
        </div>
      </div>

      <div className="header-stats">
        <div className="stat-pill stat-pill-total" title="Total Entities">
          <span className="stat-label">Total</span>
          <span className="stat-value">{totalCount}</span>
        </div>
        {totalCount > 0 && (
          <div className="stat-breakdown">
            <span className="breakdown-item breakdown-active" title="Active">
              <span className="status-dot-sm status-dot-active" />
              {activeCount}
            </span>
            <span className="breakdown-item breakdown-maint" title="Maintenance">
              <span className="status-dot-sm status-dot-maintenance" />
              {maintenanceCount}
            </span>
            <span className="breakdown-item breakdown-inact" title="Inactive">
              <span className="status-dot-sm status-dot-inactive" />
              {inactiveCount}
            </span>
          </div>
        )}
      </div>

      <div className="header-actions">
        <div
          className={`health-indicator ${
            isHealthOk === true ? 'health-ok' : isHealthOk === false ? 'health-err' : 'health-unknown'
          }`}
          title={
            isHealthOk === true
              ? 'Backend API Connected'
              : isHealthOk === false
              ? 'Backend API Unreachable'
              : 'Checking API health...'
          }
        >
          <span className="health-dot" />
          <span className="health-label">
            {isHealthOk === true ? 'API Online' : isHealthOk === false ? 'API Offline' : 'Connecting...'}
          </span>
        </div>

        {onManageDefinitionsClick && (
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onManageDefinitionsClick}
            title="Configure custom attribute definitions"
          >
            <AttributesIcon size={16} />
            <span>Attributes</span>
          </button>
        )}

        {onDataExchangeClick && (
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onDataExchangeClick}
            title="Export or import entities as CSV / GeoJSON"
            aria-label="Open Data Operations"
          >
            <DataExchangeIcon size={16} />
            <span>Data Operations</span>
          </button>
        )}

        <button
          type="button"
          className="btn btn-secondary btn-icon-only"
          onClick={onRefreshClick}
          disabled={isLoading}
          title="Refresh entities from backend"
          aria-label="Refresh entities"
        >
          <RefreshIcon size={16} className={isLoading ? 'animate-spin' : ''} />
        </button>

        <button
          type="button"
          className={`btn ${isCreateActive ? 'btn-highlight' : 'btn-primary'}`}
          onClick={onAddClick}
        >
          <PlusIcon size={16} />
          <span>Add Entity</span>
        </button>
      </div>
    </header>
  );
}
