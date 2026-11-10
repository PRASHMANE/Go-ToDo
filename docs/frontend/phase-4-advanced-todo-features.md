# Frontend Phase 4 — Editing, URL Filters, Search, Pagination, Optimistic Updates

**Goal:** make the app feel production-quality: inline editing, filters stored
in the URL (shareable, survives refresh), debounced search, pagination, and
instant ("optimistic") checkbox toggles.
**Branch:** `phase/f4-advanced-todos` · **Tag:** `f4`

Backend: `make up` (Docker, Phase B6). Vite proxy still targets `:8080`.

## Concepts

- **URL as state:** filters in `?status=active&priority=high&q=gin&page=2`
  mean refresh and back/forward work, and you can share links. `useSearchParams` reads/writes them.
- **Debounce:** wait until the user stops typing (300 ms) before searching, so
  typing "learn gin" doesn't send 9 requests.
- **Optimistic update:** update the cache *immediately*, send the request, and
  **roll back** if it fails. The UI feels instant.
- **Controlled edit mode:** a todo item switches between "view" and "edit"
  with local state; Escape cancels, Enter saves.

## Files

```
src/hooks/useDebounce.ts
src/features/todos/useTodoFilters.ts
src/features/todos/hooks.ts                    # optimistic useUpdateTodo
src/features/todos/components/TodoFilters.tsx  # + priority + search
src/features/todos/components/TodoItem.tsx     # + edit mode
src/features/todos/components/Pagination.tsx
src/pages/TodosPage.tsx
```

## Step 1 — `src/hooks/useDebounce.ts`

```ts
import { useEffect, useState } from 'react';

/** Returns `value`, but only after it has stopped changing for `delay` ms. */
export function useDebounce<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id); // value changed again → cancel the previous timer
  }, [value, delay]);
  return debounced;
}
```

The cleanup function is the key: each keystroke cancels the previous timer.

## Step 2 — `src/features/todos/useTodoFilters.ts`

```ts
import { useCallback } from 'react';
import { useSearchParams } from 'react-router';
import type { Priority, StatusFilter } from './types';

const STATUSES: StatusFilter[] = ['all', 'active', 'completed'];
const PRIORITIES: Priority[] = ['low', 'medium', 'high'];

export interface TodoFilterState {
  status: StatusFilter;
  priority: Priority | undefined;
  q: string;
  page: number;
}

/** Reads/writes todo filters from the URL query string. */
export function useTodoFilters() {
  const [params, setParams] = useSearchParams();

  // Never trust the URL: validate every value.
  const rawStatus = params.get('status') as StatusFilter | null;
  const rawPriority = params.get('priority') as Priority | null;
  const filters: TodoFilterState = {
    status: rawStatus && STATUSES.includes(rawStatus) ? rawStatus : 'all',
    priority: rawPriority && PRIORITIES.includes(rawPriority) ? rawPriority : undefined,
    q: params.get('q') ?? '',
    page: Math.max(1, Number(params.get('page')) || 1),
  };

  const setFilters = useCallback(
    (next: Partial<TodoFilterState>) => {
      setParams(
        (prev) => {
          const p = new URLSearchParams(prev);
          for (const [key, value] of Object.entries(next)) {
            const isDefault =
              value === undefined || value === '' || (key === 'status' && value === 'all') || (key === 'page' && value === 1);
            if (isDefault) p.delete(key);
            else p.set(key, String(value));
          }
          if (!('page' in next)) p.delete('page'); // any filter change → back to page 1
          return p;
        },
        { replace: true }, // don't spam browser history on every keystroke
      );
    },
    [setParams],
  );

  return { filters, setFilters };
}
```

## Step 3 — Optimistic toggle/edit in `hooks.ts`

Replace `useUpdateTodo`:

