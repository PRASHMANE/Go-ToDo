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
