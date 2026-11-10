# Frontend Phase 1 — Todo UI with Local State

**Goal:** a working todo UI (add, toggle, delete, filter) using **fake data in
`useState`**. No backend yet. You learn components, props, state and events.
**Branch:** `phase/f1-ui-local-state` · **Tag:** `f1`

## Concepts

- **Lifting state up:** the list of todos lives in the *parent* (`TodosPage`).
  Children get data via **props** and report changes via **callback props**
  (`onToggle`, `onDelete`). Data flows down, events flow up.
- **Immutable updates:** never `todos.push(x)`. Create a new array:
  `setTodos(prev => [x, ...prev])`. React compares references to know what changed.
- **Controlled inputs:** an `<input value={title} onChange={e => setTitle(e.target.value)} />`.
  React state is the single source of truth for the input's value.
- **Keys:** `todos.map(t => <TodoItem key={t.id} ... />)`. The `key` lets React
  track items between renders. Use a stable ID, never the array index.

## Files

```
src/features/todos/types.ts
src/components/ui/{Button,Input,Select,Spinner,ErrorMessage}.tsx
src/features/todos/components/{TodoForm,TodoItem,TodoList,TodoFilters}.tsx
src/pages/TodosPage.tsx
src/App.tsx
```

## Step 1 — Types: `src/features/todos/types.ts`

These mirror the Go DTOs from Backend Phase 2 **exactly** (snake_case JSON keys).

```ts
export type Priority = 'low' | 'medium' | 'high';
export type StatusFilter = 'all' | 'active' | 'completed';

export interface Todo {
  id: string;
  title: string;
  description: string;
  completed: boolean;
  priority: Priority;
  due_date: string | null; // ISO 8601 string from Go's time.Time
  created_at: string;
  updated_at: string;
}

export interface CreateTodoInput {
  title: string;
  description?: string;
  priority?: Priority;
  due_date?: string | null;
}

export interface UpdateTodoInput {
  title?: string;
  description?: string;
  completed?: boolean;
  priority?: Priority;
  due_date?: string | null;
  clear_due_date?: boolean;
}

export interface ListTodosParams {
  status?: StatusFilter;
  priority?: Priority;
  q?: string;
  page?: number;
  page_size?: number;
}

export interface ListTodosResponse {
  items: Todo[];
  page: number;
  page_size: number;
  total: number;
}
```

## Step 2 — UI primitives (`src/components/ui/`)

`Button.tsx`:

```tsx
import type { ButtonHTMLAttributes } from 'react';

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost';

const variants: Record<Variant, string> = {
  primary: 'bg-indigo-600 text-white hover:bg-indigo-700',
  secondary: 'bg-white text-slate-700 border border-slate-300 hover:bg-slate-50',
  danger: 'bg-red-600 text-white hover:bg-red-700',
  ghost: 'text-slate-600 hover:bg-slate-100',
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
}

export function Button({ variant = 'primary', className = '', type = 'button', ...props }: ButtonProps) {
  return (
    <button
      type={type}
      className={`inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-50 ${variants[variant]} ${className}`}
      {...props}
    />
  );
}
```

`Input.tsx`:

```tsx
import type { InputHTMLAttributes } from 'react';

export function Input({ className = '', ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={`w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200 ${className}`}
      {...props}
    />
  );
}
```

`Select.tsx`:

```tsx
import type { SelectHTMLAttributes } from 'react';

export function Select({ className = '', ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={`rounded-md border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none ${className}`}
      {...props}
    />
  );
}
```

`Spinner.tsx`:

```tsx
export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return (
    <div role="status" className="flex items-center gap-2 py-6 text-slate-500">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-indigo-600" />
      <span className="text-sm">{label}</span>
    </div>
  );
}
```

`ErrorMessage.tsx`:

```tsx
export function ErrorMessage({ message }: { message: string }) {
  return (
    <p role="alert" className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">
      {message}
    </p>
  );
}
```

## Step 3 — `TodoForm.tsx`

