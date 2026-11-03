import { useState, useEffect, useCallback, useId, useRef } from 'react';
import { EntityType } from '../../types/entity';
import { AttributeDefinition, CreateAttributeDefinitionInput, UpdateAttributeDefinitionInput } from '../../types/advanced';
import { attributeDefinitionsApi } from '../../api/advanced';
import { ApiError } from '../../api/client';
import { validateAttributeDefinition } from '../../utils/attributeHelpers';
import { RequestGuard } from '../../utils/requestGuard';
import {
  CloseIcon,
  PlusIcon,
  EditIcon,
  RefreshIcon,
  CheckIcon,
  AlertIcon,
  VehicleIcon,
  IoTDeviceIcon,
  FacilityIcon,
  OtherIcon,
} from '../Icons';

interface AttributeDefinitionsModalProps {
  isOpen: boolean;
  onClose: () => void;
  onToast: (type: 'success' | 'error' | 'info', message: string, title?: string) => void;
}

const ENTITY_TYPES: Array<{ type: EntityType; label: string; icon: React.ReactNode }> = [
  { type: 'vehicle', label: 'Vehicle', icon: <VehicleIcon size={16} /> },
  { type: 'iot_device', label: 'IoT Device', icon: <IoTDeviceIcon size={16} /> },
  { type: 'facility', label: 'Facility', icon: <FacilityIcon size={16} /> },
  { type: 'other', label: 'Other', icon: <OtherIcon size={16} /> },
];

