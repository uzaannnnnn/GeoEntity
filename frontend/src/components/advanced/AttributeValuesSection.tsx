import { useState, useEffect, useCallback, useId, useRef } from 'react';
import { Entity } from '../../types/entity';
import { AttributeDefinition } from '../../types/advanced';
import { attributeDefinitionsApi, entityAttributesApi } from '../../api/advanced';
import { ApiError } from '../../api/client';
import { validateAttributeValues } from '../../utils/attributeHelpers';
import { RequestGuard } from '../../utils/requestGuard';
import { EditIcon, CheckIcon, CloseIcon, RefreshIcon, InfoIcon } from '../Icons';

interface AttributeValuesSectionProps {
  entity: Entity;
  onToast: (type: 'success' | 'error' | 'info', message: string, title?: string) => void;
}

export function AttributeValuesSection({ entity, onToast }: AttributeValuesSectionProps) {
  const requestGuard = useRef(new RequestGuard()).current;
  const formId = useId();
  const [definitions, setDefinitions] = useState<AttributeDefinition[]>([]);
  const [currentValues, setCurrentValues] = useState<Record<string, any>>({});
  const [previousTypeValues, setPreviousTypeValues] = useState<Record<string, any>>({});
  const [draftValues, setDraftValues] = useState<Record<string, any>>({});
  const [isEditing, setIsEditing] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isSaving, setIsSaving] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});

  const loadData = useCallback(async () => {
    const request = requestGuard.begin();
    setIsLoading(true);
    setError(null);
    try {
      const [defs, attrData] = await Promise.all([
        attributeDefinitionsApi.getByEntityType(entity.type),
        entityAttributesApi.get(entity.id),
      ]);
      if (!requestGuard.isCurrent(request)) return;
      setDefinitions(defs);
      const vals = attrData.values || {};
      setCurrentValues(vals);
      setDraftValues(vals);
      setPreviousTypeValues(attrData.previous_type_values || {});
      setIsEditing(false);
      setValidationErrors({});
    } catch (err) {
      if (!requestGuard.isCurrent(request)) return;
      const msg = err instanceof ApiError ? err.message : 'Failed to load entity attributes';
      setError(msg);
    } finally {
      if (requestGuard.isCurrent(request)) setIsLoading(false);
    }
  }, [entity.id, entity.type, requestGuard]);

  useEffect(() => {
    loadData();
    return () => requestGuard.invalidate();
  }, [loadData, requestGuard]);

  const handleStartEdit = () => {
    setDraftValues({ ...currentValues });
    setValidationErrors({});
    setIsEditing(true);
  };

  const handleCancelEdit = () => {
    setDraftValues({ ...currentValues });
    setValidationErrors({});
    setIsEditing(false);
  };

  const handleValueChange = (key: string, value: any) => {
    setDraftValues((prev) => ({
      ...prev,
      [key]: value,
    }));
    if (validationErrors[key]) {
      setValidationErrors((prev) => {
        const next = { ...prev };
        delete next[key];
        return next;
      });
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    const errors = validateAttributeValues(definitions, draftValues);
    if (Object.keys(errors).length > 0) {
      setValidationErrors(errors);
      return;
    }

    const request = requestGuard.begin();
    setIsSaving(true);
    setError(null);
    try {
      // Clean draft values: keep only keys that exist in active definitions
      const activeKeys = new Set(definitions.filter((d) => !d.archived).map((d) => d.key));
      const payload: Record<string, any> = {};

      for (const [k, v] of Object.entries(draftValues)) {
        if (!activeKeys.has(k)) continue;
        if (v === '' || v === undefined || v === null) continue;

        const def = definitions.find((d) => d.key === k);
        if (def?.value_type === 'number') {
          payload[k] = Number(v);
        } else if (def?.value_type === 'boolean') {
          payload[k] = Boolean(v);
        } else {
          payload[k] = String(v);
        }
      }

      const saved = await entityAttributesApi.replace(entity.id, payload);
      if (!requestGuard.isCurrent(request)) return;
      setCurrentValues(saved);
      setDraftValues(saved);
      setIsEditing(false);
      onToast('success', 'Custom attributes updated successfully.', 'Saved');
    } catch (err) {
      if (!requestGuard.isCurrent(request)) return;
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : 'Failed to update attributes';
      setError(msg);
      onToast('error', msg, 'Save Error');
    } finally {
      if (requestGuard.isCurrent(request)) setIsSaving(false);
    }
  };

  if (isLoading) {
    return (
      <div className="section-loading">
        <RefreshIcon size={18} className="animate-spin text-muted" />
        <span>Loading attributes...</span>
      </div>
    );
  }

  if (error && !isEditing && definitions.length === 0) {
    return (
      <div className="section-error">
        <p className="text-danger">{error}</p>
        <button type="button" className="btn btn-secondary btn-sm" onClick={loadData}>
          <RefreshIcon size={14} />
          <span>Retry</span>
        </button>
      </div>
    );
  }

  const activeDefs = definitions.filter((d) => !d.archived);
  const archivedDefs = definitions.filter((d) => d.archived);
  const hasPreviousValues = Object.keys(previousTypeValues).length > 0;

  return (
    <div className="attributes-section">
      <div className="section-header-actions">
        <div>
          <h3 className="section-title">Custom Attributes</h3>
          <span className="section-subtitle">
            Entity Type: <strong className="capitalize">{entity.type.replace('_', ' ')}</strong>
          </span>
        </div>
        {!isEditing && activeDefs.length > 0 && (
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={handleStartEdit}
            title="Edit attributes"
          >
            <EditIcon size={14} />
            <span>Edit Attributes</span>
          </button>
        )}
      </div>

      {activeDefs.length === 0 ? (
        <div className="empty-section-card">
          <InfoIcon size={20} className="text-muted" />
          <p>No active attribute definitions exist for {entity.type.replace('_', ' ')} entities.</p>
          <span className="text-muted text-xs">
            Use the "Attributes" button in the top navigation to define custom properties.
          </span>
        </div>
      ) : isEditing ? (
        <form onSubmit={handleSave} className="attributes-form">
          {error && <div className="form-error-banner">{error}</div>}

          <div className="attr-form-fields">
            {activeDefs.map((def) => {
              const val = draftValues[def.key];
              const fieldError = validationErrors[def.key];
              const inputId = `${formId}-attr-${def.key}`;

              return (
                <div key={def.id} className="form-group">
                  <div className="attr-field-label-row">
                    <label htmlFor={inputId} className="form-label">
                      {def.label}
                      {def.required && <span className="text-danger ml-1">*</span>}
                    </label>
                    <span className="badge-type-label">{def.value_type}</span>
                  </div>

                  {def.value_type === 'string' && (
                    <input
                      id={inputId}
                      type="text"
                      className={`form-input ${fieldError ? 'input-error' : ''}`}
                      value={val !== undefined && val !== null ? String(val) : ''}
                      maxLength={def.max_length ?? 2048}
                      placeholder={
                        def.max_length ? `Max ${def.max_length} characters` : 'Enter text'
                      }
                      onChange={(e) => handleValueChange(def.key, e.target.value)}
                    />
                  )}

                  {def.value_type === 'number' && (
                    <input
                      id={inputId}
                      type="number"
                      step="any"
                      className={`form-input ${fieldError ? 'input-error' : ''}`}
                      value={val !== undefined && val !== null ? String(val) : ''}
                      min={def.min_number !== null ? def.min_number : undefined}
                      max={def.max_number !== null ? def.max_number : undefined}
                      placeholder={
                        def.min_number !== null && def.max_number !== null
                          ? `Between ${def.min_number} and ${def.max_number}`
                          : 'Enter number'
                      }
                      onChange={(e) =>
                        handleValueChange(
                          def.key,
                          e.target.value === '' ? '' : Number(e.target.value)
                        )
                      }
                    />
                  )}

                  {def.value_type === 'boolean' && (
                    <label className="checkbox-toggle-label">
                      <input
                        id={inputId}
                        type="checkbox"
                        checked={Boolean(val)}
                        onChange={(e) => handleValueChange(def.key, e.target.checked)}
                      />
                      <span>{val ? 'Enabled / True' : 'Disabled / False'}</span>
                    </label>
                  )}

                  {fieldError ? (
                    <span className="field-error-msg">{fieldError}</span>
                  ) : (
                    <span className="field-hint">
                      Key: <code>{def.key}</code>
                      {def.min_number !== null &&
                        def.min_number !== undefined &&
                        ` | Min: ${def.min_number}`}
                      {def.max_number !== null &&
                        def.max_number !== undefined &&
                        ` | Max: ${def.max_number}`}
                      {def.max_length !== null &&
                        def.max_length !== undefined &&
                        ` | Max length: ${def.max_length}`}
                    </span>
                  )}
                </div>
              );
            })}
          </div>

          <div className="form-actions-row">
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={handleCancelEdit}
              disabled={isSaving}
            >
              <CloseIcon size={14} />
              <span>Cancel</span>
            </button>
            <button type="submit" className="btn btn-primary btn-sm" disabled={isSaving}>
              <CheckIcon size={14} />
              <span>{isSaving ? 'Saving...' : 'Save Attributes'}</span>
            </button>
          </div>
        </form>
      ) : (
        <div className="attr-display-grid">
          {activeDefs.map((def) => {
            const val = currentValues[def.key];
            const hasValue = val !== undefined && val !== null && val !== '';

            return (
              <div key={def.id} className="attr-display-card">
                <div className="attr-card-header">
                  <span className="attr-label">{def.label}</span>
                  <div className="attr-card-badges">
                    {def.required && <span className="badge-required">Required</span>}
                    <span className="badge-type-label">{def.value_type}</span>
                  </div>
                </div>
                <div className="attr-card-value">
                  {hasValue ? (
                    def.value_type === 'boolean' ? (
                      <span className={`boolean-badge ${val ? 'bool-true' : 'bool-false'}`}>
                        {val ? 'Yes' : 'No'}
                      </span>
                    ) : (
                      <code>{String(val)}</code>
                    )
                  ) : (
                    <span className="text-muted italic">Not set</span>
                  )}
                </div>
                <div className="attr-card-footer">
                  <span className="text-xs text-muted">
                    Key: <code>{def.key}</code>
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Archived definitions with existing values */}
      {archivedDefs.length > 0 && (
        <div className="archived-attr-section mt-4">
          <h4 className="subsection-title text-muted">Archived Definitions</h4>
          <div className="attr-display-grid">
            {archivedDefs.map((def) => {
              const val = currentValues[def.key];
              return (
                <div key={def.id} className="attr-display-card card-archived">
                  <div className="attr-card-header">
                    <span className="attr-label text-muted">{def.label}</span>
                    <span className="badge-archived">Archived</span>
                  </div>
                  <div className="attr-card-value">
                    {val !== undefined && val !== null ? (
                      <code>{String(val)}</code>
                    ) : (
                      <span className="text-muted italic">—</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Preserved Previous Type Values */}
      {hasPreviousValues && (
        <div className="previous-type-section mt-4">
          <div className="previous-type-header">
            <InfoIcon size={16} className="text-muted" />
            <h4 className="subsection-title">Previous Type Attributes (Preserved)</h4>
          </div>
          <p className="text-xs text-muted mb-2">
            These values belonged to earlier configurations of this entity when it had a different
            type. They are retained read-only for historical continuity.
          </p>
          <div className="previous-type-grid">
            {Object.entries(previousTypeValues).map(([key, val]) => (
              <div key={key} className="previous-attr-row">
                <code className="prev-key">{key}</code>
                <span className="prev-arrow">&rarr;</span>
                <span className="prev-val">{String(val)}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
