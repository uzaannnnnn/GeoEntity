import { apiClient } from './client';
import { ApiDataResponse, EntityType } from '../types/entity';
import {
  ActivityPage,
  AttributeDefinition,
  CatalogItem,
  CreateAttributeDefinitionInput,
  EntityAttributesData,
  UpdateAttributeDefinitionInput,
} from '../types/advanced';

export const attributeDefinitionsApi = {
  async getByEntityType(entityType: EntityType): Promise<AttributeDefinition[]> {
    const res = await apiClient.get<ApiDataResponse<AttributeDefinition[]>>(
      `/attribute-definitions?entity_type=${encodeURIComponent(entityType)}`
    );
    return res.data;
  },

  async create(input: CreateAttributeDefinitionInput): Promise<AttributeDefinition> {
    const payload: Record<string, unknown> = {
      entity_type: input.entity_type,
      key: input.key.trim(),
      label: input.label.trim(),
      value_type: input.value_type,
      required: Boolean(input.required),
    };

    if (input.value_type === 'number') {
      if (input.min_number !== undefined && input.min_number !== null && !isNaN(input.min_number)) {
        payload.min_number = Number(input.min_number);
      }
      if (input.max_number !== undefined && input.max_number !== null && !isNaN(input.max_number)) {
        payload.max_number = Number(input.max_number);
      }
    } else if (input.value_type === 'string') {
      if (input.max_length !== undefined && input.max_length !== null && !isNaN(input.max_length)) {
        payload.max_length = Number(input.max_length);
      }
    }

    const res = await apiClient.post<ApiDataResponse<AttributeDefinition>>(
      '/attribute-definitions',
      payload
    );
    return res.data;
  },

  async update(id: string, input: UpdateAttributeDefinitionInput): Promise<AttributeDefinition> {
    // Only send the strict mutable fields accepted by PUT /api/attribute-definitions/{id}
    const payload = {
      label: input.label.trim(),
      required: Boolean(input.required),
      min_number:
        input.min_number !== undefined && input.min_number !== null && !isNaN(input.min_number)
          ? Number(input.min_number)
          : null,
      max_number:
        input.max_number !== undefined && input.max_number !== null && !isNaN(input.max_number)
          ? Number(input.max_number)
          : null,
      maxLength: undefined, // ensure no extra camelCase keys
      max_length:
        input.max_length !== undefined && input.max_length !== null && !isNaN(input.max_length)
          ? Number(input.max_length)
          : null,
      archived: Boolean(input.archived),
    };
    // remove undefined properties so JSON decoder won't see them
    delete (payload as Record<string, unknown>).maxLength;

    const res = await apiClient.put<ApiDataResponse<AttributeDefinition>>(
      `/attribute-definitions/${id}`,
      payload
    );
    return res.data;
  },
};

export const entityAttributesApi = {
  async get(entityId: string): Promise<EntityAttributesData> {
    const res = await apiClient.get<ApiDataResponse<EntityAttributesData>>(
      `/entities/${entityId}/attributes`
    );
    return res.data;
  },

  async replace(
    entityId: string,
    values: Record<string, string | number | boolean>
  ): Promise<Record<string, string | number | boolean>> {
    const res = await apiClient.put<ApiDataResponse<{ values: Record<string, string | number | boolean> }>>(
      `/entities/${entityId}/attributes`,
      { values }
    );
    return res.data.values;
  },
};

export const tagsApi = {
  async getAll(): Promise<CatalogItem[]> {
    const res = await apiClient.get<ApiDataResponse<CatalogItem[]>>('/tags');
    return res.data;
  },

  async create(name: string): Promise<CatalogItem> {
    const res = await apiClient.post<ApiDataResponse<CatalogItem>>('/tags', {
      name: name.trim(),
    });
    return res.data;
  },

  async delete(id: string): Promise<void> {
    await apiClient.delete(`/tags/${id}`);
  },
};

export const categoriesApi = {
  async getAll(): Promise<CatalogItem[]> {
    const res = await apiClient.get<ApiDataResponse<CatalogItem[]>>('/categories');
    return res.data;
  },

  async create(name: string): Promise<CatalogItem> {
    const res = await apiClient.post<ApiDataResponse<CatalogItem>>('/categories', {
      name: name.trim(),
    });
    return res.data;
  },

  async delete(id: string): Promise<void> {
    await apiClient.delete(`/categories/${id}`);
  },
};

export const entityAssignmentsApi = {
  async getTags(entityId: string): Promise<CatalogItem[]> {
    const res = await apiClient.get<ApiDataResponse<CatalogItem[]>>(`/entities/${entityId}/tags`);
    return res.data;
  },

  async replaceTags(entityId: string, tagIds: string[]): Promise<CatalogItem[]> {
    const res = await apiClient.put<ApiDataResponse<CatalogItem[]>>(
      `/entities/${entityId}/tags`,
      { tag_ids: tagIds }
    );
    return res.data;
  },

  async getCategories(entityId: string): Promise<CatalogItem[]> {
    const res = await apiClient.get<ApiDataResponse<CatalogItem[]>>(
      `/entities/${entityId}/categories`
    );
    return res.data;
  },

  async replaceCategories(entityId: string, categoryIds: string[]): Promise<CatalogItem[]> {
    const res = await apiClient.put<ApiDataResponse<CatalogItem[]>>(
      `/entities/${entityId}/categories`,
      { category_ids: categoryIds }
    );
    return res.data;
  },
};

export const activityApi = {
  async getByEntity(entityId: string, limit = 20, cursor?: string): Promise<ActivityPage> {
    let url = `/entities/${entityId}/activity?limit=${limit}`;
    if (cursor) {
      url += `&cursor=${encodeURIComponent(cursor)}`;
    }
    const res = await apiClient.get<ActivityPage>(url);
    return res;
  },
};