export function AttributeDefinitionsModal({
  isOpen,
  onClose,
  onToast,
}: AttributeDefinitionsModalProps) {
  const formId = useId();
  const [selectedType, setSelectedType] = useState<EntityType>('vehicle');
  const requestGuard = useRef(new RequestGuard()).current;
  const selectedTypeRef = useRef(selectedType);
  const isOpenRef = useRef(isOpen);
  selectedTypeRef.current = selectedType;
  isOpenRef.current = isOpen;
  const [definitions, setDefinitions] = useState<AttributeDefinition[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Form sub-modes: 'list' | 'create' | 'edit'
  const [modalMode, setModalMode] = useState<'list' | 'create' | 'edit'>('list');
  const [editingDefinition, setEditingDefinition] = useState<AttributeDefinition | null>(null);

  // Create Form State
  const [createKey, setCreateKey] = useState<string>('');
  const [createLabel, setCreateLabel] = useState<string>('');
  const [createValueType, setCreateValueType] = useState<'string' | 'number' | 'boolean'>('string');
  const [createRequired, setCreateRequired] = useState<boolean>(false);
  const [createMinNumber, setCreateMinNumber] = useState<string>('');
  const [createMaxNumber, setCreateMaxNumber] = useState<string>('');
  const [createMaxLength, setCreateMaxLength] = useState<string>('');

  // Edit Form State
  const [editLabel, setEditLabel] = useState<string>('');
  const [editRequired, setEditRequired] = useState<boolean>(false);
  const [editMinNumber, setEditMinNumber] = useState<string>('');
  const [editMaxNumber, setEditMaxNumber] = useState<string>('');
  const [editMaxLength, setEditMaxLength] = useState<string>('');
  const [editArchived, setEditArchived] = useState<boolean>(false);

  // Validation feedback
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);

  const loadDefinitions = useCallback(async (type: EntityType) => {
    const request = requestGuard.begin();
    setIsLoading(true);
    setError(null);
    try {
      const data = await attributeDefinitionsApi.getByEntityType(type);
      if (!requestGuard.isCurrent(request)) return;
      setDefinitions(data);
    } catch (err) {
      if (!requestGuard.isCurrent(request)) return;
      const msg = err instanceof ApiError ? err.message : 'Failed to load attribute definitions';
      setError(msg);
    } finally {
      if (requestGuard.isCurrent(request)) setIsLoading(false);
    }
  }, [requestGuard]);

  useEffect(() => {
    if (isOpen) {
      loadDefinitions(selectedType);
      setModalMode('list');
      setEditingDefinition(null);
      setIsSubmitting(false);
      setFormErrors({});
    }
    return () => requestGuard.invalidate();
  }, [isOpen, selectedType, loadDefinitions, requestGuard]);

  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const handleStartCreate = () => {
    setCreateKey('');
    setCreateLabel('');
    setCreateValueType('string');
    setCreateRequired(false);
    setCreateMinNumber('');
    setCreateMaxNumber('');
    setCreateMaxLength('');
    setFormErrors({});
    setError(null);
    setModalMode('create');
  };

  const handleStartEdit = (def: AttributeDefinition) => {
    setEditingDefinition(def);
    setEditLabel(def.label);
    setEditRequired(def.required);
    setEditMinNumber(def.min_number !== null && def.min_number !== undefined ? String(def.min_number) : '');
    setEditMaxNumber(def.max_number !== null && def.max_number !== undefined ? String(def.max_number) : '');
    setEditMaxLength(def.max_length !== null && def.max_length !== undefined ? String(def.max_length) : '');
    setEditArchived(def.archived);
    setFormErrors({});
    setError(null);
    setModalMode('edit');
  };

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const input: CreateAttributeDefinitionInput = {
      entity_type: selectedType,
      key: createKey.trim(),
      label: createLabel.trim(),
      value_type: createValueType,
      required: createRequired,
      min_number: createMinNumber !== '' ? Number(createMinNumber) : null,
      max_number: createMaxNumber !== '' ? Number(createMaxNumber) : null,
      max_length: createMaxLength !== '' ? Number(createMaxLength) : null,
    };

    const errors = validateAttributeDefinition(input, true);
    if (Object.keys(errors).length > 0) {
      setFormErrors(errors);
      return;
    }

    const request = requestGuard.begin();
    setIsSubmitting(true);
    setError(null);
    try {
      const created = await attributeDefinitionsApi.create(input);
      if (!requestGuard.isCurrent(request)) return;
      if (isOpenRef.current && selectedTypeRef.current === created.entity_type) {
        setDefinitions((prev) => [...prev, created].sort((a, b) => a.key.localeCompare(b.key)));
        setModalMode('list');
      }
      onToast('success', `Definition "${created.label}" (${created.key}) created.`, 'Created');
    } catch (err) {
      if (!requestGuard.isCurrent(request)) return;
      const msg = err instanceof ApiError ? err.message : 'Failed to create definition';
      setError(msg);
      onToast('error', msg, 'Create Error');
    } finally {
      if (requestGuard.isCurrent(request)) setIsSubmitting(false);
    }
  };

  const handleEditSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingDefinition) return;

    const input: UpdateAttributeDefinitionInput = {
      label: editLabel.trim(),
      required: editRequired,
      min_number: editMinNumber !== '' ? Number(editMinNumber) : null,
      max_number: editMaxNumber !== '' ? Number(editMaxNumber) : null,
      max_length: editMaxLength !== '' ? Number(editMaxLength) : null,
      archived: editArchived,
    };

    const errors = validateAttributeDefinition(
      { ...input, value_type: editingDefinition.value_type },
      false
    );
    if (Object.keys(errors).length > 0) {
      setFormErrors(errors);
      return;
    }

    const request = requestGuard.begin();
    setIsSubmitting(true);
    setError(null);
    try {
      const updated = await attributeDefinitionsApi.update(editingDefinition.id, input);
      if (!requestGuard.isCurrent(request)) return;
      if (isOpenRef.current && selectedTypeRef.current === updated.entity_type) {
        setDefinitions((prev) => prev.map((d) => (d.id === updated.id ? updated : d)));
        setModalMode('list');
        setEditingDefinition(null);
      }
      onToast('success', `Definition "${updated.label}" updated.`, 'Saved');
    } catch (err) {
      if (!requestGuard.isCurrent(request)) return;
      const msg = err instanceof ApiError ? err.message : 'Failed to update definition';
      setError(msg);
      onToast('error', msg, 'Update Error');
    } finally {
      if (requestGuard.isCurrent(request)) setIsSubmitting(false);
    }
  };

  if (!isOpen) return null;

  const activeCount = definitions.filter((d) => !d.archived).length;

  return (
    <div className="modal-overlay animate-fade-in" onClick={onClose} role="presentation">
      <div
        className="modal-dialog modal-lg attribute-definitions-dialog"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="def-modal-title"
      >
        <div className="modal-header">
          <div className="modal-title-row">
            <h2 id="def-modal-title" className="modal-title">
              Attribute Definitions
            </h2>
            <span className="badge-count">{activeCount} / 50 active definitions</span>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-icon-only"
            onClick={onClose}
            aria-label="Close dialog"
          >
            <CloseIcon size={18} />
          </button>
        </div>

        {/* Entity Type Selector Tabs */}
        <div className="entity-type-tabs" role="tablist">
          {ENTITY_TYPES.map((t) => (
            <button
              key={t.type}
              type="button"
              role="tab"
              aria-selected={selectedType === t.type}
              className={`type-tab-btn ${selectedType === t.type ? 'active' : ''}`}
              onClick={() => {
                setSelectedType(t.type);
                setModalMode('list');
                setEditingDefinition(null);
              }}
            >
              {t.icon}
              <span>{t.label}</span>
            </button>
          ))}
        </div>

        <div className="modal-body">
          {error && <div className="form-error-banner mb-3">{error}</div>}

          {modalMode === 'list' && (
            <div className="definitions-list-view">
              <div className="def-view-header">
                <span className="text-sm text-muted">
                  Custom typed schema properties for{' '}
                  <strong className="capitalize">{selectedType.replace('_', ' ')}</strong> entities
                </span>
                <button
                  type="button"
                  className="btn btn-primary btn-sm"
                  onClick={handleStartCreate}
                  disabled={activeCount >= 50}
                  title={activeCount >= 50 ? 'Maximum 50 active definitions reached' : 'Add new attribute'}
                >
                  <PlusIcon size={14} />
                  <span>New Definition</span>
                </button>
              </div>

              {isLoading ? (
                <div className="section-loading">
                  <RefreshIcon size={18} className="animate-spin text-muted" />
                  <span>Loading definitions...</span>
                </div>
              ) : definitions.length === 0 ? (
                <div className="empty-section-card">
                  <AlertIcon size={24} className="text-muted" />
                  <p>No attribute definitions defined for {selectedType.replace('_', ' ')} entities.</p>
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    onClick={handleStartCreate}
                  >
                    <PlusIcon size={14} />
                    <span>Create First Definition</span>
                  </button>
                </div>
              ) : (
                <div className="definitions-table-wrapper">
                  <table className="definitions-table">
                    <thead>
                      <tr>
                        <th>Key</th>
                        <th>Label</th>
                        <th>Type</th>
                        <th>Required</th>
                        <th>Constraints</th>
                        <th>Status</th>
                        <th>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {definitions.map((def) => (
                        <tr key={def.id} className={def.archived ? 'row-archived' : ''}>
                          <td>
                            <code>{def.key}</code>
                          </td>
                          <td>
                            <strong>{def.label}</strong>
                          </td>
                          <td>
                            <span className="badge-type-label">{def.value_type}</span>
                          </td>
                          <td>
                            {def.required ? (
                              <span className="badge-required">Required</span>
                            ) : (
                              <span className="text-muted text-xs">Optional</span>
                            )}
                          </td>
                          <td>
                            <span className="text-xs text-muted">
                              {def.value_type === 'number' && (
                                <>
                                  {def.min_number !== null && def.min_number !== undefined && `Min: ${def.min_number} `}
                                  {def.max_number !== null && def.max_number !== undefined && `Max: ${def.max_number}`}
                                  {def.min_number === null && def.max_number === null && 'None'}
                                </>
                              )}
                              {def.value_type === 'string' && (
                                <>
                                  {def.max_length ? `Max: ${def.max_length} chars` : 'None'}
                                </>
                              )}
                              {def.value_type === 'boolean' && 'True / False'}
                            </span>
                          </td>
                          <td>
                            {def.archived ? (
                              <span className="badge-archived">Archived</span>
                            ) : (
                              <span className="badge-active">Active</span>
                            )}
                          </td>
                          <td>
                            <button
                              type="button"
                              className="btn btn-secondary btn-xs"
                              onClick={() => handleStartEdit(def)}
                              title="Edit definition"
                            >
                              <EditIcon size={12} />
                              <span>Edit</span>
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {modalMode === 'create' && (
            <form onSubmit={handleCreateSubmit} className="definition-form">
              <div className="def-form-header">
                <h3>Add New Attribute Definition</h3>
                <span className="text-xs text-muted">
                  Creating attribute for <strong>{selectedType.replace('_', ' ')}</strong>
                </span>
              </div>

              <div className="form-grid">
                <div className="form-group">
                  <label htmlFor={`${formId}-create-key`} className="form-label">
                    Attribute Key <span className="text-danger">*</span>
                  </label>
                  <input
                    id={`${formId}-create-key`}
                    type="text"
                    className={`form-input ${formErrors.key ? 'input-error' : ''}`}
                    placeholder="e.g. battery_level, vin, serial_no"
                    value={createKey}
                    onChange={(e) => setCreateKey(e.target.value)}
                  />
                  {formErrors.key ? (
                    <span className="field-error-msg">{formErrors.key}</span>
                  ) : (
                    <span className="field-hint">
                      Regex: <code>^[a-z][a-z0-9_]{'{0,63}'}$</code> (Immutable once created)
                    </span>
                  )}
                </div>

                <div className="form-group">
                  <label htmlFor={`${formId}-create-label`} className="form-label">
                    Display Label <span className="text-danger">*</span>
                  </label>
                  <input
                    id={`${formId}-create-label`}
                    type="text"
                    className={`form-input ${formErrors.label ? 'input-error' : ''}`}
                    placeholder="e.g. Battery Level, VIN"
                    maxLength={100}
                    value={createLabel}
                    onChange={(e) => setCreateLabel(e.target.value)}
                  />
                  {formErrors.label && (
                    <span className="field-error-msg">{formErrors.label}</span>
                  )}
                </div>

                <div className="form-group">
                  <label htmlFor={`${formId}-create-val-type`} className="form-label">
                    Value Type <span className="text-danger">*</span>
                  </label>
                  <select
                    id={`${formId}-create-val-type`}
                    className="form-select"
                    value={createValueType}
                    onChange={(e) =>
                      setCreateValueType(e.target.value as 'string' | 'number' | 'boolean')
                    }
                  >
                    <option value="string">String (Text)</option>
                    <option value="number">Number (Float/Integer)</option>
                    <option value="boolean">Boolean (True/False)</option>
                  </select>
                  <span className="field-hint">Immutable once created</span>
                </div>

                <div className="form-group flex-center-start">
                  <label className="checkbox-toggle-label mt-4">
                    <input
                      type="checkbox"
                      checked={createRequired}
                      onChange={(e) => setCreateRequired(e.target.checked)}
                    />
                    <span>Required Attribute</span>
                  </label>
                </div>

                {createValueType === 'number' && (
                  <>
                    <div className="form-group">
                      <label htmlFor={`${formId}-create-min-num`} className="form-label">Minimum Number</label>
                      <input
                        id={`${formId}-create-min-num`}
                        type="number"
                        step="any"
                        className={`form-input ${formErrors.min_number ? 'input-error' : ''}`}
                        placeholder="Optional min bound"
                        value={createMinNumber}
                        onChange={(e) => setCreateMinNumber(e.target.value)}
                      />
                      {formErrors.min_number && (
                        <span className="field-error-msg">{formErrors.min_number}</span>
                      )}
                    </div>
                    <div className="form-group">
                      <label htmlFor={`${formId}-create-max-num`} className="form-label">Maximum Number</label>
                      <input
                        id={`${formId}-create-max-num`}
                        type="number"
                        step="any"
                        className={`form-input ${formErrors.max_number ? 'input-error' : ''}`}
                        placeholder="Optional max bound"
                        value={createMaxNumber}
                        onChange={(e) => setCreateMaxNumber(e.target.value)}
                      />
                      {formErrors.max_number && (
                        <span className="field-error-msg">{formErrors.max_number}</span>
                      )}
                    </div>
                  </>
                )}

                {createValueType === 'string' && (
                  <div className="form-group">
                    <label htmlFor={`${formId}-create-max-len`} className="form-label">Maximum Text Length</label>
                    <input
                      id={`${formId}-create-max-len`}
                      type="number"
                      min={1}
                      max={2048}
                      className={`form-input ${formErrors.max_length ? 'input-error' : ''}`}
                      placeholder="1 to 2048 characters"
                      value={createMaxLength}
                      onChange={(e) => setCreateMaxLength(e.target.value)}
                    />
                    {formErrors.max_length && (
                      <span className="field-error-msg">{formErrors.max_length}</span>
                    )}
                  </div>
                )}
              </div>

              <div className="modal-footer">
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setModalMode('list')}
                  disabled={isSubmitting}
                >
                  <CloseIcon size={16} />
                  <span>Cancel</span>
                </button>
                <button type="submit" className="btn btn-primary" disabled={isSubmitting}>
                  <CheckIcon size={16} />
                  <span>{isSubmitting ? 'Creating...' : 'Create Definition'}</span>
                </button>
              </div>
            </form>
          )}

          {modalMode === 'edit' && editingDefinition && (
            <form onSubmit={handleEditSubmit} className="definition-form">
              <div className="def-form-header">
                <h3>Edit Attribute Definition: <code>{editingDefinition.key}</code></h3>
                <span className="text-xs text-muted">
                  Key and Value Type are immutable once created.
                </span>
              </div>

              <div className="form-grid">
                <div className="form-group">
                  <label htmlFor={`${formId}-edit-key-ro`} className="form-label">Attribute Key (Immutable)</label>
                  <input
                    id={`${formId}-edit-key-ro`}
                    type="text"
                    className="form-input form-input-disabled"
                    disabled
                    value={editingDefinition.key}
                  />
                </div>

                <div className="form-group">
                  <label htmlFor={`${formId}-edit-val-type-ro`} className="form-label">Value Type (Immutable)</label>
                  <input
                    id={`${formId}-edit-val-type-ro`}
                    type="text"
                    className="form-input form-input-disabled capitalize"
                    disabled
                    value={editingDefinition.value_type}
                  />
                </div>

                <div className="form-group">
                  <label htmlFor={`${formId}-edit-label`} className="form-label">
                    Display Label <span className="text-danger">*</span>
                  </label>
                  <input
                    id={`${formId}-edit-label`}
                    type="text"
                    className={`form-input ${formErrors.label ? 'input-error' : ''}`}
                    maxLength={100}
                    value={editLabel}
                    onChange={(e) => setEditLabel(e.target.value)}
                  />
                  {formErrors.label && (
                    <span className="field-error-msg">{formErrors.label}</span>
                  )}
                </div>

                <div className="form-group flex-center-start">
                  <label className="checkbox-toggle-label mt-4">
                    <input
                      type="checkbox"
                      checked={editRequired}
                      onChange={(e) => setEditRequired(e.target.checked)}
                    />
                    <span>Required Attribute</span>
                  </label>
                </div>

                {editingDefinition.value_type === 'number' && (
                  <>
                    <div className="form-group">
                      <label htmlFor={`${formId}-edit-min-num`} className="form-label">Minimum Number</label>
                      <input
                        id={`${formId}-edit-min-num`}
                        type="number"
                        step="any"
                        className={`form-input ${formErrors.min_number ? 'input-error' : ''}`}
                        placeholder="Optional min bound"
                        value={editMinNumber}
                        onChange={(e) => setEditMinNumber(e.target.value)}
                      />
                      {formErrors.min_number && (
                        <span className="field-error-msg">{formErrors.min_number}</span>
                      )}
                    </div>
                    <div className="form-group">
                      <label htmlFor={`${formId}-edit-max-num`} className="form-label">Maximum Number</label>
                      <input
                        id={`${formId}-edit-max-num`}
                        type="number"
                        step="any"
                        className={`form-input ${formErrors.max_number ? 'input-error' : ''}`}
                        placeholder="Optional max bound"
                        value={editMaxNumber}
                        onChange={(e) => setEditMaxNumber(e.target.value)}
                      />
                      {formErrors.max_number && (
                        <span className="field-error-msg">{formErrors.max_number}</span>
                      )}
                    </div>
                  </>
                )}

                {editingDefinition.value_type === 'string' && (
                  <div className="form-group">
                    <label htmlFor={`${formId}-edit-max-len`} className="form-label">Maximum Text Length</label>
                    <input
                      id={`${formId}-edit-max-len`}
                      type="number"
                      min={1}
                      max={2048}
                      className={`form-input ${formErrors.max_length ? 'input-error' : ''}`}
                      placeholder="1 to 2048 characters"
                      value={editMaxLength}
                      onChange={(e) => setEditMaxLength(e.target.value)}
                    />
                    {formErrors.max_length && (
                      <span className="field-error-msg">{formErrors.max_length}</span>
                    )}
                  </div>
                )}

                <div className="form-group col-span-2">
                  <label className="checkbox-toggle-label archive-toggle">
                    <input
                      type="checkbox"
                      checked={editArchived}
                      onChange={(e) => setEditArchived(e.target.checked)}
                    />
                    <div>
                      <span className="font-semibold">Archive this definition</span>
                      <p className="text-xs text-muted">
                        Archived definitions cannot be populated for new entries, but preserve
                        historical values on existing entities.
                      </p>
                    </div>
                  </label>
                </div>
              </div>

              <div className="modal-footer">
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => {
                    setModalMode('list');
                    setEditingDefinition(null);
                  }}
                  disabled={isSubmitting}
                >
                  <CloseIcon size={16} />
                  <span>Cancel</span>
                </button>
                <button type="submit" className="btn btn-primary" disabled={isSubmitting}>
                  <CheckIcon size={16} />
                  <span>{isSubmitting ? 'Saving...' : 'Save Changes'}</span>
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
