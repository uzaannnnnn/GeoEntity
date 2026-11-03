import { useState, useEffect, useCallback, useRef } from 'react';
import { GeofenceDefinition, GeofenceEvent, GeofenceEventFilters } from '../../types/geofence';
import { geofencesApi } from '../../api/geofences';
import { ApiError } from '../../api/client';
import { formatCoordinate, formatDateTime } from '../../utils/formatters';
import { EnterIcon, ExitIcon, RefreshIcon, AlertIcon, SearchIcon, InfoIcon, ActivityIcon } from '../Icons';

interface GeofenceEventTimelineProps {
  geofence: GeofenceDefinition;
  onSelectEntity?: (entityId: string) => void;
}

export function GeofenceEventTimeline({ geofence, onSelectEntity }: GeofenceEventTimelineProps) {
  const [events, setEvents] = useState<GeofenceEvent[]>([]);
  const [nextCursor, setNextCursor] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isLoadingMore, setIsLoadingMore] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  // Filters state
  const [entityFilter, setEntityFilter] = useState<string>('');
  const [fromFilter, setFromFilter] = useState<string>('');
  const [toFilter, setToFilter] = useState<string>('');

  // Applied filters state
  const [appliedFilters, setAppliedFilters] = useState<GeofenceEventFilters>({});

  // Stale request guard
  const activeGeofenceIdRef = useRef(geofence.id);
  activeGeofenceIdRef.current = geofence.id;

  const fetchEvents = useCallback(
    async (cursor?: string) => {
      const isInitial = !cursor;
      if (isInitial) {
        setIsLoading(true);
        setError(null);
      } else {
        setIsLoadingMore(true);
      }

      try {
        const filters: GeofenceEventFilters = {
          ...appliedFilters,
          limit: 50,
          cursor: cursor || undefined,
        };

        const page = await geofencesApi.getGeofenceEvents(geofence.id, filters);

        if (activeGeofenceIdRef.current === geofence.id) {
          if (isInitial) {
            setEvents(page.data);
          } else {
            setEvents((prev) => [...prev, ...page.data]);
          }
          setNextCursor(page.meta.next_cursor || '');
        }
      } catch (err) {
        if (activeGeofenceIdRef.current === geofence.id) {
          if (err instanceof ApiError) {
            setError(err.message);
          } else {
            setError('Failed to fetch event history. Please check connection.');
          }
        }
      } finally {
        if (activeGeofenceIdRef.current === geofence.id) {
          setIsLoading(false);
          setIsLoadingMore(false);
        }
      }
    },
    [geofence.id, appliedFilters]
  );

  // Re-fetch when geofence or applied filters change
  useEffect(() => {
    fetchEvents();
  }, [fetchEvents]);

  const handleApplyFilters = (e: React.FormEvent) => {
    e.preventDefault();
    const newFilters: GeofenceEventFilters = {};
    if (entityFilter.trim()) {
      newFilters.entity_id = entityFilter.trim();
    }
    if (fromFilter) {
      newFilters.from = new Date(fromFilter).toISOString();
    }
    if (toFilter) {
      newFilters.to = new Date(toFilter).toISOString();
    }
    setAppliedFilters(newFilters);
  };

  const handleClearFilters = () => {
    setEntityFilter('');
    setFromFilter('');
    setToFilter('');
    setAppliedFilters({});
  };

  const hasActiveFilters = Boolean(
    appliedFilters.entity_id || appliedFilters.from || appliedFilters.to
  );

  return (
    <div className="geofence-event-timeline">
      {/* Informational Disclaimer Banner */}
      <div className="event-semantics-banner" role="note">
        <InfoIcon size={16} className="text-info" />
        <div className="banner-content">
          <strong>Historical Transition Log:</strong> Events represent spatial transitions
          computed on saved entity coordinate changes. This is historical operations data, not
          live GPS or real-time device telemetry.
        </div>
      </div>

      {/* Filter Toolbar */}
      <form className="event-filter-toolbar" onSubmit={handleApplyFilters}>
        <div className="filter-inputs-grid">
          <div className="form-group-sm">
            <label htmlFor="filter-entity-id" className="form-label-sm">
              Entity UUID:
            </label>
            <input
              id="filter-entity-id"
              type="text"
              className="form-control-sm"
              placeholder="e.g. 00000000-..."
              value={entityFilter}
              onChange={(e) => setEntityFilter(e.target.value)}
            />
          </div>

          <div className="form-group-sm">
            <label htmlFor="filter-from" className="form-label-sm">
              From Date:
            </label>
            <input
              id="filter-from"
              type="datetime-local"
              className="form-control-sm"
              value={fromFilter}
              onChange={(e) => setFromFilter(e.target.value)}
            />
          </div>

          <div className="form-group-sm">
            <label htmlFor="filter-to" className="form-label-sm">
              To Date:
            </label>
            <input
              id="filter-to"
              type="datetime-local"
              className="form-control-sm"
              value={toFilter}
              onChange={(e) => setToFilter(e.target.value)}
            />
          </div>
        </div>

        <div className="filter-actions-row">
          <button type="submit" className="btn btn-secondary btn-sm" disabled={isLoading}>
            <SearchIcon size={14} />
            <span>Apply Filters</span>
          </button>
          {hasActiveFilters && (
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={handleClearFilters}
              disabled={isLoading}
            >
              Clear Filters
            </button>
          )}
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon-only ml-auto"
            onClick={() => fetchEvents()}
            disabled={isLoading}
            title="Refresh events"
            aria-label="Refresh events"
          >
            <RefreshIcon size={14} className={isLoading ? 'animate-spin' : ''} />
          </button>
        </div>
      </form>

      {/* Error State */}
      {error && !isLoading && (
        <div className="alert-box alert-error" role="alert">
          <AlertIcon size={16} />
          <div className="alert-content">
            <strong>Error:</strong> {error}
          </div>
          <button type="button" className="btn btn-sm btn-secondary" onClick={() => fetchEvents()}>
            Retry
          </button>
        </div>
      )}

      {/* Initial Loading State */}
      {isLoading && (
        <div className="events-loading-state" role="status">
          <span className="spinner-icon spinner-md" aria-hidden="true" />
          <span>Loading event history...</span>
        </div>
      )}

      {/* Empty State */}
      {!isLoading && !error && events.length === 0 && (
        <div className="events-empty-state">
          <div className="empty-icon-wrap">
            <ActivityIcon size={28} />
          </div>
          <h4>No Transition Events</h4>
          <p>
            {hasActiveFilters
              ? 'No events match the selected filters for this geofence.'
              : 'No spatial transition events have been recorded for this geofence yet.'}
          </p>
        </div>
      )}

      {/* Events List */}
      {!isLoading && !error && events.length > 0 && (
        <div className="events-timeline-list">
          <div className="events-count-label">
            Showing <strong>{events.length}</strong> event(s) (newest first)
          </div>

          <div className="events-cards-stack">
            {events.map((evt) => {
              const isEnter = evt.event_type === 'enter';

              return (
                <div
                  key={evt.id}
                  className={`event-card type-${evt.event_type}`}
                  data-testid={`event-${evt.id}`}
                >
                  <div className="event-badge-col">
                    {isEnter ? (
                      <span className="event-type-badge badge-enter" title="Entity entered geofence">
                        <EnterIcon size={14} />
                        <strong>ENTER</strong>
                      </span>
                    ) : (
                      <span className="event-type-badge badge-exit" title="Entity exited geofence">
                        <ExitIcon size={14} />
                        <strong>EXIT</strong>
                      </span>
                    )}
                  </div>

                  <div className="event-details-col">
                    <div className="event-title-row">
                      <span className="event-entity-name">{evt.entity_name}</span>
                      <code className="event-entity-id" title={`Entity ID: ${evt.entity_id}`}>
                        {evt.entity_id}
                      </code>
                    </div>

                    <div className="event-meta-row">
                      <span className="meta-item">
                        Coordinates: {formatCoordinate(evt.latitude)}&deg;, {formatCoordinate(evt.longitude)}&deg;
                      </span>
                      <span className="meta-separator">•</span>
                      <span className="meta-item" title={evt.occurred_at}>
                        {formatDateTime(evt.occurred_at)}
                      </span>
                      <span className="meta-separator">•</span>
                      <span className="source-tag">{evt.source}</span>
                    </div>
                  </div>

                  {onSelectEntity && (
                    <div className="event-action-col">
                      <button
                        type="button"
                        className="btn btn-ghost btn-sm"
                        onClick={() => onSelectEntity(evt.entity_id)}
                        title="View entity details"
                      >
                        Inspect Entity
                      </button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {/* Keyset Cursor Pagination Trigger */}
          {nextCursor && (
            <div className="events-load-more-row">
              <button
                type="button"
                className="btn btn-secondary btn-load-more"
                onClick={() => fetchEvents(nextCursor)}
                disabled={isLoadingMore}
              >
                {isLoadingMore ? 'Loading More Events...' : 'Load More Events'}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
