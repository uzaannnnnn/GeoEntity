import { describe, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { Header } from './Header';

describe('Header session action', () => {
  it('renders the dedicated uppercase sign-out button with an icon', () => {
    const markup = renderToStaticMarkup(
      <Header
        totalCount={0}
        activeCount={0}
        maintenanceCount={0}
        inactiveCount={0}
        isHealthOk={true}
        isLoading={false}
        onAddClick={vi.fn()}
        onRefreshClick={vi.fn()}
        onSignOut={vi.fn()}
        isCreateActive={false}
      />
    );

    expect(markup).toContain('class="auth-signout"');
    expect(markup).toContain('SIGN OUT');
    expect(markup).toContain('aria-hidden="true"');
  });
});
