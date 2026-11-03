import { useRef, useState, type FormEvent } from 'react';
import { AuthLoginError, type AuthLoginFailureKind } from '../../api/auth';
import { AlertIcon, CloseIcon, EyeIcon, EyeOffIcon, LogoIcon } from '../Icons';

interface SignInScreenProps {
  onSignIn: (passphrase: string) => Promise<void>;
  initialMessage?: string | null;
}

export function loginFailureMessage(kind: AuthLoginFailureKind): { title: string; message: string } {
  switch (kind) {
    case 'invalid-passphrase':
      return { title: 'Incorrect passphrase', message: 'Please check your passphrase and try again.' };
    case 'rate-limited':
      return { title: 'Too many attempts', message: 'Wait a moment before trying to sign in again.' };
    case 'network':
      return { title: 'Connection problem', message: 'Check your connection and try again.' };
    case 'service':
      return { title: 'Sign in unavailable', message: 'We could not sign you in right now. Please try again.' };
  }
}

export function SignInScreen({ onSignIn, initialMessage = null }: SignInScreenProps) {
  const [passphrase, setPassphrase] = useState('');
  const [isPassphraseVisible, setIsPassphraseVisible] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submitGuard = useRef(false);
  const [feedback, setFeedback] = useState<{ title: string; message: string } | null>(null);
  const [notice, setNotice] = useState(initialMessage);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitGuard.current || isSubmitting) return;

    submitGuard.current = true;
    setIsSubmitting(true);
    setFeedback(null);
    setNotice(null);
    try {
      await onSignIn(passphrase);
      setPassphrase('');
      setIsPassphraseVisible(false);
    } catch (error) {
      const kind = error instanceof AuthLoginError ? error.kind : 'service';
      setFeedback(loginFailureMessage(kind));
    } finally {
      submitGuard.current = false;
      setIsSubmitting(false);
    }
  };

  return (
    <main className="auth-screen">
      <form className="auth-card" onSubmit={handleSubmit}>
        <div className="auth-brand">
          <div className="auth-brand-mark"><LogoIcon size={24} /></div>
          <span>GEOENTITY</span>
        </div>

        <div className="auth-heading">
          <h1>Welcome back</h1>
          <p>Sign in to manage your geospatial operations.</p>
        </div>

        {notice && (
          <div className="auth-notice" role="status">
            <span>{notice}</span>
            <button type="button" onClick={() => setNotice(null)} aria-label="Dismiss message">
              <CloseIcon size={16} />
            </button>
          </div>
        )}

        {feedback && (
          <div className="auth-feedback" role="alert" aria-live="assertive">
            <AlertIcon size={20} aria-hidden="true" />
            <div className="auth-feedback-copy">
              <strong>{feedback.title}</strong>
              <span>{feedback.message}</span>
            </div>
            <button type="button" onClick={() => setFeedback(null)} aria-label="Dismiss sign-in error">
              <CloseIcon size={16} />
            </button>
          </div>
        )}

        <div className="auth-field-group">
          <label htmlFor="admin-passphrase">Administrator passphrase</label>
          <div className="auth-password-control">
            <input
              id="admin-passphrase"
              type={isPassphraseVisible ? 'text' : 'password'}
              autoComplete="current-password"
              required
              value={passphrase}
              onChange={(event) => setPassphrase(event.target.value)}
              aria-describedby="auth-passphrase-help"
            />
            <button
              type="button"
              className="auth-visibility-toggle"
              onClick={() => setIsPassphraseVisible((visible) => !visible)}
              aria-label={isPassphraseVisible ? 'Hide passphrase' : 'Show passphrase'}
              aria-pressed={isPassphraseVisible}
            >
              {isPassphraseVisible ? <EyeOffIcon /> : <EyeIcon />}
            </button>
          </div>
          <span id="auth-passphrase-help" className="auth-help-text">
            Use the administrator passphrase configured for this installation.
          </span>
        </div>

        <button
          className="auth-submit"
          type="submit"
          disabled={isSubmitting || passphrase.length === 0}
        >
          {isSubmitting ? <><span className="auth-submit-spinner" aria-hidden="true" /> Signing in…</> : 'Sign in'}
        </button>

        <p className="auth-security-note">Protected administrator access</p>
      </form>
    </main>
  );
}
