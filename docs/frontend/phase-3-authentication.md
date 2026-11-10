# Frontend Phase 3 — Authentication & Protected Routes

**Goal:** register, log in, log out; attach the JWT to every request; redirect
to `/login` when not authenticated. All traffic now goes through the **API gateway**.
**Branch:** `phase/f3-auth` · **Tag:** `f3`

Backend needed: auth-service, todo-service and api-gateway running (Backend B5).

## Concepts

- **Routing:** `react-router` maps URLs to components. `<Outlet />` is where
  nested child routes render. That's how we wrap pages with a layout or a guard.
- **Context:** shares a value (the current user, `login()`, `logout()`) with any
  component in the tree, without passing props through every level.
- **Token storage trade-off:**
  - `localStorage`: simple, survives refresh, but readable by any JavaScript on
    the page (an XSS bug could steal it).
  - `httpOnly` cookie: JS can't read it (safer), but needs CSRF protection and
    backend cookie handling.
  - We use `localStorage` to keep learning focused, and keep the TTL short.
    Moving to httpOnly cookies + refresh tokens is a great follow-up.
- **Interceptors:** axios runs a function before every request (add
  `Authorization: Bearer …`) and after every response (on `401`, log out).

## Files

```
src/lib/token-storage.ts
src/lib/api-client.ts                  # + interceptors
src/features/auth/types.ts
src/features/auth/api.ts
src/features/auth/AuthContext.ts
src/features/auth/AuthProvider.tsx
src/features/auth/useAuth.ts
src/features/auth/ProtectedRoute.tsx
src/features/auth/components/{LoginForm,RegisterForm}.tsx
src/components/layout/AppLayout.tsx
src/pages/{LoginPage,RegisterPage,NotFoundPage}.tsx
src/app/providers.tsx                  # + Router + AuthProvider
src/App.tsx                            # routes
vite.config.ts                         # proxy → gateway :8080
```

## Step 1 — Point the dev proxy at the gateway

`vite.config.ts`:

```ts
proxy: {
  '/api': { target: 'http://localhost:8080', changeOrigin: true },
},
```

## Step 2 — `src/lib/token-storage.ts`

```ts
const KEY = 'go-todo.access_token';

// try/catch: storage can throw in private mode or when disabled.
export const tokenStorage = {
  get(): string | null {
    try {
      return localStorage.getItem(KEY);
    } catch {
      return null;
    }
  },
  set(token: string) {
    try {
      localStorage.setItem(KEY, token);
    } catch {
      /* ignore */
    }
  },
  clear() {
    try {
      localStorage.removeItem(KEY);
    } catch {
      /* ignore */
    }
  },
};
```

## Step 3 — Interceptors in `src/lib/api-client.ts` (append)

```ts
import { tokenStorage } from './token-storage';

apiClient.interceptors.request.use((config) => {
  const token = tokenStorage.get();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

let onUnauthorized: (() => void) | null = null;

/** AuthProvider registers its logout() here. */
export function setUnauthorizedHandler(handler: (() => void) | null) {
  onUnauthorized = handler;
}

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      onUnauthorized?.(); // expired/invalid token → log out everywhere
    }
    return Promise.reject(error);
  },
);
```

## Step 4 — Auth types & API

`src/features/auth/types.ts`:

```ts
export interface User {
  id: string;
  email: string;
  name: string;
  created_at: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface RegisterInput extends LoginInput {
  name: string;
}

export interface LoginResponse {
  access_token: string;
  token_type: 'Bearer';
  expires_at: string;
  user: User;
}
```

`src/features/auth/api.ts`:

```ts
import { apiClient } from '../../lib/api-client';
import type { LoginInput, LoginResponse, RegisterInput, User } from './types';

export const authApi = {
  async register(input: RegisterInput): Promise<User> {
    const { data } = await apiClient.post<User>('/auth/register', input);
    return data;
  },
  async login(input: LoginInput): Promise<LoginResponse> {
    const { data } = await apiClient.post<LoginResponse>('/auth/login', input);
    return data;
  },
  async me(): Promise<User> {
    const { data } = await apiClient.get<User>('/auth/me');
    return data;
  },
};
```