```ts
import type { ListTodosResponse, Todo, UpdateTodoInput } from './types';

function applyUpdate(todo: Todo, input: UpdateTodoInput): Todo {
  const { clear_due_date, ...fields } = input;
  return { ...todo, ...fields, ...(clear_due_date ? { due_date: null } : {}) };
}

export function useUpdateTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateTodoInput }) => todosApi.update(id, input),

    // 1. before the request: patch every cached list immediately
    onMutate: async ({ id, input }) => {
      await qc.cancelQueries({ queryKey: todoKeys.lists() }); // don't let an in-flight fetch overwrite us
      const previous = qc.getQueriesData<ListTodosResponse>({ queryKey: todoKeys.lists() });
      qc.setQueriesData<ListTodosResponse>({ queryKey: todoKeys.lists() }, (old) =>
        old ? { ...old, items: old.items.map((t) => (t.id === id ? applyUpdate(t, input) : t)) } : old,
      );
      return { previous }; // → becomes `context` below
    },

    // 2. on failure: put the snapshot back
    onError: (_err, _vars, context) => {
      context?.previous.forEach(([key, data]) => qc.setQueryData(key, data));
    },

    // 3. always: refetch so we end up with the server's truth (e.g. filters)
    onSettled: () => qc.invalidateQueries({ queryKey: todoKeys.all }),
  });
}
```

## Step 4 — `TodoFilters.tsx` with priority and search

```tsx
import { useEffect, useState } from 'react';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { useDebounce } from '../../../hooks/useDebounce';
import type { TodoFilterState } from '../useTodoFilters';
import type { Priority, StatusFilter } from '../types';

const statusOptions: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'completed', label: 'Completed' },
];

interface TodoFiltersProps {
  filters: TodoFilterState;
  onChange: (next: Partial<TodoFilterState>) => void;
}

export function TodoFilters({ filters, onChange }: TodoFiltersProps) {
  const [search, setSearch] = useState(filters.q);
  const debouncedSearch = useDebounce(search.trim(), 300);

  useEffect(() => {
    if (debouncedSearch !== filters.q) onChange({ q: debouncedSearch });
  }, [debouncedSearch, filters.q, onChange]);

  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
      <div role="tablist" aria-label="Filter by status" className="inline-flex rounded-md border border-slate-300 bg-white p-0.5">
        {statusOptions.map((o) => (
          <button
            key={o.value}
            role="tab"
            aria-selected={filters.status === o.value}
            onClick={() => onChange({ status: o.value })}
            className={`rounded px-3 py-1.5 text-sm ${filters.status === o.value ? 'bg-indigo-600 text-white' : 'text-slate-600 hover:bg-slate-100'}`}
          >
            {o.label}
          </button>
        ))}
      </div>
      <Select
        aria-label="Filter by priority"
        value={filters.priority ?? ''}
        onChange={(e) => onChange({ priority: (e.target.value || undefined) as Priority | undefined })}
      >
        <option value="">Any priority</option>
        <option value="high">High</option>
        <option value="medium">Medium</option>
        <option value="low">Low</option>
      </Select>
      <Input
        type="search"
        aria-label="Search todos"
        placeholder="Search…"
        value={search}
        maxLength={100}
        onChange={(e) => setSearch(e.target.value)}
      />
    </div>
  );
}
```

## Step 5 — `TodoItem.tsx` with edit mode

Add an `onUpdate` prop and local edit state. The key parts:

