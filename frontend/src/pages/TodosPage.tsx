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
