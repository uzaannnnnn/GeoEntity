import { useState, useEffect, type FormEvent } from 'react';
import { Entity, EntityInput, EntityStatus, EntityType } from '../types/entity';
import { validateEntityInput, ValidationErrors, ALLOWED_TYPES, ALLOWED_STATUSES } from '../utils/validation';
import { formatTypeLabel, formatStatusLabel } from '../utils/formatters';
import { CrosshairIcon, AlertIcon, ArrowLeftIcon } from './Icons';

interface EntityFormProps {
  mode: 'create' | 'edit';
  initialEntity?: Entity | null;
  draftLocation: { lat: number; lng: number } | null;
  isSelectingLocation: boolean;
  onToggleSelectLocation: () => void;
  onSubmit: (data: EntityInput) => Promise<void>;
  onCancel: () => void;
  backendError: string | null;
  backendErrorField?: string;
}

export function EntityForm({
  mode,
  initialEntity,
  draftLocation,
  isSelectingLocation,
  onToggleSelectLocation,
  onSubmit,
  onCancel,
  backendError,
  backendErrorField,
}: EntityFormProps) {
  const [formData, setFormData] = useState<EntityInput>({
    name: initialEntity?.name ?? '',
    type: initialEntity?.type ?? 'vehicle',
    description: initialEntity?.description ?? '',
    status: initialEntity?.status ?? 'active',
    latitude: initialEntity?.latitude ?? (draftLocation ? draftLocation.lat : 0),
    longitude: initialEntity?.longitude ?? (draftLocation ? draftLocation.lng : 0),
  });

  const [latInput, setLatInput] = useState<string>(
    initialEntity?.latitude !== undefined
      ? String(initialEntity.latitude)
      : draftLocation
      ? String(draftLocation.lat)
      : ''
  );

  const [lngInput, setLngInput] = useState<string>(
    initialEntity?.longitude !== undefined
      ? String(initialEntity.longitude)
      : draftLocation
      ? String(draftLocation.lng)
      : ''
  );

  const [errors, setErrors] = useState<ValidationErrors>({});
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Sync draft location when picked from map
  useEffect(() => {
    if (draftLocation) {
      setLatInput(String(draftLocation.lat));
      setLngInput(String(draftLocation.lng));
      setFormData((prev) => ({
        ...prev,
        latitude: draftLocation.lat,
        longitude: draftLocation.lng,
      }));
      setErrors((prev) => ({
        ...prev,
        latitude: undefined,
        longitude: undefined,
      }));
    }
  }, [draftLocation]);

  // Sync backend field error if provided
  useEffect(() => {
    if (backendErrorField) {
      setErrors((prev) => ({
        ...prev,
        [backendErrorField]: backendError || 'Invalid field value',
      }));
    }
  }, [backendError, backendErrorField]);

  const handleFieldChange = (
    field: keyof EntityInput,
    value: string
  ) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    if (errors[field]) {
      setErrors((prev) => ({ ...prev, [field]: undefined }));
    }
  };

  const handleLatChange = (value: string) => {
    setLatInput(value);
    const parsed = parseFloat(value);
    setFormData((prev) => ({
      ...prev,
      latitude: Number.isNaN(parsed) ? (undefined as unknown as number) : parsed,
    }));
    if (errors.latitude) {
      setErrors((prev) => ({ ...prev, latitude: undefined }));
    }
  };

  const handleLngChange = (value: string) => {
    setLngInput(value);
    const parsed = parseFloat(value);
    setFormData((prev) => ({
      ...prev,
      longitude: Number.isNaN(parsed) ? (undefined as unknown as number) : parsed,
    }));
    if (errors.longitude) {
      setErrors((prev) => ({ ...prev, longitude: undefined }));
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();

    // Prepare input with trimmed name
    const payload: EntityInput = {
      name: formData.name.trim(),
      type: formData.type,
      description: formData.description,
      status: formData.status,
      latitude: parseFloat(latInput),
      longitude: parseFloat(lngInput),
    };

    // Client-side validation
    const validationErrors = validateEntityInput(payload);
    if (Object.keys(validationErrors).length > 0) {
      setErrors(validationErrors);
      return;
    }

    setIsSubmitting(true);
    try {
      await onSubmit(payload);
    } catch {
      // Handled by parent via backendError / backendErrorField
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="entity-form-panel animate-fade-in">
      <div className="form-top-nav">
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={onCancel}
          disabled={isSubmitting}
        >
          <ArrowLeftIcon size={16} />
          <span>Cancel</span>
        </button>
        <h3 className="form-heading-title">
          {mode === 'create' ? 'Create New Entity' : 'Edit Entity'}
        </h3>
      </div>

      <form onSubmit={handleSubmit} className="entity-form" noValidate>
        {backendError && (
          <div className="form-alert form-alert-danger animate-fade-in">
            <AlertIcon size={18} className="alert-icon" />
            <div className="alert-text">
              <strong>Server error:</strong> {backendError}
            </div>
          </div>
        )}

        {/* Entity Name */}
        <div className="form-group">
          <label htmlFor="entity-name" className="form-label">
            Name <span className="text-danger">*</span>
          </label>
          <input
            id="entity-name"
            type="text"
            className={`form-input ${errors.name ? 'input-error' : ''}`}
            placeholder="e.g. Delivery Van #42 or Temperature Sensor 01"
            value={formData.name}
            onChange={(e) => handleFieldChange('name', e.target.value)}
            disabled={isSubmitting}
            autoFocus
          />
          {errors.name && <span className="field-error-msg">{errors.name}</span>}
        </div>

        {/* Type & Status Grid */}
        <div className="form-row-2">
          <div className="form-group">
            <label htmlFor="entity-type" className="form-label">
              Type <span className="text-danger">*</span>
            </label>
            <select
              id="entity-type"
              className={`form-select ${errors.type ? 'input-error' : ''}`}
              value={formData.type}
              onChange={(e) => handleFieldChange('type', e.target.value as EntityType)}
              disabled={isSubmitting}
            >
              {ALLOWED_TYPES.map((t) => (
                <option key={t} value={t}>
                  {formatTypeLabel(t)}
                </option>
              ))}
            </select>
            {errors.type && <span className="field-error-msg">{errors.type}</span>}
          </div>

          <div className="form-group">
            <label htmlFor="entity-status" className="form-label">
              Status <span className="text-danger">*</span>
            </label>
            <select
              id="entity-status"
              className={`form-select ${errors.status ? 'input-error' : ''}`}
              value={formData.status}
              onChange={(e) => handleFieldChange('status', e.target.value as EntityStatus)}
              disabled={isSubmitting}
            >
              {ALLOWED_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {formatStatusLabel(s)}
                </option>
              ))}
            </select>
            {errors.status && <span className="field-error-msg">{errors.status}</span>}
          </div>
        </div>

        {/* Description */}
        <div className="form-group">
          <label htmlFor="entity-desc" className="form-label">
            Description <span className="text-muted">(Optional)</span>
          </label>
          <textarea
            id="entity-desc"
            rows={3}
            className="form-textarea"
            placeholder="Additional context, notes, or identifier details..."
            value={formData.description}
            onChange={(e) => handleFieldChange('description', e.target.value)}
            disabled={isSubmitting}
          />
        </div>

        {/* Geographic Coordinates Picker & Manual inputs */}
        <div className="form-coordinates-section">
          <div className="coord-section-header">
            <div>
              <span className="form-label m-0">Location Coordinates</span>
              <p className="coord-section-sub">
                Click the map to place/move the marker, or enter coordinates manually.
              </p>
            </div>
            <button
              type="button"
              className={`btn btn-xs ${isSelectingLocation ? 'btn-highlight' : 'btn-secondary'}`}
              onClick={onToggleSelectLocation}
              disabled={isSubmitting}
            >
              <CrosshairIcon size={14} />
              <span>{isSelectingLocation ? 'Done Picking' : 'Pick on Map'}</span>
            </button>
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label htmlFor="entity-lat" className="form-label">
                Latitude <span className="text-danger">*</span>
              </label>
              <input
                id="entity-lat"
                type="number"
                step="any"
                min="-90"
                max="90"
                className={`form-input font-mono ${errors.latitude ? 'input-error' : ''}`}
                placeholder="-90 to 90"
                value={latInput}
                onChange={(e) => handleLatChange(e.target.value)}
                disabled={isSubmitting}
              />
              {errors.latitude && <span className="field-error-msg">{errors.latitude}</span>}
            </div>

            <div className="form-group">
              <label htmlFor="entity-lng" className="form-label">
                Longitude <span className="text-danger">*</span>
              </label>
              <input
                id="entity-lng"
                type="number"
                step="any"
                min="-180"
                max="180"
                className={`form-input font-mono ${errors.longitude ? 'input-error' : ''}`}
                placeholder="-180 to 180"
                value={lngInput}
                onChange={(e) => handleLngChange(e.target.value)}
                disabled={isSubmitting}
              />
              {errors.longitude && <span className="field-error-msg">{errors.longitude}</span>}
            </div>
          </div>
        </div>

        {/* Submit Actions */}
        <div className="form-actions">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onCancel}
            disabled={isSubmitting}
          >
            Cancel
          </button>
          <button
            type="submit"
            className="btn btn-primary"
            disabled={isSubmitting}
          >
            {isSubmitting ? (
              <span className="btn-loading-content">
                <span className="spinner-sm" />
                <span>Saving...</span>
              </span>
            ) : mode === 'create' ? (
              'Create Entity'
            ) : (
              'Save Changes'
            )}
          </button>
        </div>
      </form>
    </div>
  );
}
