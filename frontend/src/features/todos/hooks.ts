import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { todosApi } from './api';
import type { ListTodosParams, ListTodosResponse, Todo, UpdateTodoInput } from './types';

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

function applyUpdate(todo: Todo, input: UpdateTodoInput): Todo {
  const { clear_due_date, ...fields } = input;
  return { ...todo, ...fields, ...(clear_due_date ? { due_date: null } : {}) };
}

/** Optimistic update: the UI changes instantly and rolls back if the request fails. */
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

export function useDeleteTodo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: todosApi.remove,
    onSuccess: () => qc.invalidateQueries({ queryKey: todoKeys.all }),
  });
}
