# Frontend Phase 6 — Testing, Activity Feed & Polish

**Goal:** component and integration tests, an activity feed powered by the
event-driven activity-service (Backend B8), an error boundary, and an
accessibility pass.
**Branch:** `phase/f6-testing-polish` · **Tag:** `f6`

## Concepts

- **Vitest:** a test runner built on Vite (same config, very fast). API like Jest.
- **Testing Library:** test what the **user** sees: find elements by role/label
  (`getByRole('button', { name: 'Add' })`), not by CSS class. If your test needs a
  class name, the UI is probably not accessible.
- **user-event:** simulates real typing/clicking (focus, keydown, input, keyup).
- **MSW (Mock Service Worker):** intercepts HTTP requests in tests and returns
  fake responses. Your real `axios` code runs; only the network is fake. (Same
  philosophy as `httptest` in Go!)
- **Error boundary:** a component that catches render errors below it and shows
  a fallback instead of a white screen.

## Step 1 — Install & configure

```bash
cd frontend
npm install -D vitest jsdom @testing-library/react @testing-library/user-event @testing-library/jest-dom msw
npm install react-error-boundary
```

`vite.config.ts`: add a `test` section:

```ts
/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
// ...
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { /* unchanged */ },
  test: {
    environment: 'jsdom',          // fake browser DOM
    setupFiles: ['./src/test/setup.ts'],
  },
});
```

`package.json` scripts:

```json
"test": "vitest",
"test:run": "vitest run"
```

## Step 2 — Test setup & MSW

`src/test/handlers.ts`:

```ts
import { http, HttpResponse } from 'msw';
import type { Todo } from '../features/todos/types';

export const sampleTodo: Todo = {
  id: '11111111-1111-1111-1111-111111111111',
  title: 'Write tests',
  description: '',
  completed: false,
  priority: 'high',
  due_date: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

export const handlers = [
  http.get('/api/v1/todos', () =>
    HttpResponse.json({ items: [sampleTodo], page: 1, page_size: 10, total: 1 }),
  ),
  http.post('/api/v1/todos', async ({ request }) => {
    const body = (await request.json()) as { title: string };
    return HttpResponse.json({ ...sampleTodo, id: crypto.randomUUID(), title: body.title }, { status: 201 });
  }),
  // TodosPage also renders the ActivityFeed (Step 5), so tests need this too
  http.get('/api/v1/activity', () => HttpResponse.json({ items: [] })),
];
```

`src/test/server.ts`:

```ts
import { setupServer } from 'msw/node';
import { handlers } from './handlers';

export const server = setupServer(...handlers);
```

`src/test/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterAll, afterEach, beforeAll } from 'vitest';
import { server } from './server';

beforeAll(() => server.listen({ onUnhandledRequest: 'error' })); // unknown request = test bug
afterEach(() => {
  server.resetHandlers(); // undo per-test overrides
  cleanup();              // unmount rendered components
});
afterAll(() => server.close());
```

`src/test/render.tsx`, a helper that wraps components in the providers they need:

```tsx
import type { ReactElement } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

export function renderWithProviders(ui: ReactElement, { route = '/' } = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } }); // fresh cache per test
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}
```

## Step 3 — Component tests

`src/features/todos/components/TodoForm.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { TodoForm } from './TodoForm';

describe('TodoForm', () => {
  it('submits a trimmed title and clears the input', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<TodoForm onSubmit={onSubmit} />);

    const input = screen.getByLabelText('Todo title');
    await user.type(input, '  learn vitest  ');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ title: 'learn vitest', priority: 'medium' }));
    expect(input).toHaveValue('');
  });

  it('disables the button when the title is blank', () => {
    render(<TodoForm onSubmit={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Add' })).toBeDisabled();
  });
});
```

`TodoItem.test.tsx`: write it yourself. Cover: the checkbox calls `onToggle`, an overdue
date shows "overdue", Escape cancels editing, Enter saves a changed title.

## Step 4 — Integration test with MSW

`src/pages/TodosPage.test.tsx`:

```tsx
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { renderWithProviders } from '../test/render';
import { server } from '../test/server';
import { TodosPage } from './TodosPage';

describe('TodosPage', () => {
  it('loads and shows todos from the API', async () => {
    renderWithProviders(<TodosPage />);
    expect(await screen.findByText('Write tests')).toBeInTheDocument(); // findBy* waits
  });

  it('shows the API error message', async () => {
    server.use(
      http.get('/api/v1/todos', () =>
        HttpResponse.json({ error: { code: 'internal_error', message: 'something went wrong' } }, { status: 500 }),
      ),
    );
    renderWithProviders(<TodosPage />);
    expect(await screen.findByRole('alert')).toHaveTextContent('something went wrong');
  });

  it('sends filters as query params', async () => {
    let lastUrl = '';
    server.use(
      http.get('/api/v1/todos', ({ request }) => {
        lastUrl = request.url;
        return HttpResponse.json({ items: [], page: 1, page_size: 10, total: 0 });
      }),
    );
    const user = userEvent.setup();
    renderWithProviders(<TodosPage />);
    await user.click(await screen.findByRole('tab', { name: 'Completed' }));
    await screen.findByText(/nothing here/i);
    expect(new URL(lastUrl).searchParams.get('status')).toBe('completed');
  });
});
```

## Step 5 — Activity feed (uses Backend B8)

`src/features/activity/types.ts`:

