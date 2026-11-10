import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { Route, Routes } from 'react-router';
import { describe, expect, it } from 'vitest';
import { tokenStorage } from '../../lib/token-storage';
import { LoginPage } from '../../pages/LoginPage';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/server';
import { AuthProvider } from './AuthProvider';
import { ProtectedRoute } from './ProtectedRoute';

const user = { id: 'u1', email: 'a@x.com', name: 'Alice', created_at: '2026-01-01T00:00:00Z' };

function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedRoute />}>
          <Route index element={<p>secret todos</p>} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}

describe('authentication flow', () => {
  it('redirects to /login, logs in, and lands on the protected page', async () => {
    let authHeader: string | null = null;
    server.use(
      http.post('/api/v1/auth/login', () =>
        HttpResponse.json({
          access_token: 'tok-123',
          token_type: 'Bearer',
          expires_at: '2099-01-01T00:00:00Z',
          user,
        }),
      ),
      http.get('/api/v1/auth/me', ({ request }) => {
        authHeader = request.headers.get('Authorization');
        return HttpResponse.json(user);
      }),
    );
    const u = userEvent.setup();
    renderWithProviders(<App />);

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
    await u.type(screen.getByLabelText('Email'), 'a@x.com');
    await u.type(screen.getByLabelText('Password'), 'password1');
    await u.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByText('secret todos')).toBeInTheDocument();
    expect(tokenStorage.get()).toBe('tok-123');
    expect(authHeader).toBeNull(); // /me was not needed: login response already had the user
  });

  it('shows the server error for wrong credentials', async () => {
    server.use(
      http.post('/api/v1/auth/login', () =>
        HttpResponse.json(
          { error: { code: 'invalid_credentials', message: 'invalid email or password' } },
          { status: 401 },
        ),
      ),
    );
    const u = userEvent.setup();
    renderWithProviders(<App />, { route: '/login' });
    await u.type(await screen.findByLabelText('Email'), 'a@x.com');
    await u.type(screen.getByLabelText('Password'), 'wrong');
    await u.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('invalid email or password');
  });

  it('restores a stored session via /me, and logs out on an invalid token', async () => {
    tokenStorage.set('stored-token');
    server.use(http.get('/api/v1/auth/me', () => HttpResponse.json(user)));
    const first = renderWithProviders(<App />);
    expect(await screen.findByText('secret todos')).toBeInTheDocument();
    first.unmount();

    tokenStorage.set('expired-token');
    server.use(
      http.get('/api/v1/auth/me', () =>
        HttpResponse.json(
          { error: { code: 'unauthorized', message: 'invalid or expired token' } },
          { status: 401 },
        ),
      ),
    );
    renderWithProviders(<App />);
    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
    expect(tokenStorage.get()).toBeNull();
  });
});
