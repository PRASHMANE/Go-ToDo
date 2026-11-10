import axios from 'axios';
import { tokenStorage } from './token-storage';

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? '/api/v1',
  headers: { 'Content-Type': 'application/json' },
  timeout: 10_000,
});

/** Shape of every error our Go services return. */
export interface ApiErrorBody {
  error: { code: string; message: string };
}

/** Turn any thrown value into a human-readable message. */
export function getErrorMessage(err: unknown): string {
  if (axios.isAxiosError<ApiErrorBody>(err)) {
    return err.response?.data?.error?.message ?? err.message;
  }
  if (err instanceof Error) return err.message;
  return 'Something went wrong';
}

// Attach the JWT to every request.
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
