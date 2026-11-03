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

  it('classifies an unauthorized response as an incorrect passphrase without exposing response details', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response('private backend detail', { status: 401 }));

    await expect(authApi.login('wrong test passphrase')).rejects.toMatchObject({
      name: 'AuthLoginError',
      kind: 'invalid-passphrase',
    });
  });

  it('classifies network and service failures separately from an incorrect passphrase', async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new TypeError('connection refused'));
    await expect(authApi.login('test passphrase')).rejects.toMatchObject({ kind: 'network' });

    vi.mocked(fetch).mockResolvedValueOnce(new Response('private server detail', { status: 503 }));
    await expect(authApi.login('test passphrase')).rejects.toMatchObject({ kind: 'service' });

    vi.mocked(fetch).mockResolvedValueOnce(new Response('private rate detail', { status: 429 }));
    await expect(authApi.login('test passphrase')).rejects.toMatchObject({ kind: 'rate-limited' });
  });

  it('sends the session-bound CSRF token for logout and clears it', async () => {
    setCSRFToken('csrf-after-login');
    vi.mocked(fetch).mockResolvedValueOnce(new Response(null, { status: 204 }));
    await authApi.logout();
    expect(fetch).toHaveBeenCalledWith('/api/auth/logout', expect.objectContaining({ method: 'POST', headers: expect.objectContaining({ 'X-CSRF-Token': 'csrf-after-login' }) }));
    expect(getCSRFToken()).toBeNull();
  });

  it('returns an unauthenticated session after successful logout', async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ authenticated: false }), { status: 200 }));

    await authApi.logout();
    await expect(authApi.session()).resolves.toEqual({ authenticated: false });
  });
});
