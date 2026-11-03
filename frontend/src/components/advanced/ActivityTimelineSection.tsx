import { useState, useEffect, useCallback, useRef } from 'react';
import { Entity } from '../../types/entity';
import { ActivityEvent } from '../../types/advanced';
import { activityApi } from '../../api/advanced';
import { ApiError } from '../../api/client';
import { formatDateTime } from '../../utils/formatters';
import { RequestGuard } from '../../utils/requestGuard';
import {
  ActivityIcon,
  RefreshIcon,
  PlusIcon,
  EditIcon,
  TrashIcon,
  AttributesIcon,
  TagIcon,
  CategoryIcon,
} from '../Icons';

interface ActivityTimelineSectionProps {
  entity: Entity;
}

export function ActivityTimelineSection({ entity }: ActivityTimelineSectionProps) {
  const requestGuard = useRef(new RequestGuard()).current;
  const loadingMoreRef = useRef(false);
  const [events, setEvents] = useState<ActivityEvent[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isLoadingMore, setIsLoadingMore] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const loadInitial = useCallback(async () => {
    const request = requestGuard.begin();
    setIsLoading(true);
    setError(null);
    try {
      const page = await activityApi.getByEntity(entity.id, 20);
      if (!requestGuard.isCurrent(request)) return;
      setEvents(page.data || []);
      setNextCursor(page.meta?.next_cursor || null);
    } catch (err) {
      if (!requestGuard.isCurrent(request)) return;
      const msg = err instanceof ApiError ? err.message : 'Failed to load activity timeline';
      setError(msg);
    } finally {
      if (requestGuard.isCurrent(request)) setIsLoading(false);
    }
  }, [entity.id, requestGuard]);

  useEffect(() => {
    loadInitial();
    return () => requestGuard.invalidate();
  }, [loadInitial, requestGuard]);

  const handleLoadMore = async () => {
    if (!nextCursor || loadingMoreRef.current) return;
    const request = requestGuard.begin();
    loadingMoreRef.current = true;
    setIsLoadingMore(true);
    try {
      const page = await activityApi.getByEntity(entity.id, 20, nextCursor);
      if (!requestGuard.isCurrent(request)) return;
      setEvents((prev) => [...prev, ...(page.data || [])]);
      setNextCursor(page.meta?.next_cursor || null);
    } catch (err) {
      if (!requestGuard.isCurrent(request)) return;
      const msg = err instanceof ApiError ? err.message : 'Failed to load more activity events';
      setError(msg);
    } finally {
      loadingMoreRef.current = false;
      if (requestGuard.isCurrent(request)) setIsLoadingMore(false);
    }
  };

  const getEventBadge = (eventType: string) => {
    switch (eventType) {
      case 'entity.created':
        return {
          icon: <PlusIcon size={14} className="timeline-node-icon icon-created" />,
          title: 'Entity Created',
          badgeClass: 'badge-created',
        };
      case 'entity.updated':
        return {
          icon: <EditIcon size={14} className="timeline-node-icon icon-updated" />,
          title: 'Entity Updated',
          badgeClass: 'badge-updated',
        };
      case 'entity.deleted':
        return {
          icon: <TrashIcon size={14} className="timeline-node-icon icon-deleted" />,
          title: 'Entity Deleted',
          badgeClass: 'badge-deleted',
        };
      case 'entity.attributes.updated':
        return {
          icon: <AttributesIcon size={14} className="timeline-node-icon icon-attributes" />,
          title: 'Attributes Updated',
          badgeClass: 'badge-attributes',
        };
      case 'entity.tags.updated':
        return {
          icon: <TagIcon size={14} className="timeline-node-icon icon-tags" />,
          title: 'Tags Updated',
          badgeClass: 'badge-tags',
        };
      case 'entity.categories.updated':
        return {
          icon: <CategoryIcon size={14} className="timeline-node-icon icon-categories" />,
          title: 'Categories Updated',
          badgeClass: 'badge-categories',
        };
      default:
        return {
          icon: <ActivityIcon size={14} className="timeline-node-icon" />,
          title: eventType,
          badgeClass: 'badge-default',
        };
    }
  };

  const renderEventChanges = (event: ActivityEvent) => {
    const changes = event.changes || {};

    if (event.event_type === 'entity.created') {
      const ent = changes.entity || {};
      return (
        <div className="diff-summary">
          <span className="diff-item">
            Initial Type: <strong>{ent.type || '—'}</strong>
          </span>
          <span className="diff-item">
            Status: <strong>{ent.status || '—'}</strong>
          </span>
          {ent.latitude !== undefined && (
            <span className="diff-item">
              Location: <code>{Number(ent.latitude).toFixed(4)}, {Number(ent.longitude).toFixed(4)}</code>
            </span>
          )}
        </div>
      );
    }

    if (event.event_type === 'entity.updated') {
      const before = changes.before || {};
      const after = changes.after || {};
      const keys = Array.from(new Set([...Object.keys(before), ...Object.keys(after)]));
      const modifiedKeys = keys.filter(
        (k) =>
          k !== 'updated_at' &&
          k !== 'id' &&
          k !== 'description_truncated' &&
          JSON.stringify(before[k]) !== JSON.stringify(after[k])
      );

      if (modifiedKeys.length === 0) {
        return <span className="text-muted text-xs">No field values changed.</span>;
      }

      return (
        <div className="diff-table">
          {modifiedKeys.map((k) => (
            <div key={k} className="diff-row">
              <span className="diff-prop">{k}:</span>
              <span className="diff-val-old">{String(before[k] ?? '—')}</span>
              <span className="diff-arrow">&rarr;</span>
              <span className="diff-val-new">{String(after[k] ?? '—')}</span>
            </div>
          ))}
        </div>
      );
    }

    if (event.event_type === 'entity.attributes.updated') {
      const before = changes.before || {};
      const after = changes.after || {};
      const keys = Array.from(new Set([...Object.keys(before), ...Object.keys(after)]));
      const modifiedKeys = keys.filter(
        (k) => JSON.stringify(before[k]) !== JSON.stringify(after[k])
      );

      if (modifiedKeys.length === 0) {
        return <span className="text-muted text-xs">Attributes saved with no changes.</span>;
      }

      return (
        <div className="diff-table">
          {modifiedKeys.map((k) => {
            const oldVal = before[k];
            const newVal = after[k];
            const oldDisplay =
              typeof oldVal === 'object' && oldVal?.truncated ? `${oldVal.value}...` : String(oldVal ?? '—');
            const newDisplay =
              typeof newVal === 'object' && newVal?.truncated ? `${newVal.value}...` : String(newVal ?? '—');

            return (
              <div key={k} className="diff-row">
                <span className="diff-prop"><code>{k}</code>:</span>
                <span className="diff-val-old">{oldDisplay}</span>
                <span className="diff-arrow">&rarr;</span>
                <span className="diff-val-new">{newDisplay}</span>
              </div>
            );
          })}
        </div>
      );
    }

    if (event.event_type === 'entity.tags.updated' || event.event_type === 'entity.categories.updated') {
      const beforeList: Array<{ id: string; name: string }> = changes.before || [];
      const afterList: Array<{ id: string; name: string }> = changes.after || [];

      const beforeNames = beforeList.map((x) => x.name).join(', ') || 'None';
      const afterNames = afterList.map((x) => x.name).join(', ') || 'None';

      return (
        <div className="diff-row">
          <span className="diff-val-old">{beforeNames}</span>
          <span className="diff-arrow">&rarr;</span>
          <span className="diff-val-new">{afterNames}</span>
        </div>
      );
    }

    if (event.event_type === 'entity.deleted') {
      return (
        <div className="diff-summary">
          <span className="text-danger text-xs font-semibold">Entity was deleted.</span>
        </div>
      );
    }

    return null;
  };

  if (isLoading) {
    return (
      <div className="section-loading">
        <RefreshIcon size={18} className="animate-spin text-muted" />
        <span>Loading activity timeline...</span>
      </div>
    );
  }

  if (error && events.length === 0) {
    return (
      <div className="section-error">
        <p className="text-danger">{error}</p>
        <button type="button" className="btn btn-secondary btn-sm" onClick={loadInitial}>
          <RefreshIcon size={14} />
          <span>Retry</span>
        </button>
      </div>
    );
  }

  return (
    <div className="activity-timeline-section">
      <div className="section-header-actions">
        <div>
          <h3 className="section-title">Audit Activity Log</h3>
          <span className="section-subtitle">
            Immutable chronological record of events
          </span>
        </div>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={loadInitial}
          title="Refresh activity"
        >
          <RefreshIcon size={14} />
          <span>Refresh</span>
        </button>
      </div>

      {events.length === 0 ? (
        <div className="empty-section-card">
          <ActivityIcon size={24} className="text-muted" />
          <p>No activity records logged for this entity yet.</p>
        </div>
      ) : (
        <div className="timeline-container">
          <div className="timeline-track" />
          <div className="timeline-events-list">
            {events.map((event) => {
              const meta = getEventBadge(event.event_type);

              return (
                <div key={event.id} className="timeline-item">
                  <div className="timeline-node">{meta.icon}</div>
                  <div className="timeline-card">
                    <div className="timeline-card-header">
                      <div className="timeline-event-name-group">
                        <span className={`timeline-badge ${meta.badgeClass}`}>{meta.title}</span>
                        <span className="timeline-actor">by {event.actor?.kind || 'system'}</span>
                      </div>
                      <time
                        className="timeline-timestamp"
                        dateTime={event.occurred_at}
                        title={event.occurred_at}
                      >
                        {formatDateTime(event.occurred_at)}
                      </time>
                    </div>

                    <div className="timeline-card-body">
                      {renderEventChanges(event)}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>

          {nextCursor && (
            <div className="timeline-load-more">
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={handleLoadMore}
                disabled={isLoadingMore}
              >
                {isLoadingMore ? (
                  <>
                    <RefreshIcon size={14} className="animate-spin" />
                    <span>Loading more events...</span>
                  </>
                ) : (
                  <span>Load Earlier Activity</span>
                )}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
