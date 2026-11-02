import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  mapGeolocationError,
  getCurrentUserLocation,
} from './geolocation';

describe('geolocation', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('mapGeolocationError', () => {
    it('maps PERMISSION_DENIED (code 1) to user-friendly message', () => {
      const result = mapGeolocationError({ code: 1, message: 'User denied Geolocation' });
      expect(result.code).toBe('PERMISSION_DENIED');
      expect(result.message).toContain('Location access was denied');
    });

    it('maps POSITION_UNAVAILABLE (code 2) to user-friendly message', () => {
      const result = mapGeolocationError({ code: 2, message: 'Position unavailable' });
      expect(result.code).toBe('POSITION_UNAVAILABLE');
      expect(result.message).toContain('unavailable');
    });

    it('maps TIMEOUT (code 3) to user-friendly message', () => {
      const result = mapGeolocationError({ code: 3, message: 'Timeout' });
      expect(result.code).toBe('TIMEOUT');
      expect(result.message).toContain('timed out');
    });

    it('handles unexpected error objects', () => {
      const result = mapGeolocationError(new Error('Device error'));
      expect(result.code).toBe('UNKNOWN');
      expect(result.message).toBe('Device error');
    });
  });

  describe('getCurrentUserLocation', () => {
    it('rejects with UNSUPPORTED when navigator.geolocation is absent', async () => {
      vi.stubGlobal('navigator', {});
      vi.stubGlobal('window', { isSecureContext: true });

      await expect(getCurrentUserLocation()).rejects.toEqual({
        code: 'UNSUPPORTED',
        message: 'Geolocation is not supported by your browser.',
      });
    });

    it('rejects with NOT_SECURE when window.isSecureContext is false', async () => {
      vi.stubGlobal('navigator', { geolocation: { getCurrentPosition: vi.fn() } });
      vi.stubGlobal('window', { isSecureContext: false });

      await expect(getCurrentUserLocation()).rejects.toEqual({
        code: 'NOT_SECURE',
        message: 'Geolocation requires a secure connection (HTTPS or localhost).',
      });
    });

    it('resolves with user coordinates and accuracy on success', async () => {
      const mockPosition = {
        coords: {
          latitude: -6.2088,
          longitude: 106.8456,
          accuracy: 15,
          altitude: null,
          altitudeAccuracy: null,
          heading: null,
          speed: null,
        },
        timestamp: 1700000000000,
      } as unknown as GeolocationPosition;

      const getCurrentPositionMock = vi.fn().mockImplementation((success) => {
        success(mockPosition);
      });

      vi.stubGlobal('navigator', {
        geolocation: {
          getCurrentPosition: getCurrentPositionMock,
        },
      });
      vi.stubGlobal('window', { isSecureContext: true });

      const location = await getCurrentUserLocation();
      expect(location.latitude).toBe(-6.2088);
      expect(location.longitude).toBe(106.8456);
      expect(location.accuracy).toBe(15);
      expect(location.timestamp).toBe(1700000000000);
      expect(getCurrentPositionMock).toHaveBeenCalledTimes(1);
    });

    it('rejects with mapped error when getCurrentPosition fails', async () => {
      const mockError = { code: 1, message: 'Denied' };
      const getCurrentPositionMock = vi.fn().mockImplementation((_success, error) => {
        error(mockError);
      });

      vi.stubGlobal('navigator', {
        geolocation: {
          getCurrentPosition: getCurrentPositionMock,
        },
      });
      vi.stubGlobal('window', { isSecureContext: true });

      await expect(getCurrentUserLocation()).rejects.toMatchObject({
        code: 'PERMISSION_DENIED',
      });
    });
  });
});
