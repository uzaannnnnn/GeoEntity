import { AttributeDefinition, CreateAttributeDefinitionInput, UpdateAttributeDefinitionInput } from '../types/advanced';
import { EntityType } from '../types/entity';
import { ALLOWED_TYPES } from './validation';

export const RESERVED_ATTRIBUTE_KEYS = new Set([
  'id',
  'name',
  'type',
  'description',
  'status',
  'latitude',
  'longitude',
  'created_at',
  'updated_at',
  'tags',
  'categories',
  'attributes',
]);

const ATTRIBUTE_KEY_REGEX = /^[a-z][a-z0-9_]{0,63}$/;

export function validateAttributeKey(key: string): string | null {
  const trimmed = key.trim();
  if (!trimmed) {
    return 'Key is required.';
  }
  if (RESERVED_ATTRIBUTE_KEYS.has(trimmed.toLowerCase())) {
    return `"${trimmed}" is a reserved system key and cannot be used.`;
  }
  if (!ATTRIBUTE_KEY_REGEX.test(trimmed)) {
    return 'Key must start with a lowercase letter and contain only lowercase letters, digits, or underscores (max 64 chars).';
  }
  return null;
}

export function validateAttributeDefinition(
  input: Partial<CreateAttributeDefinitionInput & UpdateAttributeDefinitionInput>,
  isCreate = true
): Record<string, string> {
  const errors: Record<string, string> = {};

  if (isCreate) {
    const keyErr = validateAttributeKey(input.key || '');
    if (keyErr) errors.key = keyErr;

    if (!input.entity_type || !ALLOWED_TYPES.includes(input.entity_type as EntityType)) {
      errors.entity_type = `Entity type must be one of: ${ALLOWED_TYPES.join(', ')}`;
    }
    if (!input.value_type) {
      errors.value_type = 'Value type is required.';
    }
  }

  const label = (input.label || '').trim();
  if (!label) {
    errors.label = 'Label is required.';
  } else if (label.length > 100) {
    errors.label = 'Label must be 100 characters or fewer.';
  }

  if (input.value_type === 'number') {
    if (input.min_number !== undefined && input.min_number !== null && !isNaN(input.min_number)) {
      if (!isFinite(input.min_number)) {
        errors.min_number = 'Minimum value must be a finite number.';
      }
    }
    if (input.max_number !== undefined && input.max_number !== null && !isNaN(input.max_number)) {
      if (!isFinite(input.max_number)) {
        errors.max_number = 'Maximum value must be a finite number.';
      }
    }
    if (
      input.min_number !== undefined &&
      input.min_number !== null &&
      !isNaN(input.min_number) &&
      input.max_number !== undefined &&
      input.max_number !== null &&
      !isNaN(input.max_number)
    ) {
      if (input.min_number > input.max_number) {
        errors.min_number = 'Minimum number cannot exceed maximum number.';
      }
    }
  } else if (input.value_type === 'string') {
    if (input.max_length !== undefined && input.max_length !== null && !isNaN(input.max_length)) {
      if (input.max_length < 1 || input.max_length > 2048) {
        errors.max_length = 'Max length must be between 1 and 2048.';
      }
    }
  }

  return errors;
}

export function validateAttributeValues(
  definitions: AttributeDefinition[],
  values: Record<string, any>
): Record<string, string> {
  const errors: Record<string, string> = {};

  for (const def of definitions) {
    // Only active definitions are enforced for input validation
    if (def.archived) continue;

    const val = values[def.key];
    const isMissing = val === undefined || val === null || val === '';

    if (def.required && isMissing) {
      errors[def.key] = `${def.label} is required.`;
      continue;
    }

    if (isMissing) continue;

    if (def.value_type === 'string') {
      const strVal = String(val);
      const maxBytes = def.max_length ?? 2048;
      if (new TextEncoder().encode(strVal).length > maxBytes) {
        errors[def.key] = `${def.label} cannot exceed ${maxBytes} UTF-8 bytes.`;
      }
    } else if (def.value_type === 'number') {
      const numVal = Number(val);
      if (isNaN(numVal) || !isFinite(numVal)) {
        errors[def.key] = `${def.label} must be a valid number.`;
      } else {
        if (def.min_number !== undefined && def.min_number !== null && numVal < def.min_number) {
          errors[def.key] = `${def.label} cannot be less than ${def.min_number}.`;
        }
        if (def.max_number !== undefined && def.max_number !== null && numVal > def.max_number) {
          errors[def.key] = `${def.label} cannot be greater than ${def.max_number}.`;
        }
      }
    } else if (def.value_type === 'boolean') {
      if (typeof val !== 'boolean') {
        errors[def.key] = `${def.label} must be a boolean.`;
      }
    }
  }

  return errors;
}
