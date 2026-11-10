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
