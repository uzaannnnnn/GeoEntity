import React, { useState } from 'react';
import { GeofenceDefinition, GeofenceGeometry, GeofenceInput } from '../../types/geofence';
import { ApiError } from '../../api/client';
import { AlertIcon, CrosshairIcon } from '../Icons';

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
    const errs: { [key: string]: string } = {};

    if (!name.trim()) {
      errs.name = 'Geofence name is required (1 to 100 characters).';
    } else if (name.trim().length > 100) {
      errs.name = 'Geofence name cannot exceed 100 characters.';
    }

    if (geomType === 'Circle') {
      const lat = parseFloat(circleLat);
      const lon = parseFloat(circleLon);
      const rad = parseFloat(circleRadius);

      if (isNaN(lat) || lat < -90 || lat > 90) {
        errs.circleLat = 'Latitude must be a valid number between -90 and 90.';
      }
      if (isNaN(lon) || lon < -180 || lon > 180) {
        errs.circleLon = 'Longitude must be a valid number between -180 and 180.';
      }
      if (isNaN(rad) || rad <= 0 || rad > 10000000) {
        errs.circleRadius = 'Radius must be a positive number up to 10,000,000 meters.';
      }

      if (Object.keys(errs).length > 0) {
        setErrors(errs);
        return null;
      }

      setErrors({});
      return {
        type: 'Circle',
        center: [lon, lat], // [longitude, latitude]
        radius_m: rad,
      };
    } else {
      // Polygon validation
      try {
        const parsed = JSON.parse(polyCoordsJson);
        if (!Array.isArray(parsed) || parsed.length === 0 || !Array.isArray(parsed[0])) {
          errs.polygon = 'Coordinates must be an array containing an exterior ring of [lon, lat] points.';
        } else {
          const ring = parsed[0];
          if (ring.length < 4) {
            errs.polygon = 'Exterior ring must have at least 4 positions (minimum 3 distinct vertices plus closed end).';
          } else {
            const first = ring[0];
            const last = ring[ring.length - 1];
            if (first[0] !== last[0] || first[1] !== last[1]) {
              errs.polygon = 'Exterior ring must be closed (first and last positions must be identical).';
            }
          }
        }
        if (Object.keys(errs).length > 0) {
          setErrors(errs);
          return null;
        }

        setErrors({});
        return {
          type: 'Polygon',
          coordinates: parsed,
        };
      } catch (e) {
        errs.polygon = 'Invalid JSON syntax for polygon coordinates.';
        setErrors(errs);
        return null;
      }
    }
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
          className={`form-control ${errors.name ? 'is-invalid' : ''}`}
          placeholder="e.g. Central Warehouse Perimeter"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={100}
          required
        />
        {errors.name && <span className="field-error">{errors.name}</span>}
      </div>

      {/* Enabled Toggle */}
      <div className="form-group">
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
          />
          <span className="checkbox-text">
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
            <span className="option-desc">Center point and radius in meters</span>
          </button>
          <button
            type="button"
            className={`geom-toggle-btn ${geomType === 'Polygon' ? 'active' : ''}`}
            onClick={() => setGeomType('Polygon')}
            role="radio"
            aria-checked={geomType === 'Polygon'}
          >
            <strong>Polygon</strong>
            <span className="option-desc">Closed exterior ring of coordinates</span>
          </button>
        </div>
      </div>

      {/* Circle Parameters */}
      {geomType === 'Circle' && (
        <div className="circle-fields-container">
          <div className="fields-grid-2">
            <div className="form-group">
              <label htmlFor="circle-lat" className="form-label required">
                Center Latitude (&deg;)
              </label>
              <input
                id="circle-lat"
                type="number"
                step="any"
                className={`form-control ${errors.circleLat ? 'is-invalid' : ''}`}
                value={circleLat}
                onChange={(e) => setCircleLat(e.target.value)}
                required
              />
              {errors.circleLat && <span className="field-error">{errors.circleLat}</span>}
            </div>

            <div className="form-group">
              <label htmlFor="circle-lon" className="form-label required">
                Center Longitude (&deg;)
              </label>
              <input
                id="circle-lon"
                type="number"
                step="any"
                className={`form-control ${errors.circleLon ? 'is-invalid' : ''}`}
                value={circleLon}
                onChange={(e) => setCircleLon(e.target.value)}
                required
              />
              {errors.circleLon && <span className="field-error">{errors.circleLon}</span>}
            </div>
          </div>

          <div className="form-group">
            <div className="radius-label-row">
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
              className={`form-control ${errors.circleRadius ? 'is-invalid' : ''}`}
              value={circleRadius}
              onChange={(e) => setCircleRadius(e.target.value)}
              required
            />
            {errors.circleRadius && <span className="field-error">{errors.circleRadius}</span>}
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
              className={`form-control font-mono ${errors.polygon ? 'is-invalid' : ''}`}
              value={polyCoordsJson}
              onChange={(e) => setPolyCoordsJson(e.target.value)}
              required
            />
            <span className="field-hint">
              Exterior ring must be closed (first coordinate equals last) with at least 3 distinct
              vertices. Maximum 256 vertices.
            </span>
            {errors.polygon && <span className="field-error">{errors.polygon}</span>}
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
