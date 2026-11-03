import { EntityType } from './entity';

export type AttributeValueType = 'string' | 'number' | 'boolean';

export interface AttributeDefinition {
  id: string;
  entity_type: EntityType;
  key: string;
  label: string;
  value_type: AttributeValueType;
  required: boolean;
  min_number?: number | null;
  max_number?: number | null;
  max_length?: number | null;
  archived: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateAttributeDefinitionInput {
  entity_type: EntityType;
  key: string;
  label: string;
  value_type: AttributeValueType;
  required: boolean;
  min_number?: number | null;
  max_number?: number | null;
  max_length?: number | null;
}

export interface UpdateAttributeDefinitionInput {
  label: string;
  required: boolean;
  min_number?: number | null;
  max_number?: number | null;
  max_length?: number | null;
  archived: boolean;
}

export interface EntityAttributesData {
  entity_type: EntityType;
  values: Record<string, string | number | boolean>;
  previous_type_values: Record<string, string | number | boolean>;
}

export interface CatalogItem {
  id: string;
  name: string;
}

export interface ActivityActor {
  kind: string;
  id: string | null;
}

export interface ActivityEvent {
  id: string;
  entity_id: string;
  event_type: string;
  occurred_at: string;
  actor: ActivityActor;
  payload_version: number;
  changes: Record<string, any>;
}

export interface ActivityPage {
  data: ActivityEvent[];
  meta: {
    next_cursor: string;
  };
}
