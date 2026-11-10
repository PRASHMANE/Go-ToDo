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
