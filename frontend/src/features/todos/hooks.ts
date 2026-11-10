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