```ts
export type ActivityType = 'todos.created' | 'todos.updated' | 'todos.completed' | 'todos.deleted';

export interface Activity {
  id: string;
  todo_id: string;
  type: ActivityType;
  title: string;
  occurred_at: string;
}
```

`src/features/activity/api.ts` + `hooks.ts`:

```ts
// api.ts
import { apiClient } from '../../lib/api-client';
import type { Activity } from './types';

export async function fetchActivity(limit = 20): Promise<Activity[]> {
  const { data } = await apiClient.get<{ items: Activity[] }>('/activity', { params: { limit } });
  return data.items;
}

// hooks.ts
import { useQuery } from '@tanstack/react-query';
import { fetchActivity } from './api';

export const activityKeys = { all: ['activity'] as const };

export function useActivity() {
  return useQuery({
    queryKey: activityKeys.all,
    queryFn: () => fetchActivity(20),
    refetchInterval: 5_000, // events are async: poll so the feed catches up
  });
}
```

`src/features/activity/ActivityFeed.tsx`:

```tsx
import { Spinner } from '../../components/ui/Spinner';
import { useActivity } from './hooks';
import type { ActivityType } from './types';

const labels: Record<ActivityType, string> = {
  'todos.created': '🆕 Created',
  'todos.updated': '✏️ Updated',
  'todos.completed': '✅ Completed',
  'todos.deleted': '🗑️ Deleted',
};

const timeFormat = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });

function timeAgo(iso: string): string {
  const seconds = Math.round((new Date(iso).getTime() - Date.now()) / 1000);
  if (Math.abs(seconds) < 60) return timeFormat.format(seconds, 'second');
  if (Math.abs(seconds) < 3600) return timeFormat.format(Math.round(seconds / 60), 'minute');
  return timeFormat.format(Math.round(seconds / 3600), 'hour');
}

export function ActivityFeed() {
  const { data, isPending, isError } = useActivity();
  return (
    <aside aria-labelledby="activity-heading" className="rounded-lg border border-slate-200 bg-white p-4">
      <h2 id="activity-heading" className="mb-3 font-semibold">Activity</h2>
      {isPending ? (
        <Spinner />
      ) : isError ? (
        <p className="text-sm text-slate-500">Activity is unavailable right now.</p>
      ) : data.length === 0 ? (
        <p className="text-sm text-slate-500">No activity yet.</p>
      ) : (
        <ol className="flex flex-col gap-2 text-sm">
          {data.map((a) => (
            <li key={a.id} className="flex justify-between gap-2">
              <span className="truncate">{labels[a.type]} <strong>{a.title}</strong></span>
              <time dateTime={a.occurred_at} className="shrink-0 text-slate-400">{timeAgo(a.occurred_at)}</time>
            </li>
          ))}
        </ol>
      )}
    </aside>
  );
}
```

Notice the error message: if activity-service is down, **only this panel**
degrades; todos keep working. That's the microservice benefit, visible in the UI.

Also invalidate the feed after todo mutations. In each todo mutation's
`onSuccess`/`onSettled`, add `qc.invalidateQueries({ queryKey: activityKeys.all })`.

Place `<ActivityFeed />` next to the todo list (widen the layout: `max-w-5xl`,
`grid md:grid-cols-[1fr_280px] gap-6`).

## Step 6 — Error boundary

`src/App.tsx`: wrap the routes:

```tsx
import { ErrorBoundary } from 'react-error-boundary';

function Fallback({ error, resetErrorBoundary }: { error: unknown; resetErrorBoundary: () => void }) {
  return (
    <div role="alert" className="mx-auto mt-24 max-w-md text-center">
      <h1 className="text-xl font-bold">Something broke 😵</h1>
      <pre className="mt-2 whitespace-pre-wrap text-sm text-slate-500">
        {error instanceof Error ? error.message : String(error)}
      </pre>
      <button className="mt-4 text-indigo-600 underline" onClick={resetErrorBoundary}>Try again</button>
    </div>
  );
}

// <ErrorBoundary FallbackComponent={Fallback}> <Routes>…</Routes> </ErrorBoundary>
```

## Step 7 — Accessibility & polish checklist

- [ ] Every input has a label (`<label>` or `aria-label`)
- [ ] Every icon-only button has an `aria-label` (✕ delete, edit)
- [ ] The whole app works with the keyboard only (Tab / Shift+Tab / Enter / Space / Escape)
- [ ] Visible focus ring on interactive elements
- [ ] Error messages use `role="alert"`; loading uses `role="status"`
- [ ] Page `<title>` set (`index.html`: "Go-ToDo")
- [ ] Lighthouse (Chrome DevTools) accessibility score ≥ 95

## Step 8 — Frontend CI: `.github/workflows/ci-frontend.yml`

```yaml
name: frontend-ci
on:
  push:
    branches: [main]
    paths: ["frontend/**", ".github/workflows/ci-frontend.yml"]
  pull_request:
    paths: ["frontend/**"]
jobs:
  build:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npm run typecheck
      - run: npm run test:run
      - run: npm run build
```

## Verify

```bash
npm run test:run      # all green
npm run lint && npm run typecheck && npm run build
make up && make open  # activity feed updates a few seconds after each change
docker compose stop activity-service   # feed says "unavailable", todos still work
```

## Commit

```bash
git add -A
git commit -m "feat(frontend): activity feed, error boundary and component tests (F6)"
```

**Next:** [Backend Phase 9 — Observability & CI](../backend/phase-9-observability-and-ci.md),
the last phase. 🎉
