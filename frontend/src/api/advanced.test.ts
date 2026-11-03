import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  attributeDefinitionsApi,
  entityAttributesApi,
  tagsApi,
  categoriesApi,
  entityAssignmentsApi,
  activityApi,
} from './advanced';
import { apiClient } from './client';

describe('Advanced Entity Management API', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('attributeDefinitionsApi', () => {
    it('creates a string definition with max_length', async () => {
      const def = {
        id: 'def-str',
        entity_type: 'iot_device' as const,
        key: 'firmware_version',
        label: 'Firmware Version',
        value_type: 'string' as const,
        required: false,
        max_length: 32,
        archived: false,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };
      vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ data: def });

      const res = await attributeDefinitionsApi.create({
        entity_type: 'iot_device',
        key: 'firmware_version',
        label: 'Firmware Version',
        value_type: 'string',
        required: false,
        max_length: 32,
      });

      expect(apiClient.post).toHaveBeenCalledWith('/attribute-definitions', {
        entity_type: 'iot_device',
        key: 'firmware_version',
        label: 'Firmware Version',
        value_type: 'string',
        required: false,
        max_length: 32,
      });
      expect(res).toEqual(def);
    });

    it('fetches definitions filtered by entity_type', async () => {
      const mockDefs = [
        {
          id: 'def-1',
          entity_type: 'vehicle',
          key: 'vin',
          label: 'VIN',
          value_type: 'string',
          required: true,
          archived: false,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ];
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: mockDefs });

      const res = await attributeDefinitionsApi.getByEntityType('vehicle');
      expect(apiClient.get).toHaveBeenCalledWith('/attribute-definitions?entity_type=vehicle');
      expect(res).toEqual(mockDefs);
    });

    it('creates a new definition with allowed fields', async () => {
      const newDef = {
        id: 'def-2',
        entity_type: 'vehicle',
        key: 'year',
        label: 'Model Year',
        value_type: 'number' as const,
        required: true,
        min_number: 1900,
        max_number: 2100,
        archived: false,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      };
      vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ data: newDef });

      const res = await attributeDefinitionsApi.create({
        entity_type: 'vehicle',
        key: 'year',
        label: 'Model Year',
        value_type: 'number',
        required: true,
        min_number: 1900,
        max_number: 2100,
      });

      expect(apiClient.post).toHaveBeenCalledWith('/attribute-definitions', {
        entity_type: 'vehicle',
        key: 'year',
        label: 'Model Year',
        value_type: 'number',
        required: true,
        min_number: 1900,
        max_number: 2100,
      });
      expect(res).toEqual(newDef);
    });

    it('updates a definition with strict mutable fields', async () => {
      const updatedDef = {
        id: 'def-2',
        entity_type: 'vehicle',
        key: 'year',
        label: 'Year of Production',
        value_type: 'number' as const,
        required: false,
        min_number: 1950,
        max_number: 2050,
        archived: false,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-02T00:00:00Z',
      };
      vi.spyOn(apiClient, 'put').mockResolvedValueOnce({ data: updatedDef });

      const res = await attributeDefinitionsApi.update('def-2', {
        label: 'Year of Production',
        required: false,
        min_number: 1950,
        max_number: 2050,
        max_length: null,
        archived: false,
      });

      expect(apiClient.put).toHaveBeenCalledWith('/attribute-definitions/def-2', {
        label: 'Year of Production',
        required: false,
        min_number: 1950,
        max_number: 2050,
        max_length: null,
        archived: false,
      });
      expect(res).toEqual(updatedDef);
    });
  });

  describe('entityAttributesApi', () => {
    it('fetches entity attributes with current and previous type values', async () => {
      const mockData = {
        entity_type: 'vehicle' as const,
        values: { vin: '1HGCR2F83HA000000' },
        previous_type_values: { 'iot_device.battery_level': 95 },
      };
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: mockData });

      const res = await entityAttributesApi.get('entity-uuid-1');
      expect(apiClient.get).toHaveBeenCalledWith('/entities/entity-uuid-1/attributes');
      expect(res).toEqual(mockData);
    });

    it('replaces entity attributes with validated values object', async () => {
      vi.spyOn(apiClient, 'put').mockResolvedValueOnce({
        data: { values: { vin: '1HGCR2F83HA000000' } },
      });

      const res = await entityAttributesApi.replace('entity-uuid-1', {
        vin: '1HGCR2F83HA000000',
      });

      expect(apiClient.put).toHaveBeenCalledWith('/entities/entity-uuid-1/attributes', {
        values: { vin: '1HGCR2F83HA000000' },
      });
      expect(res).toEqual({ vin: '1HGCR2F83HA000000' });
    });
  });

  describe('tags and categories catalogs', () => {
    it('manages tags catalog', async () => {
      const mockTags = [{ id: 't1', name: 'Fleet Alpha' }];
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: mockTags });
      const tags = await tagsApi.getAll();
      expect(apiClient.get).toHaveBeenCalledWith('/tags');
      expect(tags).toEqual(mockTags);

      vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ data: { id: 't2', name: 'Field Unit' } });
      const created = await tagsApi.create('Field Unit');
      expect(apiClient.post).toHaveBeenCalledWith('/tags', { name: 'Field Unit' });
      expect(created.name).toBe('Field Unit');

      vi.spyOn(apiClient, 'delete').mockResolvedValueOnce(undefined);
      await tagsApi.delete('t2');
      expect(apiClient.delete).toHaveBeenCalledWith('/tags/t2');
    });

    it('manages categories catalog', async () => {
      const mockCats = [{ id: 'c1', name: 'Logistics' }];
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: mockCats });
      const cats = await categoriesApi.getAll();
      expect(apiClient.get).toHaveBeenCalledWith('/categories');
      expect(cats).toEqual(mockCats);

      vi.spyOn(apiClient, 'post').mockResolvedValueOnce({ data: { id: 'c2', name: 'Sensors' } });
      const created = await categoriesApi.create('Sensors');
      expect(apiClient.post).toHaveBeenCalledWith('/categories', { name: 'Sensors' });
      expect(created.name).toBe('Sensors');

      vi.spyOn(apiClient, 'delete').mockResolvedValueOnce(undefined);
      await categoriesApi.delete('c2');
      expect(apiClient.delete).toHaveBeenCalledWith('/categories/c2');
    });
  });

  describe('entity assignments', () => {
    it('replaces tags on an entity', async () => {
      const mockAssigned = [{ id: 't1', name: 'Fleet Alpha' }];
      vi.spyOn(apiClient, 'put').mockResolvedValueOnce({ data: mockAssigned });

      const res = await entityAssignmentsApi.replaceTags('entity-1', ['t1']);
      expect(apiClient.put).toHaveBeenCalledWith('/entities/entity-1/tags', {
        tag_ids: ['t1'],
      });
      expect(res).toEqual(mockAssigned);
    });

    it('replaces categories on an entity', async () => {
      const mockAssigned = [{ id: 'c1', name: 'Logistics' }];
      vi.spyOn(apiClient, 'put').mockResolvedValueOnce({ data: mockAssigned });

      const res = await entityAssignmentsApi.replaceCategories('entity-1', ['c1']);
      expect(apiClient.put).toHaveBeenCalledWith('/entities/entity-1/categories', {
        category_ids: ['c1'],
      });
      expect(res).toEqual(mockAssigned);
    });
  });

  describe('activityApi', () => {
    it('fetches initial activity events without cursor', async () => {
      const mockPage = {
        data: [],
        meta: { next_cursor: '' },
      };
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce(mockPage);

      const res = await activityApi.getByEntity('entity-1');
      expect(apiClient.get).toHaveBeenCalledWith('/entities/entity-1/activity?limit=20');
      expect(res).toEqual(mockPage);
    });

    it('fetches paginated activity events with limit and cursor', async () => {
      const mockPage = {
        data: [
          {
            id: 'ev-1',
            entity_id: 'entity-1',
            event_type: 'entity.updated',
            occurred_at: '2026-01-01T12:00:00Z',
            actor: { kind: 'system', id: null },
            payload_version: 1,
            changes: { before: { name: 'Old' }, after: { name: 'New' } },
          },
        ],
        meta: { next_cursor: 'opaque-cursor-token' },
      };
      vi.spyOn(apiClient, 'get').mockResolvedValueOnce(mockPage);

      const res = await activityApi.getByEntity('entity-1', 20, 'prev-token');
      expect(apiClient.get).toHaveBeenCalledWith('/entities/entity-1/activity?limit=20&cursor=prev-token');
      expect(res).toEqual(mockPage);
    });
  });
});
