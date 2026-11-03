import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { authApi } from './auth';
import { getCSRFToken, setCSRFToken } from './client';

describe('authApi', () => {
  beforeEach(() => { vi.stubGlobal('fetch', vi.fn()); setCSRFToken(null); });
  afterEach(() => { vi.unstubAllGlobals(); vi.resetAllMocks(); });

  it('keeps bootstrap CSRF only in module memory and uses the same-origin cookie', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: true, csrfToken: 'csrf-session-token' }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    await expect(authApi.session()).resolves.toEqual({ authenticated: true, csrfToken: 'csrf-session-token' });
    expect(getCSRFToken()).toBe('csrf-session-token');
    expect(fetch).toHaveBeenCalledWith('/api/auth/session', expect.objectContaining({ credentials: 'same-origin' }));
  });

  it('posts the passphrase without logging or putting it in the URL and stores the returned CSRF token', async () => {
    const passphrase = 'long private test passphrase';
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: true, csrfToken: 'csrf-after-login' }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    await authApi.login(passphrase);
    expect(fetch).toHaveBeenCalledWith('/api/auth/login', expect.objectContaining({ method: 'POST', credentials: 'same-origin', body: JSON.stringify({ password: passphrase }) }));
    expect(getCSRFToken()).toBe('csrf-after-login');
  });

  it('sends the session-bound CSRF token for logout and clears it', async () => {
    setCSRFToken('csrf-after-login');
    vi.mocked(fetch).mockResolvedValueOnce(new Response(null, { status: 204 }));
    await authApi.logout();
    expect(fetch).toHaveBeenCalledWith('/api/auth/logout', expect.objectContaining({ method: 'POST', headers: expect.objectContaining({ 'X-CSRF-Token': 'csrf-after-login' }) }));
    expect(getCSRFToken()).toBeNull();
  });
});
