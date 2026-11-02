import { useMemo, useState } from 'react';
import { Entity, EntityStatus, EntityType } from '../types/entity';
import { StatusBadge } from './StatusBadge';
import { TypeBadge } from './TypeBadge';
import { SearchIcon, PlusIcon, CloseIcon, AlertIcon } from './Icons';
import { formatCoordinate } from '../utils/formatters';

interface EntityListProps {
  entities: Entity[];
  selectedEntity: Entity | null;
  isLoading: boolean;
  error: string | null;
  onSelectEntity: (entity: Entity) => void;
  onAddNewClick: () => void;
  onRetry: () => void;
}

export function EntityList({
  entities,
  selectedEntity,
  isLoading,
  error,
  onSelectEntity,
  onAddNewClick,
  onRetry,
}: EntityListProps) {
  const [searchTerm, setSearchTerm] = useState('');
  const [typeFilter, setTypeFilter] = useState<EntityType | 'all'>('all');
  const [statusFilter, setStatusFilter] = useState<EntityStatus | 'all'>('all');

  const filteredEntities = useMemo(() => {
    return entities.filter((entity) => {
      const matchSearch =
        !searchTerm.trim() ||
        entity.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        entity.description.toLowerCase().includes(searchTerm.toLowerCase());

      const matchType = typeFilter === 'all' || entity.type === typeFilter;
      const matchStatus = statusFilter === 'all' || entity.status === statusFilter;

      return matchSearch && matchType && matchStatus;
    });
  }, [entities, searchTerm, typeFilter, statusFilter]);

  const hasActiveFilters = searchTerm !== '' || typeFilter !== 'all' || statusFilter !== 'all';

  const clearFilters = () => {
    setSearchTerm('');
    setTypeFilter('all');
    setStatusFilter('all');
  };

  return (
    <div className="entity-list-panel">
      {/* Search and filters */}
      <div className="list-filters-header">
        <div className="search-bar">
          <SearchIcon size={16} className="search-icon" />
          <input
            type="text"
            className="search-input"
            placeholder="Search by name or description..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
          {searchTerm && (
            <button
              type="button"
              className="search-clear-btn"
              onClick={() => setSearchTerm('')}
              aria-label="Clear search"
            >
              <CloseIcon size={14} />
            </button>
          )}
        </div>

        <div className="filters-row">
          <select
            className="filter-select"
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value as EntityType | 'all')}
            aria-label="Filter by type"
          >
            <option value="all">All Types</option>
            <option value="vehicle">Vehicles</option>
            <option value="iot_device">IoT Devices</option>
            <option value="facility">Facilities</option>
            <option value="other">Other</option>
          </select>

          <select
            className="filter-select"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as EntityStatus | 'all')}
            aria-label="Filter by status"
          >
            <option value="all">All Statuses</option>
            <option value="active">Active</option>
            <option value="maintenance">Maintenance</option>
            <option value="inactive">Inactive</option>
          </select>

          {hasActiveFilters && (
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={clearFilters}
              title="Reset all filters"
            >
              Reset
            </button>
          )}
        </div>

        <div className="list-counts">
          <span>
            Showing <strong>{filteredEntities.length}</strong> of {entities.length} entities
          </span>
        </div>
      </div>

      {/* List body */}
      <div className="list-content">
        {isLoading && (
          <div className="skeleton-list">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="skeleton-card">
                <div className="skeleton-line skeleton-title" />
                <div className="skeleton-badges">
                  <div className="skeleton-badge" />
                  <div className="skeleton-badge" />
                </div>
                <div className="skeleton-line skeleton-desc" />
              </div>
            ))}
          </div>
        )}

        {!isLoading && error && (
          <div className="state-card state-error">
            <AlertIcon size={32} className="state-icon text-danger" />
            <h4 className="state-title">Failed to load entities</h4>
            <p className="state-message">{error}</p>
            <button type="button" className="btn btn-secondary btn-sm" onClick={onRetry}>
              Retry
            </button>
          </div>
        )}

        {!isLoading && !error && entities.length === 0 && (
          <div className="state-card state-empty">
            <div className="empty-icon-wrapper">
              <PlusIcon size={32} />
            </div>
            <h4 className="state-title">No Entities Found</h4>
            <p className="state-message">
              The database is currently empty. Click &ldquo;Add Entity&rdquo; to place your first
              marker on the map.
            </p>
            <button type="button" className="btn btn-primary" onClick={onAddNewClick}>
              <PlusIcon size={16} />
              <span>Create First Entity</span>
            </button>
          </div>
        )}

        {!isLoading && !error && entities.length > 0 && filteredEntities.length === 0 && (
          <div className="state-card state-empty">
            <SearchIcon size={32} className="state-icon text-muted" />
            <h4 className="state-title">No Matching Entities</h4>
            <p className="state-message">No entity matches the selected filters or search query.</p>
            <button type="button" className="btn btn-secondary btn-sm" onClick={clearFilters}>
              Reset Filters
            </button>
          </div>
        )}

        {!isLoading && !error && filteredEntities.length > 0 && (
          <div className="entity-cards-scroll">
            {filteredEntities.map((entity) => {
              const isSelected = selectedEntity?.id === entity.id;

              return (
                <div
                  key={entity.id}
                  className={`entity-card ${isSelected ? 'entity-card-selected' : ''}`}
                  onClick={() => onSelectEntity(entity)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      onSelectEntity(entity);
                    }
                  }}
                >
                  <div className="card-top">
                    <h3 className="card-title" title={entity.name}>
                      {entity.name}
                    </h3>
                  </div>

                  <div className="card-badges">
                    <TypeBadge type={entity.type} />
                    <StatusBadge status={entity.status} />
                  </div>

                  {entity.description && (
                    <p className="card-desc" title={entity.description}>
                      {entity.description}
                    </p>
                  )}

                  <div className="card-footer">
                    <span className="card-coordinates">
                      {formatCoordinate(entity.latitude)}, {formatCoordinate(entity.longitude)}
                    </span>
                    <span className="card-arrow">&rarr;</span>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
