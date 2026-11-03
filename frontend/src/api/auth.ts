import { API_BASE_URL, getCSRFToken, setCSRFToken } from './client';

export type SessionState = { authenticated: boolean; csrfToken?: string };

export type AuthLoginFailureKind = 'invalid-passphrase' | 'rate-limited' | 'network' | 'service';

export class AuthLoginError extends Error {
  constructor(readonly kind: AuthLoginFailureKind) {
    super(kind);
    this.name = 'AuthLoginError';
  }
}

let sessionFlight: Promise<SessionState> | null = null;
export const authApi = {
  async session(): Promise<SessionState> {
    if (sessionFlight) return sessionFlight;
    sessionFlight = (async () => {
    const response = await fetch(`${API_BASE_URL}/auth/session`, { credentials: 'same-origin', headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error('Unable to check session');
    const state = (await response.json()) as SessionState;
    setCSRFToken(state.csrfToken || null);
    return state;
    })();
    try { return await sessionFlight; } finally { sessionFlight = null; }
  },
  async login(password: string): Promise<void> {
    let response: Response;
    try {
      response = await fetch(`${API_BASE_URL}/auth/login`, {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ password }),
      });
    } catch {
      throw new AuthLoginError('network');
    }
    if (response.status === 401) throw new AuthLoginError('invalid-passphrase');
    if (response.status === 429) throw new AuthLoginError('rate-limited');
    if (!response.ok) throw new AuthLoginError('service');

    let state: SessionState;
    try {
      state = (await response.json()) as SessionState;
    } catch {
      throw new AuthLoginError('service');
    }
    setCSRFToken(state.csrfToken || null);
  },
  async logout(): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/auth/logout`, {
      method: 'POST', credentials: 'same-origin', headers: { Accept: 'application/json', 'X-CSRF-Token': getCSRFToken() || '' },
    });
    if (!response.ok && response.status !== 401) throw new Error('Unable to sign out');
    setCSRFToken(null);
  },
};
