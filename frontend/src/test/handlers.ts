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
  http.get('/api/v1/todos', () => HttpResponse.json({ items: [sampleTodo], page: 1, page_size: 10, total: 1 })),
  http.post('/api/v1/todos', async ({ request }) => {
    const body = (await request.json()) as { title: string };
    return HttpResponse.json({ ...sampleTodo, id: crypto.randomUUID(), title: body.title }, { status: 201 });
  }),
  http.patch('/api/v1/todos/:id', async ({ request, params }) => {
    const body = (await request.json()) as Partial<Todo>;
    return HttpResponse.json({ ...sampleTodo, id: params.id as string, ...body });
  }),
  // TodosPage also renders the ActivityFeed, so tests need this too
  http.get('/api/v1/activity', () => HttpResponse.json({ items: [] })),
];
