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

  const todosQuery = useTodos({ status, page_size: 50 }); // server state
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