## Step 5 — Context, provider, hook (3 small files)

`src/features/auth/AuthContext.ts`:

```ts
import { createContext } from 'react';
import type { RegisterInput, User } from './types';

export interface AuthContextValue {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean; // true while we check a stored token on startup
  login: (email: string, password: string) => Promise<void>;
  register: (input: RegisterInput) => Promise<void>;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);
```

`src/features/auth/AuthProvider.tsx`:

```tsx
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { setUnauthorizedHandler } from '../../lib/api-client';
import { tokenStorage } from '../../lib/token-storage';
import { authApi } from './api';
import { AuthContext, type AuthContextValue } from './AuthContext';
import type { RegisterInput } from './types';

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [token, setToken] = useState<string | null>(() => tokenStorage.get());

  // "Who am I?": only runs when we have a token. Validates stored tokens on refresh.
  const meQuery = useQuery({
    queryKey: ['auth', 'me', token],
    queryFn: authApi.me,
    enabled: token !== null,
    retry: false,
    staleTime: Infinity,
  });

  const logout = useCallback(() => {
    tokenStorage.clear();
    setToken(null);
    queryClient.clear(); // drop ALL cached data, so the next user never sees the old user's todos
  }, [queryClient]);

  useEffect(() => {
    setUnauthorizedHandler(logout);
    return () => setUnauthorizedHandler(null);
  }, [logout]);

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await authApi.login({ email, password });
      tokenStorage.set(res.access_token);
      queryClient.setQueryData(['auth', 'me', res.access_token], res.user); // no extra /me call
      setToken(res.access_token);
    },
    [queryClient],
  );

  const register = useCallback(
    async (input: RegisterInput) => {
      await authApi.register(input);
      await login(input.email, input.password); // log straight in after sign-up
    },
    [login],
  );

  const value = useMemo<AuthContextValue>(
    () => ({
      user: meQuery.data ?? null,
      isAuthenticated: token !== null && meQuery.isSuccess,
      isLoading: token !== null && meQuery.isPending,
      login,
      register,
      logout,
    }),
    [token, meQuery.data, meQuery.isSuccess, meQuery.isPending, login, register, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
```

`src/features/auth/useAuth.ts`:

```ts
import { useContext } from 'react';
import { AuthContext } from './AuthContext';

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside <AuthProvider>');
  return ctx;
}
```

(Three files instead of one keeps the `react-refresh/only-export-components`
lint rule happy: a file that exports a component should only export components.)

## Step 6 — `src/features/auth/ProtectedRoute.tsx`

```tsx
import { Navigate, Outlet, useLocation } from 'react-router';
import { Spinner } from '../../components/ui/Spinner';
import { useAuth } from './useAuth';

export function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) return <Spinner label="Checking your session…" />;
  if (!isAuthenticated) {
    // remember where the user wanted to go
    return <Navigate to="/login" replace state={{ from: location }} />;
  }
  return <Outlet />;
}
```

## Step 7 — Forms

`src/features/auth/components/LoginForm.tsx`:

```tsx
import { useState, type FormEvent } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { Button } from '../../../components/ui/Button';
import { ErrorMessage } from '../../../components/ui/ErrorMessage';
import { Input } from '../../../components/ui/Input';
import { getErrorMessage } from '../../../lib/api-client';
import { useAuth } from '../useAuth';

export function LoginForm() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const from = (location.state as { from?: { pathname: string } } | null)?.from?.pathname ?? '/';

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(email, password);
      navigate(from, { replace: true });
    } catch (err) {
      setError(getErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3">
      {error && <ErrorMessage message={error} />}
      <label className="flex flex-col gap-1 text-sm">
        Email
        <Input type="email" autoComplete="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
      </label>
      <label className="flex flex-col gap-1 text-sm">
        Password
        <Input type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
      </label>
      <Button type="submit" disabled={submitting}>
        {submitting ? 'Signing in…' : 'Sign in'}
      </Button>
    </form>
  );
}
```

