import { useState, useEffect, useRef } from 'react';
import { Entity, EntityStatus, EntityType } from '../types/entity';
import { StatusBadge } from './StatusBadge';
import { TypeBadge } from './TypeBadge';
import { SearchIcon, PlusIcon, CloseIcon, AlertIcon } from './Icons';
import { formatCoordinate } from '../utils/formatters';
import { hasActiveFilters } from '../utils/searchFilter';

export interface EntityListFilters {
  q: string;
  type: EntityType | 'all';
  status: EntityStatus | 'all';
}

export interface EntityListProps {
  entities: Entity[];
  selectedEntity: Entity | null;
  isLoading: boolean;
  isLoadingMore?: boolean;
  error: string | null;
  matchingCount: number;
  hasMore: boolean;
  activeFilters: EntityListFilters;
  onFilterChange: (filters: EntityListFilters) => void;
  onLoadMore: () => void;
  onSelectEntity: (entity: Entity) => void;
  onAddNewClick: () => void;
  onRetry: () => void;
}

export function EntityList({
  entities,
  selectedEntity,
  isLoading,
  isLoadingMore = false,
  error,
  matchingCount,
  hasMore,
  activeFilters,
  onFilterChange,
  onLoadMore,
  onSelectEntity,
  onAddNewClick,
  onRetry,
}: EntityListProps) {
  // Local immediate state for typing responsiveness
  const [searchTerm, setSearchTerm] = useState(activeFilters.q);
  const debounceTimerRef = useRef<NodeJS.Timeout | null>(null);
  const isInternalChangeRef = useRef(false);

  // Sync external filter changes (e.g. from reset) into local search input
  useEffect(() => {
    if (!isInternalChangeRef.current) {
      setSearchTerm(activeFilters.q);
    }
    isInternalChangeRef.current = false;
  }, [activeFilters.q]);

  // Clean up timer on unmount
  useEffect(() => {
    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
    };
  }, []);

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    isInternalChangeRef.current = true;

    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }

    debounceTimerRef.current = setTimeout(() => {
      onFilterChange({
        ...activeFilters,
        q: value,
      });
    }, 300);
  };

  const handleClearSearch = () => {
    setSearchTerm('');
    isInternalChangeRef.current = true;
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }
    onFilterChange({
      ...activeFilters,
      q: '',
    });
  };

  const handleTypeChange = (newType: EntityType | 'all') => {
    onFilterChange({
      ...activeFilters,
      type: newType,
    });
  };

  const handleStatusChange = (newStatus: EntityStatus | 'all') => {
    onFilterChange({
      ...activeFilters,
      status: newStatus,
    });
  };

  const handleResetFilters = () => {
    setSearchTerm('');
    isInternalChangeRef.current = true;
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }
    onFilterChange({
      q: '',
      type: 'all',
      status: 'all',
    });
  };

  const isFiltered = hasActiveFilters(activeFilters) || searchTerm.trim() !== '';

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
            onChange={(e) => handleSearchChange(e.target.value)}
            aria-label="Search entities by name or description"
          />
          {searchTerm && (
            <button
              type="button"
              className="search-clear-btn"
              onClick={handleClearSearch}
              aria-label="Clear search"
            >
              <CloseIcon size={14} />
            </button>
          )}
        </div>

        <div className="filters-row">
          <select
            className="filter-select"
            value={activeFilters.type}
            onChange={(e) => handleTypeChange(e.target.value as EntityType | 'all')}
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
            value={activeFilters.status}
            onChange={(e) => handleStatusChange(e.target.value as EntityStatus | 'all')}
            aria-label="Filter by status"
          >
            <option value="all">All Statuses</option>
            <option value="active">Active</option>
            <option value="maintenance">Maintenance</option>
            <option value="inactive">Inactive</option>
          </select>

          {isFiltered && (
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={handleResetFilters}
              title="Reset all filters"
            >
              Reset
            </button>
          )}
        </div>

        <div className="list-counts">
          <span>
            Showing <strong>{entities.length}</strong> of{' '}
            <strong>{isLoading ? '...' : matchingCount}</strong> entities
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

        {!isLoading && !error && matchingCount === 0 && !isFiltered && (
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

        {!isLoading && !error && matchingCount === 0 && isFiltered && (
          <div className="state-card state-empty">
            <SearchIcon size={32} className="state-icon text-muted" />
            <h4 className="state-title">No Matching Entities</h4>
            <p className="state-message">No entity matches the selected filters or search query.</p>
            <button type="button" className="btn btn-secondary btn-sm" onClick={handleResetFilters}>
              Reset Filters
            </button>
          </div>
        )}

        {!isLoading && !error && entities.length > 0 && (
          <div className="entity-cards-scroll">
            {entities.map((entity) => {
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

            {/* Pagination Controls */}
            {hasMore && (
              <div className="list-pagination-controls">
                <button
                  type="button"
                  className={`btn btn-secondary btn-sm btn-load-more ${isLoadingMore ? 'is-loading' : ''}`}
                  onClick={onLoadMore}
                  disabled={isLoadingMore}
                  aria-label="Load more entities from server"
                >
                  {isLoadingMore ? (
                    'Loading More...'
                  ) : (
                    `Load More (${entities.length} of ${matchingCount})`
                  )}
                </button>
              </div>
            )}

            {!hasMore && entities.length > 0 && (
              <div className="list-end-indicator">
                <span>All {matchingCount} matching entities loaded</span>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
