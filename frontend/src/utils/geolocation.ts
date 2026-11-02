export interface UserLocation {
  latitude: number;
  longitude: number;
  accuracy: number;
  timestamp: number;
}

export type GeolocationErrorCode =
  | 'PERMISSION_DENIED'
  | 'POSITION_UNAVAILABLE'
  | 'TIMEOUT'
  | 'UNSUPPORTED'
  | 'NOT_SECURE'
  | 'UNKNOWN';

export interface GeolocationErrorDetails {
  code: GeolocationErrorCode;
  message: string;
}

export const DEFAULT_GEOLOCATION_OPTIONS: PositionOptions = {
  enableHighAccuracy: false,
  timeout: 10000,
  maximumAge: 30000,
};

/**
 * Maps standard browser GeolocationPositionError codes to clear, accessible user messages.
 */
export function mapGeolocationError(error: GeolocationPositionError | Error | unknown): GeolocationErrorDetails {
  if (typeof error === 'object' && error !== null && 'code' in error) {
    const posErr = error as GeolocationPositionError;
    switch (posErr.code) {
      case 1: // GeolocationPositionError.PERMISSION_DENIED
        return {
          code: 'PERMISSION_DENIED',
          message: 'Location access was denied. Allow location permissions in your browser to use Locate Me.',
        };
      case 2: // GeolocationPositionError.POSITION_UNAVAILABLE
        return {
          code: 'POSITION_UNAVAILABLE',
          message: 'Location information is currently unavailable. Please verify your device settings.',
        };
      case 3: // GeolocationPositionError.TIMEOUT
        return {
          code: 'TIMEOUT',
          message: 'Location request timed out. Please try again.',
        };
    }
  }

  if (error instanceof Error && error.message) {
    return {
      code: 'UNKNOWN',
      message: error.message,
    };
  }

  return {
    code: 'UNKNOWN',
    message: 'An unexpected error occurred while requesting your location.',
  };
}

/**
 * Performs a one-shot browser geolocation request.
 *
 * Privacy Guarantees:
 * - Only runs after explicit user action.
 * - Uses getCurrentPosition (never watchPosition).
 * - Result is held in MapView memory only.
 * - Never persisted in localStorage or sessionStorage.
 * - Never sent to backend API or entity storage.
 */
export function getCurrentUserLocation(
  options: PositionOptions = DEFAULT_GEOLOCATION_OPTIONS
): Promise<UserLocation> {
  return new Promise((resolve, reject) => {
    if (typeof window === 'undefined' || typeof navigator === 'undefined' || !navigator.geolocation) {
      reject({
        code: 'UNSUPPORTED',
        message: 'Geolocation is not supported by your browser.',
      } as GeolocationErrorDetails);
      return;
    }

    // Modern browsers require secure context (HTTPS or localhost)
    if (window.isSecureContext === false) {
      reject({
        code: 'NOT_SECURE',
        message: 'Geolocation requires a secure connection (HTTPS or localhost).',
      } as GeolocationErrorDetails);
      return;
    }

    navigator.geolocation.getCurrentPosition(
      (position) => {
        resolve({
          latitude: position.coords.latitude,
          longitude: position.coords.longitude,
          accuracy: position.coords.accuracy,
          timestamp: position.timestamp,
        });
      },
      (error) => {
        reject(mapGeolocationError(error));
      },
      options
    );
  });
}
