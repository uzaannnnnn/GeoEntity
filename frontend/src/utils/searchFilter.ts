import { EntityType, EntityStatus } from '../types/entity';

export interface ActiveFilters {
  q: string;
  type: EntityType | 'all';
  status: EntityStatus | 'all';
}

/**
 * Normalizes query string by trimming and collapsing whitespace runs into a single space.
 */
export function normalizeSearchQuery(query: string): string {
  if (!query) return '';
  return query.trim().replace(/\s+/g, ' ');
}

/**
 * Checks if any search term or category/type filter is active.
 */
export function hasActiveFilters(filters: ActiveFilters): boolean {
  return (
    normalizeSearchQuery(filters.q) !== '' ||
    (filters.type !== 'all' && Boolean(filters.type)) ||
    (filters.status !== 'all' && Boolean(filters.status))
  );
}

/**
 * Checks equality between two filter sets.
 */
export function areFiltersEqual(a: ActiveFilters, b: ActiveFilters): boolean {
  return (
    normalizeSearchQuery(a.q) === normalizeSearchQuery(b.q) &&
    a.type === b.type &&
    a.status === b.status
  );
}