```tsx
import { useState, type FormEvent } from 'react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import type { CreateTodoInput, Priority } from '../types';

interface TodoFormProps {
  onSubmit: (input: CreateTodoInput) => void;
  isSubmitting?: boolean;
}

export function TodoForm({ onSubmit, isSubmitting = false }: TodoFormProps) {
  const [title, setTitle] = useState('');
  const [priority, setPriority] = useState<Priority>('medium');
  const [dueDate, setDueDate] = useState(''); // "YYYY-MM-DD" from <input type="date">

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); // stop the browser's full-page form submit
    const trimmed = title.trim();
    if (!trimmed) return;
    onSubmit({
      title: trimmed,
      priority,
      due_date: dueDate ? new Date(dueDate).toISOString() : null, // Go expects RFC3339
    });
    setTitle('');
    setPriority('medium');
    setDueDate('');
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-2 sm:flex-row">
      <Input
        aria-label="Todo title"
        placeholder="What needs to be done?"
        value={title}
        maxLength={200}
        onChange={(e) => setTitle(e.target.value)}
      />
      <Select aria-label="Priority" value={priority} onChange={(e) => setPriority(e.target.value as Priority)}>
        <option value="low">Low</option>
        <option value="medium">Medium</option>
        <option value="high">High</option>
      </Select>
      <Input
        aria-label="Due date"
        type="date"
        className="sm:w-40"
        value={dueDate}
        onChange={(e) => setDueDate(e.target.value)}
      />
      <Button type="submit" disabled={isSubmitting || !title.trim()}>
        Add
      </Button>
    </form>
  );
}
```

## Step 4 — `TodoItem.tsx`

```tsx
import { Button } from '../../../components/ui/Button';
import type { Priority, Todo } from '../types';

const priorityStyles: Record<Priority, string> = {
  low: 'bg-slate-100 text-slate-600',
  medium: 'bg-amber-100 text-amber-700',
  high: 'bg-red-100 text-red-700',
};

interface TodoItemProps {
  todo: Todo;
  onToggle: (todo: Todo) => void;
  onDelete: (todo: Todo) => void;
}

export function TodoItem({ todo, onToggle, onDelete }: TodoItemProps) {
  const isOverdue = !todo.completed && todo.due_date !== null && new Date(todo.due_date) < new Date();

  return (
    <li className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3">
      <input
        type="checkbox"
        className="h-4 w-4 accent-indigo-600"
        checked={todo.completed}
        onChange={() => onToggle(todo)}
        aria-label={`Mark "${todo.title}" as ${todo.completed ? 'not done' : 'done'}`}
      />
      <div className="min-w-0 flex-1">
        <p className={`truncate ${todo.completed ? 'text-slate-400 line-through' : ''}`}>{todo.title}</p>
        {todo.due_date && (
          <p className={`text-xs ${isOverdue ? 'font-medium text-red-600' : 'text-slate-500'}`}>
            Due {new Date(todo.due_date).toLocaleDateString()}
            {isOverdue && ' · overdue'}
          </p>
        )}
      </div>
      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${priorityStyles[todo.priority]}`}>
        {todo.priority}
      </span>
      <Button variant="ghost" onClick={() => onDelete(todo)} aria-label={`Delete "${todo.title}"`}>
        ✕
      </Button>
    </li>
  );
}
```

## Step 5 — `TodoList.tsx`

```tsx
import type { Todo } from '../types';
import { TodoItem } from './TodoItem';

interface TodoListProps {
  todos: Todo[];
  onToggle: (todo: Todo) => void;
  onDelete: (todo: Todo) => void;
}

export function TodoList({ todos, onToggle, onDelete }: TodoListProps) {
  if (todos.length === 0) {
    return <p className="py-10 text-center text-slate-500">Nothing here. Add your first todo ✨</p>;
  }
  return (
    <ul className="flex flex-col gap-2">
      {todos.map((todo) => (
        <TodoItem key={todo.id} todo={todo} onToggle={onToggle} onDelete={onDelete} />
      ))}
    </ul>
  );
}
```

## Step 6 — `TodoFilters.tsx`

```tsx
import type { StatusFilter } from '../types';

const options: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'completed', label: 'Completed' },
];

