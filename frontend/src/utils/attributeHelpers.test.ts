import { describe, it, expect } from 'vitest';
import {
  validateAttributeKey,
  validateAttributeDefinition,
  validateAttributeValues,
  RESERVED_ATTRIBUTE_KEYS,
} from './attributeHelpers';
import { AttributeDefinition } from '../types/advanced';

describe('attributeHelpers', () => {
  describe('validateAttributeKey', () => {
    it('validates keys matching ^[a-z][a-z0-9_]{0,63}$', () => {
      expect(validateAttributeKey('vin')).toBeNull();
      expect(validateAttributeKey('battery_level')).toBeNull();
      expect(validateAttributeKey('temp2')).toBeNull();
    });

    it('rejects empty or whitespace keys', () => {
      expect(validateAttributeKey('')).toContain('required');
      expect(validateAttributeKey('   ')).toContain('required');
    });

    it('rejects keys starting with digits or capital letters', () => {
      expect(validateAttributeKey('1number')).toBeTruthy();
      expect(validateAttributeKey('Capital')).toBeTruthy();
      expect(validateAttributeKey('_underscore')).toBeTruthy();
    });

    it('rejects keys with dashes or special characters', () => {
      expect(validateAttributeKey('kebab-case')).toBeTruthy();
      expect(validateAttributeKey('has space')).toBeTruthy();
      expect(validateAttributeKey('dollar$')).toBeTruthy();
    });

    it('rejects reserved entity keys', () => {
      for (const reserved of Array.from(RESERVED_ATTRIBUTE_KEYS)) {
        expect(validateAttributeKey(reserved)).toContain('reserved');
      }
    });
  });

  describe('validateAttributeDefinition', () => {
    it('validates correct definition input', () => {
      const errors = validateAttributeDefinition({
        entity_type: 'vehicle',
        key: 'model_year',
        label: 'Model Year',
        value_type: 'number',
        min_number: 1990,
        max_number: 2030,
        required: true,
      });
      expect(Object.keys(errors)).toHaveLength(0);
    });

    it('flags min_number greater than max_number', () => {
      const errors = validateAttributeDefinition({
        entity_type: 'vehicle',
        key: 'speed',
        label: 'Speed',
        value_type: 'number',
        min_number: 100,
        max_number: 50,
      });
      expect(errors.min_number).toContain('cannot exceed');
    });

    it('flags max_length out of bounds for string', () => {
      const errors = validateAttributeDefinition({
        entity_type: 'iot_device',
        key: 'serial',
        label: 'Serial Number',
        value_type: 'string',
        max_length: 3000,
      });
      expect(errors.max_length).toContain('between 1 and 2048');
    });

    it('accepts only the contract entity types for definitions', () => {
      const supportedTypes = ['vehicle', 'iot_device', 'facility', 'other'] as const;
      for (const entity_type of supportedTypes) {
        expect(validateAttributeDefinition({
          entity_type,
          key: 'serial',
          label: 'Serial',
          value_type: 'string',
        }).entity_type).toBeUndefined();
      }

      for (const unsupported of ['sensor', 'person']) {
        expect(validateAttributeDefinition({
          entity_type: unsupported as never,
          key: 'serial',
          label: 'Serial',
          value_type: 'string',
        }).entity_type).toContain('vehicle, iot_device, facility, other');
      }
    });
  });

  describe('validateAttributeValues', () => {
    const definitions: AttributeDefinition[] = [
      {
        id: '1',
        entity_type: 'vehicle',
        key: 'vin',
        label: 'Vehicle Identification Number',
        value_type: 'string',
        required: true,
        max_length: 17,
        archived: false,
        created_at: '',
        updated_at: '',
      },
      {
        id: '2',
        entity_type: 'vehicle',
        key: 'mileage',
        label: 'Mileage',
        value_type: 'number',
        required: false,
        min_number: 0,
        max_number: 1000000,
        archived: false,
        created_at: '',
        updated_at: '',
      },
      {
        id: '3',
        entity_type: 'vehicle',
        key: 'is_ev',
        label: 'Electric Vehicle',
        value_type: 'boolean',
        required: false,
        archived: false,
        created_at: '',
        updated_at: '',
      },
      {
        id: '4',
        entity_type: 'vehicle',
        key: 'archived_prop',
        label: 'Archived Property',
        value_type: 'string',
        required: true,
        archived: true,
        created_at: '',
        updated_at: '',
      },
    ];

    it('catches missing required fields', () => {
      const errors = validateAttributeValues(definitions, {});
      expect(errors.vin).toContain('required');
      // Archived definition is ignored even if required
      expect(errors.archived_prop).toBeUndefined();
    });

    it('catches string exceeding max_length', () => {
      const errors = validateAttributeValues(definitions, {
        vin: '123456789012345678', // 18 chars
      });
      expect(errors.vin).toContain('cannot exceed 17 UTF-8 bytes');
    });

    it('catches numbers violating min/max bounds', () => {
      const errorsBelow = validateAttributeValues(definitions, {
        vin: '12345678901234567',
        mileage: -10,
      });
      expect(errorsBelow.mileage).toContain('cannot be less than 0');

      const errorsAbove = validateAttributeValues(definitions, {
        vin: '12345678901234567',
        mileage: 2000000,
      });
      expect(errorsAbove.mileage).toContain('cannot be greater than 1000000');
    });

    it('passes with valid values', () => {
      const errors = validateAttributeValues(definitions, {
        vin: '12345678901234567',
        mileage: 12500,
        is_ev: true,
      });
      expect(Object.keys(errors)).toHaveLength(0);
    });
  });

  it('uses UTF-8 byte length and the backend default string limit', () => {
    const baseStringDefinition: AttributeDefinition = {
      id: 'utf8', entity_type: 'vehicle', key: 'text', label: 'Text', value_type: 'string',
      required: false, archived: false, created_at: '', updated_at: '',
    };
    const definition = { ...baseStringDefinition, max_length: 4 };
    expect(validateAttributeValues([definition], { text: '🌍' })).toEqual({});
    expect(validateAttributeValues([definition], { text: '🌍a' }).text).toContain('4 UTF-8 bytes');
    expect(validateAttributeValues([{ ...baseStringDefinition, max_length: null }], { text: 'x'.repeat(2049) }).text).toContain('2048 UTF-8 bytes');
  });
});
