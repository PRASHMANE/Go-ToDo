import { useEffect } from 'react';
import { ErrorMessage } from '../components/ui/ErrorMessage';
import { Spinner } from '../components/ui/Spinner';
import { ActivityFeed } from '../features/activity/ActivityFeed';
import { getErrorMessage } from '../lib/api-client';
import { Pagination } from '../features/todos/components/Pagination';
import { TodoFilters } from '../features/todos/components/TodoFilters';
import { TodoForm } from '../features/todos/components/TodoForm';
import { TodoList } from '../features/todos/components/TodoList';
import { useCreateTodo, useDeleteTodo, useTodos, useUpdateTodo } from '../features/todos/hooks';
import { useTodoFilters } from '../features/todos/useTodoFilters';

const PAGE_SIZE = 10;

export function TodosPage() {
  const { filters, setFilters } = useTodoFilters(); // client state, stored in the URL
  const todosQuery = useTodos({
    status: filters.status,
    priority: filters.priority,
    q: filters.q || undefined, // axios drops undefined params
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

  const mutationError = createTodo.error ?? updateTodo.error ?? deleteTodo.error;

  return (
    <div className="grid gap-6 md:grid-cols-[1fr_280px]">
      <section className="flex flex-col gap-4" aria-label="Todos">
        <TodoForm
          onSubmit={(input) => createTodo.mutate(input)}
          isSubmitting={createTodo.isPending}
        />

        {mutationError && <ErrorMessage message={getErrorMessage(mutationError)} />}

        <TodoFilters filters={filters} onChange={setFilters} />

        <div className="flex h-5 items-center justify-between text-sm text-slate-500">
          <span>{todosQuery.data ? `${total} todos` : ''}</span>
          {todosQuery.isFetching && !todosQuery.isPending && <span>Updating…</span>}
        </div>

        {todosQuery.isPending ? (
          <Spinner />
        ) : todosQuery.isError ? (
          <ErrorMessage message={getErrorMessage(todosQuery.error)} />
        ) : (
          <>
            <TodoList
              todos={todosQuery.data.items}
              onToggle={(t) => updateTodo.mutate({ id: t.id, input: { completed: !t.completed } })}
              onUpdate={(t, input) => updateTodo.mutate({ id: t.id, input })}
              onDelete={(t) => deleteTodo.mutate(t.id)}
            />
            <Pagination
              page={filters.page}
              pageSize={PAGE_SIZE}
              total={total}
              onPageChange={(page) => setFilters({ page })}
            />
          </>
        )}
      </section>
      <ActivityFeed />
    </div>
  );
}