interface TodoFiltersProps {
  status: StatusFilter;
  onStatusChange: (status: StatusFilter) => void;
}

export function TodoFilters({ status, onStatusChange }: TodoFiltersProps) {
  return (
    <div role="tablist" aria-label="Filter todos" className="inline-flex rounded-md border border-slate-300 bg-white p-0.5">
      {options.map((o) => (
        <button
          key={o.value}
          role="tab"
          aria-selected={status === o.value}
          onClick={() => onStatusChange(o.value)}
          className={`rounded px-3 py-1.5 text-sm ${status === o.value ? 'bg-indigo-600 text-white' : 'text-slate-600 hover:bg-slate-100'}`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
```

## Step 7 — `src/pages/TodosPage.tsx` (state lives here)

```tsx
import { useState } from 'react';
import { TodoFilters } from '../features/todos/components/TodoFilters';
import { TodoForm } from '../features/todos/components/TodoForm';
import { TodoList } from '../features/todos/components/TodoList';
import type { CreateTodoInput, StatusFilter, Todo } from '../features/todos/types';

const now = new Date().toISOString();
const initialTodos: Todo[] = [
  { id: '1', title: 'Install Go', description: '', completed: true, priority: 'high', due_date: null, created_at: now, updated_at: now },
  { id: '2', title: 'Build todo-service', description: '', completed: false, priority: 'medium', due_date: null, created_at: now, updated_at: now },
];

export function TodosPage() {
  const [todos, setTodos] = useState<Todo[]>(initialTodos);
  const [status, setStatus] = useState<StatusFilter>('all');

  // Derived data: computed during render, NOT stored in state.
  const visible = todos.filter((t) =>
    status === 'all' ? true : status === 'active' ? !t.completed : t.completed,
  );
  const remaining = todos.filter((t) => !t.completed).length;

  function addTodo(input: CreateTodoInput) {
    const ts = new Date().toISOString();
    const todo: Todo = {
      id: crypto.randomUUID(),
      title: input.title,
      description: input.description ?? '',
      completed: false,
      priority: input.priority ?? 'medium',
      due_date: input.due_date ?? null,
      created_at: ts,
      updated_at: ts,
    };
    setTodos((prev) => [todo, ...prev]);
  }

  function toggleTodo(todo: Todo) {
    setTodos((prev) => prev.map((t) => (t.id === todo.id ? { ...t, completed: !t.completed } : t)));
  }

  function deleteTodo(todo: Todo) {
    setTodos((prev) => prev.filter((t) => t.id !== todo.id));
  }

  return (
    <section className="flex flex-col gap-4">
      <TodoForm onSubmit={addTodo} />
      <div className="flex items-center justify-between">
        <TodoFilters status={status} onStatusChange={setStatus} />
        <span className="text-sm text-slate-500">{remaining} left</span>
      </div>
      <TodoList todos={visible} onToggle={toggleTodo} onDelete={deleteTodo} />
    </section>
  );
}
```

## Step 8 — `src/App.tsx`

```tsx
import { TodosPage } from './pages/TodosPage';

export default function App() {
  return (
    <main className="mx-auto max-w-2xl p-6">
      <h1 className="mb-6 text-3xl font-bold">Go-ToDo</h1>
      <TodosPage />
    </main>
  );
}
```

## Verify

- Add a todo with a past due date → shows "overdue" in red.
- Toggle, delete, switch filters; the "left" counter updates.
- Refresh the page → back to the 2 fake todos (state is in memory; F2 fixes it).
- `npm run lint && npm run typecheck` pass.
- Use the keyboard only (Tab, Space, Enter). Does everything work?

## Exercises

1. Add a "Clear completed" button. Where does the function live, and how does the button get it?
2. Why is `visible` a plain `const` and not `useState`? What bug would storing it in state cause?
3. Open React DevTools (browser extension) and watch which components re-render when you type in the form.

## Commit

```bash
git add -A
git commit -m "feat(frontend): todo UI components with local state (F1)"
```

**Next:** [Backend Phase 3 — PostgreSQL](../backend/phase-3-postgres-and-migrations.md), then
[Frontend Phase 2 — Connect the API](phase-2-connect-api.md).