`RegisterForm.tsx`: same structure with a `name` field, `minLength={8}` on
password, `autoComplete="new-password"`, and `await register({ name, email, password })`
then `navigate('/', { replace: true })`. **Write it yourself**, using LoginForm as the template.

## Step 8 — Layout & pages

`src/components/layout/AppLayout.tsx`:

```tsx
import { Outlet } from 'react-router';
import { useAuth } from '../../features/auth/useAuth';
import { Button } from '../ui/Button';

export function AppLayout() {
  const { user, logout } = useAuth();
  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-2xl items-center justify-between p-4">
          <span className="text-xl font-bold">Go-ToDo</span>
          <div className="flex items-center gap-3 text-sm">
            <span className="text-slate-600">{user?.name}</span>
            <Button variant="secondary" onClick={logout}>Log out</Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-2xl p-6">
        <Outlet />
      </main>
    </div>
  );
}
```

`src/pages/LoginPage.tsx`:

```tsx
import { Link, Navigate } from 'react-router';
import { LoginForm } from '../features/auth/components/LoginForm';
import { useAuth } from '../features/auth/useAuth';

export function LoginPage() {
  const { isAuthenticated } = useAuth();
  if (isAuthenticated) return <Navigate to="/" replace />;
  return (
    <div className="mx-auto mt-24 max-w-sm rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
      <h1 className="mb-4 text-2xl font-bold">Sign in</h1>
      <LoginForm />
      <p className="mt-4 text-sm text-slate-600">
        No account? <Link to="/register" className="text-indigo-600 hover:underline">Create one</Link>
      </p>
    </div>
  );
}
```

`RegisterPage.tsx`: the mirror image. `NotFoundPage.tsx`: "Page not found" + `<Link to="/">`.

## Step 9 — Providers & routes

`src/app/providers.tsx`: order matters (AuthProvider needs the QueryClient and the Router):

```tsx
import type { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { BrowserRouter } from 'react-router';
import { AuthProvider } from '../features/auth/AuthProvider';
import { queryClient } from '../lib/query-client';

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>{children}</AuthProvider>
      </BrowserRouter>
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  );
}
```

`src/App.tsx`:

```tsx
import { Route, Routes } from 'react-router';
import { AppLayout } from './components/layout/AppLayout';
import { ProtectedRoute } from './features/auth/ProtectedRoute';
import { LoginPage } from './pages/LoginPage';
import { NotFoundPage } from './pages/NotFoundPage';
import { RegisterPage } from './pages/RegisterPage';
import { TodosPage } from './pages/TodosPage';

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      <Route element={<ProtectedRoute />}>      {/* guard */}
        <Route element={<AppLayout />}>         {/* layout */}
          <Route index element={<TodosPage />} />
        </Route>
      </Route>

      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
```

## Verify

- Visit `/` logged out → redirected to `/login`.
- Register → you land on `/` with an empty list and your name in the header.
- Create todos, log out, register a second user → empty list (per-user data from B5).
- Refresh the page while logged in → still logged in (token from storage + `/me`).
- DevTools → Application → Local Storage: edit the token to garbage, refresh → back to `/login`.
- Network tab: every `/api/v1/todos` request has `Authorization: Bearer …`.
- Wrong password → "invalid email or password" from the Go service.

## Exercises

1. Paste the token into jwt.io. Which claims do you see? Why is it fine that they are readable?
2. Make the app log out automatically when the token's `expires_at` passes (hint: `setTimeout` in AuthProvider, store `expires_at`).
3. What would change to use an httpOnly cookie instead of localStorage? List the changes in Go and in React.

## Commit

```bash
git add -A
git commit -m "feat(frontend): login, register and protected routes (F3)"
```

**Next:** [Backend Phase 6 — Docker](../backend/phase-6-docker.md), then
[Frontend Phase 4](phase-4-advanced-todo-features.md).
