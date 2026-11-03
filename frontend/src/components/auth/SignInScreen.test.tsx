import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { SignInScreen, loginFailureMessage } from './SignInScreen';

describe('SignInScreen', () => {
  it('renders an accessible passphrase field and show/hide control', () => {
    const markup = renderToStaticMarkup(<SignInScreen onSignIn={async () => {}} />);

    expect(markup).toContain('Welcome back');
    expect(markup).toContain('type="password"');
    expect(markup).toContain('autoComplete="current-password"');
    expect(markup).toContain('aria-label="Show passphrase"');
    expect(markup).toContain('disabled=""');
  });

  it('shows a dismissible session notice without exposing credentials', () => {
    const markup = renderToStaticMarkup(
      <SignInScreen onSignIn={async () => {}} initialMessage="Your session expired. Sign in again." />
    );

    expect(markup).toContain('role="status"');
    expect(markup).toContain('Your session expired. Sign in again.');
    expect(markup).toContain('aria-label="Dismiss message"');
  });

  it('maps authentication failures to distinct safe feedback', () => {
    expect(loginFailureMessage('invalid-passphrase').message).toBe(
      'Please check your passphrase and try again.'
    );
    expect(loginFailureMessage('network').title).toBe('Connection problem');
    expect(loginFailureMessage('service').title).toBe('Sign in unavailable');
    expect(loginFailureMessage('rate-limited').title).toBe('Too many attempts');
  });
});
