import axios from 'axios';

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
