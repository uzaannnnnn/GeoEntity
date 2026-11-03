import React, { useState } from 'react';
import { GeofenceDefinition, GeofenceGeometry, GeofenceInput } from '../../types/geofence';
import { ApiError } from '../../api/client';
import { AlertIcon, CrosshairIcon } from '../Icons';
import { validateGeofenceGeometry } from '../../utils/geofenceFormValidation';

interface GeofenceFormProps {
  initialGeofence?: GeofenceDefinition | null;
  mapCenter?: { lat: number; lng: number };
  onSave: (input: GeofenceInput) => Promise<void>;
  onCancel: () => void;
  activeCount?: number;
}

export function GeofenceForm({
  initialGeofence,
  mapCenter,
  onSave,
  onCancel,
  activeCount = 0,
}: GeofenceFormProps) {
  const isEditing = Boolean(initialGeofence);

  const [name, setName] = useState<string>(initialGeofence?.name || '');
  const [enabled, setEnabled] = useState<boolean>(initialGeofence?.enabled ?? true);
  const [geomType, setGeomType] = useState<'Circle' | 'Polygon'>(
    initialGeofence?.geometry.type || 'Circle'
  );

  // Circle inputs
  const [circleLat, setCircleLat] = useState<string>(
    initialGeofence?.geometry.type === 'Circle' && initialGeofence.geometry.center
      ? String(initialGeofence.geometry.center[1])
      : mapCenter
      ? String(mapCenter.lat.toFixed(6))
      : '-6.2088'
  );
  const [circleLon, setCircleLon] = useState<string>(
    initialGeofence?.geometry.type === 'Circle' && initialGeofence.geometry.center
      ? String(initialGeofence.geometry.center[0])
      : mapCenter
      ? String(mapCenter.lng.toFixed(6))
      : '106.8456'
  );
  const [circleRadius, setCircleRadius] = useState<string>(
    initialGeofence?.geometry.type === 'Circle' && initialGeofence.geometry.radius_m
      ? String(initialGeofence.geometry.radius_m)
      : '500'
  );

  // Polygon inputs (JSON string of coordinates: [[ [lon, lat], ... ]])
  const defaultPolyCoords = mapCenter
    ? [
        [
          [Number((mapCenter.lng - 0.01).toFixed(6)), Number((mapCenter.lat - 0.01).toFixed(6))],
          [Number((mapCenter.lng + 0.01).toFixed(6)), Number((mapCenter.lat - 0.01).toFixed(6))],
          [Number((mapCenter.lng + 0.01).toFixed(6)), Number((mapCenter.lat + 0.01).toFixed(6))],
          [Number((mapCenter.lng - 0.01).toFixed(6)), Number((mapCenter.lat + 0.01).toFixed(6))],
          [Number((mapCenter.lng - 0.01).toFixed(6)), Number((mapCenter.lat - 0.01).toFixed(6))],
        ],
      ]
    : [
        [
          [106.81, -6.21],
          [106.83, -6.21],
          [106.83, -6.23],
          [106.81, -6.23],
          [106.81, -6.21],
        ],
      ];

  const [polyCoordsJson, setPolyCoordsJson] = useState<string>(
    initialGeofence?.geometry.type === 'Polygon'
      ? JSON.stringify(initialGeofence.geometry.coordinates, null, 2)
      : JSON.stringify(defaultPolyCoords, null, 2)
  );

  // Validation & Submission
  const [errors, setErrors] = useState<{ [key: string]: string }>({});
  const [serverError, setServerError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);

  const validateForm = (): GeofenceGeometry | null => {
    const result = validateGeofenceGeometry(geomType, {
      name,
      circleLat,
      circleLon,
      circleRadius,
      polyCoordsJson,
    });
    setErrors(result.errors);
    return result.geometry;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setServerError(null);

    const validatedGeom = validateForm();
    if (!validatedGeom) return;

    setIsSubmitting(true);
    try {
      await onSave({
        name: name.trim(),
        enabled,
        geometry: validatedGeom,
      });
    } catch (err) {
      if (err instanceof ApiError) {
        setServerError(err.message);
        if (err.field) {
          setErrors((prev) => ({ ...prev, [err.field!]: err.message }));
        }
      } else {
        setServerError('An unexpected error occurred while saving the geofence.');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleUseMapCenter = () => {
    if (mapCenter) {
      setCircleLat(String(mapCenter.lat.toFixed(6)));
      setCircleLon(String(mapCenter.lng.toFixed(6)));
    }
  };

  return (
    <form className="geofence-form animate-fade-in" onSubmit={handleSubmit}>
      <div className="form-header-row">
        <h3 className="form-title">
          {isEditing && initialGeofence ? `Edit Geofence: ${initialGeofence.name}` : 'Create New Geofence'}
        </h3>
        <span className="form-subtitle">
          Define authoritative spatial boundary for entity containment monitoring.
        </span>
      </div>

      {serverError && (
        <div className="alert-box alert-error" role="alert">
          <AlertIcon size={16} />
          <div className="alert-content">
            <strong>Save Failed:</strong> {serverError}
          </div>
        </div>
      )}

      {/* Geofence Name */}
      <div className="form-group">
        <label htmlFor="geofence-name" className="form-label required">
          Geofence Name
        </label>
        <input
          id="geofence-name"
          type="text"
          className={`form-input ${errors.name ? 'input-error' : ''}`}
          placeholder="e.g. Central Warehouse Perimeter"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={100}
          required
        />
        {errors.name && <span className="field-error-msg">{errors.name}</span>}
      </div>

      {/* Enabled Toggle */}
      <div className="form-group">
        <label className="checkbox-toggle-label geofence-enabled-toggle">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          <span className="geofence-checkbox-text">
            <strong>Enable Geofence</strong> — Actively evaluate containment on entity coordinate updates
          </span>
        </label>
        {enabled && !isEditing && activeCount >= 100 && (
          <span className="field-warning">
            Maximum active geofence limit is 100. Saving may return 409 GEOFENCE_LIMIT.
          </span>
        )}
      </div>

      {/* Geometry Type Selection */}
      <div className="form-group">
        <label className="form-label">Boundary Geometry Type</label>
        <div className="geom-type-toggle-group" role="radiogroup" aria-label="Geometry Type">
          <button
            type="button"
            className={`geom-toggle-btn ${geomType === 'Circle' ? 'active' : ''}`}
            onClick={() => setGeomType('Circle')}
            role="radio"
            aria-checked={geomType === 'Circle'}
          >
            <strong>Circle</strong>
            <span className="geom-type-desc">Center point and radius in meters</span>
          </button>
          <button
            type="button"
            className={`geom-toggle-btn ${geomType === 'Polygon' ? 'active' : ''}`}
            onClick={() => setGeomType('Polygon')}
            role="radio"
            aria-checked={geomType === 'Polygon'}
          >
            <strong>Polygon</strong>
            <span className="geom-type-desc">Closed exterior ring of coordinates</span>
          </button>
        </div>
      </div>

      {/* Circle Parameters */}
      {geomType === 'Circle' && (
        <div className="circle-fields-container">
          <div className="coord-inputs-row">
            <div className="form-group">
              <label htmlFor="circle-lat" className="form-label required">
                Center Latitude (&deg;)
              </label>
              <input
                id="circle-lat"
                type="number"
                step="any"
                className={`form-input ${errors.circleLat ? 'input-error' : ''}`}
                value={circleLat}
                onChange={(e) => setCircleLat(e.target.value)}
                required
              />
              {errors.circleLat && <span className="field-error-msg">{errors.circleLat}</span>}
            </div>

            <div className="form-group">
              <label htmlFor="circle-lon" className="form-label required">
                Center Longitude (&deg;)
              </label>
              <input
                id="circle-lon"
                type="number"
                step="any"
                className={`form-input ${errors.circleLon ? 'input-error' : ''}`}
                value={circleLon}
                onChange={(e) => setCircleLon(e.target.value)}
                required
              />
              {errors.circleLon && <span className="field-error-msg">{errors.circleLon}</span>}
            </div>
          </div>

          <div className="form-group">
            <div className="geofence-radius-label-row">
              <label htmlFor="circle-radius" className="form-label required">
                Radius (meters)
              </label>
              {mapCenter && (
                <button
                  type="button"
                  className="btn btn-ghost btn-sm btn-use-center"
                  onClick={handleUseMapCenter}
                  title="Fill coordinates from current map viewport center"
                >
                  <CrosshairIcon size={14} />
                  <span>Use Map Center</span>
                </button>
              )}
            </div>
            <input
              id="circle-radius"
              type="number"
              step="any"
              min="0.1"
              max="10000000"
              className={`form-input ${errors.circleRadius ? 'input-error' : ''}`}
              value={circleRadius}
              onChange={(e) => setCircleRadius(e.target.value)}
              required
            />
            {errors.circleRadius && <span className="field-error-msg">{errors.circleRadius}</span>}
          </div>
        </div>
      )}

      {/* Polygon Parameters */}
      {geomType === 'Polygon' && (
        <div className="polygon-fields-container">
          <div className="form-group">
            <label htmlFor="polygon-coords" className="form-label required">
              Polygon Coordinates (GeoJSON format: <code>[[ [lon, lat], ... ]]</code>)
            </label>
            <textarea
              id="polygon-coords"
              rows={6}
              className={`form-textarea geofence-polygon-input ${errors.polygon ? 'input-error' : ''}`}
              value={polyCoordsJson}
              onChange={(e) => setPolyCoordsJson(e.target.value)}
              required
            />
            {errors.polygon && (
              <span className="field-error-msg geofence-polygon-error">{errors.polygon}</span>
            )}
            <span className="geofence-field-hint">
              Exterior ring must be closed (first coordinate equals last) with at least 3 distinct
              vertices. Maximum 256 vertices.
            </span>
          </div>
        </div>
      )}

      {/* Action Buttons */}
      <div className="form-actions-row">
        <button
          type="button"
          className="btn btn-secondary"
          onClick={onCancel}
          disabled={isSubmitting}
        >
          Cancel
        </button>
        <button type="submit" className="btn btn-primary" disabled={isSubmitting}>
          {isSubmitting ? 'Saving Geofence...' : isEditing ? 'Save Changes' : 'Create Geofence'}
        </button>
      </div>
    </form>
  );
}
