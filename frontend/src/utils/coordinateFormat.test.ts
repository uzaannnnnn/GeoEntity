import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  formatCoordinateSixDecimals,
  formatCenterCopyText,
  copyTextToClipboard,
} from './coordinateFormat';

describe('coordinateFormat', () => {
  describe('formatCoordinateSixDecimals', () => {
    it('formats numbers with exactly six decimal places', () => {
      expect(formatCoordinateSixDecimals(-6.2088)).toBe('-6.208800');
      expect(formatCoordinateSixDecimals(106.8456)).toBe('106.845600');
      expect(formatCoordinateSixDecimals(0)).toBe('0.000000');
      expect(formatCoordinateSixDecimals(12.3456789)).toBe('12.345679');
    });

    it('handles non-finite values safely', () => {
      expect(formatCoordinateSixDecimals(NaN)).toBe('0.000000');
      expect(formatCoordinateSixDecimals(Infinity)).toBe('0.000000');
      expect(formatCoordinateSixDecimals(-Infinity)).toBe('0.000000');
      expect(formatCoordinateSixDecimals(null as unknown as number)).toBe('0.000000');
    });
  });

  describe('formatCenterCopyText', () => {
    it('formats latitude then longitude separated by comma and space', () => {
      const formatted = formatCenterCopyText(-6.2088, 106.8456);
      expect(formatted).toBe('-6.208800, 106.845600');
    });

    it('never produces GeoJSON longitude-first ordering', () => {
      const lat = 10.123456;
      const lng = 100.654321;
      const formatted = formatCenterCopyText(lat, lng);
      expect(formatted.startsWith('10.123456')).toBe(true);
      expect(formatted.endsWith('100.654321')).toBe(true);
    });
  });

  describe('copyTextToClipboard', () => {
    beforeEach(() => {
      vi.restoreAllMocks();
    });

    it('uses navigator.clipboard.writeText when available', async () => {
      const writeTextMock = vi.fn().mockResolvedValue(undefined);
      vi.stubGlobal('navigator', {
        clipboard: {
          writeText: writeTextMock,
        },
      });

      const success = await copyTextToClipboard('-6.208800, 106.845600');
      expect(success).toBe(true);
      expect(writeTextMock).toHaveBeenCalledWith('-6.208800, 106.845600');
    });

    it('falls back to document.execCommand when clipboard API fails', async () => {
      const writeTextMock = vi.fn().mockRejectedValue(new Error('Permission denied'));
      const execCommandMock = vi.fn().mockReturnValue(true);

      vi.stubGlobal('navigator', {
        clipboard: {
          writeText: writeTextMock,
        },
      });

      const appendChildMock = vi.fn();
      const removeChildMock = vi.fn();
      vi.stubGlobal('document', {
        createElement: vi.fn().mockReturnValue({
          style: {},
          setAttribute: vi.fn(),
          select: vi.fn(),
        }),
        body: {
          appendChild: appendChildMock,
          removeChild: removeChildMock,
        },
        execCommand: execCommandMock,
      });

      const success = await copyTextToClipboard('-6.208800, 106.845600');
      expect(success).toBe(true);
      expect(execCommandMock).toHaveBeenCalledWith('copy');
    });

    it('returns false when clipboard and fallback both fail', async () => {
      vi.stubGlobal('navigator', {});
      vi.stubGlobal('document', undefined);

      const success = await copyTextToClipboard('-6.208800, 106.845600');
      expect(success).toBe(false);
    });
  });
});
