# Frontend Phase 2 — Connect to the API with TanStack Query

**Goal:** replace the fake `useState` data with the real todo-service (Backend
Phase 3, Postgres). Learn **server state** vs **client state**.
**Branch:** `phase/f2-connect-api` · **Tag:** `f2`

Backend needed: `make db-up migrate-todo run-todo` (todo-service on :8082).

## Concepts

- **Client state:** UI-only things such as "which filter tab is selected" or "the
  text in the input". Keep it in `useState`.
- **Server state:** data that lives on the server (todos). It can be stale,
  needs loading/error handling, caching, refetching. **Don't** copy it into
  `useState`. Let **TanStack Query** own it.
- **Query** (`useQuery`): read data, identified by a **query key** like
  `['todos', 'list', { status: 'active' }]`. Same key = same cache entry.
- **Mutation** (`useMutation`): change data (POST/PATCH/DELETE). After success
  we **invalidate** `['todos']` so every todo query refetches.
- **Why a dev proxy?** The browser calls `/api/v1/todos` on `localhost:5173`;
  Vite forwards it to `:8082`. Same origin → no CORS needed in development.

## Files

```
src/lib/api-client.ts
src/lib/query-client.ts
src/app/providers.tsx
src/features/todos/api.ts
src/features/todos/hooks.ts
src/pages/TodosPage.tsx    # rewritten
src/main.tsx               # wrap in providers
```

## Step 1 — `src/lib/api-client.ts`

```ts
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
```

## Step 2 — `src/lib/query-client.ts`

```ts
import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,           // data is "fresh" for 30s → no refetch on every mount
      retry: 1,                    // retry failed requests once
      refetchOnWindowFocus: true,  // refresh when you come back to the tab
    },
  },
});
```

## Step 3 — `src/app/providers.tsx`

```tsx
import type { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { queryClient } from '../lib/query-client';

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      {children}
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  );
}
```

`src/main.tsx`: wrap `<App />`:

```tsx
import { AppProviders } from './app/providers';
// ...
  <StrictMode>
    <AppProviders>
      <App />
    </AppProviders>
  </StrictMode>,
```

## Step 4 — `src/features/todos/api.ts` (pure HTTP, no React)

```ts
import { apiClient } from '../../lib/api-client';
import type { CreateTodoInput, ListTodosParams, ListTodosResponse, Todo, UpdateTodoInput } from './types';

export const todosApi = {
  async list(params: ListTodosParams): Promise<ListTodosResponse> {
    const { data } = await apiClient.get<ListTodosResponse>('/todos', { params });
    return data;
  },
  async create(input: CreateTodoInput): Promise<Todo> {
    const { data } = await apiClient.post<Todo>('/todos', input);
    return data;
  },
  async update(id: string, input: UpdateTodoInput): Promise<Todo> {
    const { data } = await apiClient.patch<Todo>(`/todos/${id}`, input);
    return data;
  },
  async remove(id: string): Promise<void> {
    await apiClient.delete(`/todos/${id}`);
  },
};
```

## Step 5 — `src/features/todos/hooks.ts` (React + cache)

```ts
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { todosApi } from './api';
import type { ListTodosParams, UpdateTodoInput } from './types';

/** Central place for query keys. Never type them by hand elsewhere. */
export const todoKeys = {
  all: ['todos'] as const,
  lists: () => [...todoKeys.all, 'list'] as const,
  list: (params: ListTodosParams) => [...todoKeys.lists(), params] as const,
};

export function useTodos(params: ListTodosParams) {
  return useQuery({
    queryKey: todoKeys.list(params),
    queryFn: () => todosApi.list(params),
    placeholderData: keepPreviousData, // keep showing old list while the new filter loads
  });
}

export function useCreateTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: todosApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: todoKeys.all }),
  });
}

export function useUpdateTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateTodoInput }) => todosApi.update(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: todoKeys.all }),
  });
}

export function useDeleteTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: todosApi.remove,
    onSuccess: () => qc.invalidateQueries({ queryKey: todoKeys.all }),
  });
}
```

## Step 6 — Rewrite `src/pages/TodosPage.tsx`

```tsx
import { useState } from 'react';
import { ErrorMessage } from '../components/ui/ErrorMessage';
import { Spinner } from '../components/ui/Spinner';
import { getErrorMessage } from '../lib/api-client';
import { TodoFilters } from '../features/todos/components/TodoFilters';
import { TodoForm } from '../features/todos/components/TodoForm';
import { TodoList } from '../features/todos/components/TodoList';
import { useCreateTodo, useDeleteTodo, useTodos, useUpdateTodo } from '../features/todos/hooks';
import type { StatusFilter, Todo } from '../features/todos/types';

export function TodosPage() {
  const [status, setStatus] = useState<StatusFilter>('all'); // client state

  const todosQuery = useTodos({ status, page_size: 50 });   // server state
  const createTodo = useCreateTodo();
  const updateTodo = useUpdateTodo();
  const deleteTodo = useDeleteTodo();

  const mutationError = createTodo.error ?? updateTodo.error ?? deleteTodo.error;

  return (
    <section className="flex flex-col gap-4">
      <TodoForm onSubmit={(input) => createTodo.mutate(input)} isSubmitting={createTodo.isPending} />

      {mutationError && <ErrorMessage message={getErrorMessage(mutationError)} />}

      <div className="flex items-center justify-between">
        <TodoFilters status={status} onStatusChange={setStatus} />
        {todosQuery.data && <span className="text-sm text-slate-500">{todosQuery.data.total} todos</span>}
      </div>

      {todosQuery.isPending ? (
        <Spinner />
      ) : todosQuery.isError ? (
        <ErrorMessage message={getErrorMessage(todosQuery.error)} />
      ) : (
        <TodoList
          todos={todosQuery.data.items}
          onToggle={(t: Todo) => updateTodo.mutate({ id: t.id, input: { completed: !t.completed } })}
          onDelete={(t: Todo) => deleteTodo.mutate(t.id)}
        />
      )}
    </section>
  );
}
```

Notice what **disappeared**: `addTodo`, `toggleTodo`, the fake data, the
manual filtering. The **server** filters (`?status=active`), and TanStack Query
caches each filter separately.

## Verify

```bash
# terminal 1 (repo root)
make db-up migrate-todo run-todo
# terminal 2
cd frontend && npm run dev
```

- Add todos → refresh the page → they're still there (Postgres!).
- Check in psql: `select title, completed from todos;`
- DevTools → Network tab: see `GET /api/v1/todos?status=all&page_size=50`.
- React Query Devtools (flower icon, bottom-right): one cache entry per filter.
- Stop todo-service → you see an error message, not a blank page. Start it again → focus the tab → data comes back.
- Add a todo with an empty title by removing `disabled` temporarily → the Go validation message is shown.

## Exercises

1. Switch filters quickly: why doesn't the list flash a spinner? (`keepPreviousData`.)
2. Open the app in two tabs. Add a todo in one, then click into the other. What refreshes it, and why?
3. Disable a todo's checkbox while its update is in flight (`updateTodo.isPending && updateTodo.variables?.id === todo.id`).

## Commit

```bash
git add -A
git commit -m "feat(frontend): connect todo UI to API with TanStack Query (F2)"
```

**Next:** [Backend Phase 4 — Auth](../backend/phase-4-auth-service.md) →
[Backend Phase 5 — Gateway](../backend/phase-5-api-gateway.md) →
[Frontend Phase 3 — Authentication](phase-3-authentication.md)