```tsx
import { useState, type KeyboardEvent } from 'react';
import type { Todo, UpdateTodoInput } from '../types';
// ...existing imports

interface TodoItemProps {
  todo: Todo;
  onToggle: (todo: Todo) => void;
  onUpdate: (todo: Todo, input: UpdateTodoInput) => void;
  onDelete: (todo: Todo) => void;
}

export function TodoItem({ todo, onToggle, onUpdate, onDelete }: TodoItemProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState(todo.title);

  function startEditing() {
    setDraft(todo.title);
    setIsEditing(true);
  }

  function save() {
    const title = draft.trim();
    setIsEditing(false);
    if (title && title !== todo.title) onUpdate(todo, { title });
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') save();
    if (e.key === 'Escape') setIsEditing(false);
  }

  // in the JSX, replace the title <p> with:
  //   {isEditing ? (
  //     <Input autoFocus aria-label="Edit title" value={draft} maxLength={200}
  //            onChange={(e) => setDraft(e.target.value)} onBlur={save} onKeyDown={handleKeyDown} />
  //   ) : (
  //     <p onDoubleClick={startEditing} className=...>{todo.title}</p>
  //   )}
  // and add an "Edit" ghost button next to delete that calls startEditing.
  //
  // Priority: turn the badge into a <Select> that calls
  //   onUpdate(todo, { priority: e.target.value as Priority })
  //
  // Delete with confirmation:
  //   onClick={() => { if (confirm(`Delete "${todo.title}"?`)) onDelete(todo); }}
}
```

Finish the JSX yourself. You know all the pieces from F1.

## Step 6 — `Pagination.tsx`

```tsx
import { Button } from '../../../components/ui/Button';

interface PaginationProps {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}

export function Pagination({ page, pageSize, total, onPageChange }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  if (totalPages <= 1) return null;
  return (
    <nav aria-label="Pagination" className="flex items-center justify-between pt-2">
      <Button variant="secondary" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
        ← Previous
      </Button>
      <span className="text-sm text-slate-600">
        Page {page} of {totalPages}
      </span>
      <Button variant="secondary" disabled={page >= totalPages} onClick={() => onPageChange(page + 1)}>
        Next →
      </Button>
    </nav>
  );
}
```

## Step 7 — `TodosPage.tsx` wiring

```tsx
const PAGE_SIZE = 10;

export function TodosPage() {
  const { filters, setFilters } = useTodoFilters();
  const todosQuery = useTodos({
    status: filters.status,
    priority: filters.priority,
    q: filters.q || undefined,          // axios drops undefined params
    page: filters.page,
    page_size: PAGE_SIZE,
  });
  const createTodo = useCreateTodo();
  const updateTodo = useUpdateTodo();
  const deleteTodo = useDeleteTodo();

  // If you delete the last item on page 3, go back to page 2.
  const total = todosQuery.data?.total ?? 0;
  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE));
  useEffect(() => {
    if (todosQuery.isSuccess && filters.page > lastPage) setFilters({ page: lastPage });
  }, [todosQuery.isSuccess, filters.page, lastPage, setFilters]);

  // JSX: <TodoForm/>, <TodoFilters filters={filters} onChange={setFilters}/>,
  // <TodoList ... onUpdate={(t, input) => updateTodo.mutate({ id: t.id, input })}/>,
  // <Pagination page={filters.page} pageSize={PAGE_SIZE} total={total}
  //             onPageChange={(page) => setFilters({ page })}/>
}
```

Pass `onUpdate` through `TodoList` to each `TodoItem`. Show a subtle "Updating…"
indicator with `todosQuery.isFetching && !todosQuery.isPending`.

## Verify

- Filter "active" + "high" + search "gin" → URL shows `?status=active&priority=high&q=gin`. Refresh: same view. Back button: previous filter.
- Type fast in search → Network tab shows **one** request after you stop.
- Create 25 todos (curl loop from B3, with your token) → 3 pages; Next/Previous work.
- Toggle a checkbox → changes instantly. Then `docker compose stop todo-service` and toggle again → it flips, then **flips back** with an error (rollback).
- Double-click a title → edit → Enter saves, Escape cancels.

## Exercises

1. Make the page size selectable (10/20/50) and stored in the URL.
2. Add optimistic **delete** (remove from cache in `onMutate`, restore on error).
3. Add a description field that expands under the title when the item is clicked.

## Commit

```bash
git add -A
git commit -m "feat(frontend): editing, URL filters, search, pagination and optimistic updates (F4)"
```

**Next:** [Phase 5 — Docker + Nginx](phase-5-docker-nginx.md)
