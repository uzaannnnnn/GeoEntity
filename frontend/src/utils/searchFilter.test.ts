import { describe, it, expect } from 'vitest';
import { normalizeSearchQuery, hasActiveFilters, areFiltersEqual } from './searchFilter';

describe('searchFilter utils', () => {
  it('normalizes search queries by trimming and collapsing whitespace', () => {
    expect(normalizeSearchQuery('')).toBe('');
    expect(normalizeSearchQuery('   ')).toBe('');
    expect(normalizeSearchQuery('  Truck   Alpha  ')).toBe('Truck Alpha');
    expect(normalizeSearchQuery('Facility\t\nNorth')).toBe('Facility North');
  });

  it('determines if active filters are present', () => {
    expect(hasActiveFilters({ q: '', type: 'all', status: 'all' })).toBe(false);
    expect(hasActiveFilters({ q: '   ', type: 'all', status: 'all' })).toBe(false);
    expect(hasActiveFilters({ q: 'truck', type: 'all', status: 'all' })).toBe(true);
    expect(hasActiveFilters({ q: '', type: 'vehicle', status: 'all' })).toBe(true);
    expect(hasActiveFilters({ q: '', type: 'all', status: 'active' })).toBe(true);
  });

  it('evaluates filter equality correctly', () => {
    const f1 = { q: 'truck', type: 'vehicle' as const, status: 'active' as const };
    const f2 = { q: '  truck  ', type: 'vehicle' as const, status: 'active' as const };
    const f3 = { q: 'truck', type: 'all' as const, status: 'active' as const };

    expect(areFiltersEqual(f1, f2)).toBe(true);
    expect(areFiltersEqual(f1, f3)).toBe(false);
  });
});
