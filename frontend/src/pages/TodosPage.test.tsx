import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { sampleTodo } from '../test/handlers';
import { renderWithProviders } from '../test/render';
import { server } from '../test/server';
import { TodosPage } from './TodosPage';

describe('TodosPage', () => {
  it('loads and shows todos from the API', async () => {
    renderWithProviders(<TodosPage />);
    expect(await screen.findByText('Write tests')).toBeInTheDocument(); // findBy* waits
    expect(screen.getByText('1 todos')).toBeInTheDocument();
  });

  it('shows the API error message', async () => {
    server.use(
      http.get('/api/v1/todos', () =>
        HttpResponse.json(
          { error: { code: 'internal_error', message: 'something went wrong' } },
          { status: 500 },
        ),
      ),
    );
    renderWithProviders(<TodosPage />);
    expect(await screen.findByRole('alert')).toHaveTextContent('something went wrong');
  });

  it('sends filters as query params', async () => {
    let lastUrl = '';
    server.use(
      http.get('/api/v1/todos', ({ request }) => {
        lastUrl = request.url;
        return HttpResponse.json({ items: [], page: 1, page_size: 10, total: 0 });
      }),
    );
    const user = userEvent.setup();
    renderWithProviders(<TodosPage />);
    await user.click(await screen.findByRole('tab', { name: 'Completed' }));
    await user.selectOptions(screen.getByLabelText('Filter by priority'), 'high');
    await waitFor(() => {
      const params = new URL(lastUrl).searchParams;
      expect(params.get('status')).toBe('completed');
      expect(params.get('priority')).toBe('high');
    });
  });

  it('reads initial filters from the URL', async () => {
    let lastUrl = '';
    server.use(
      http.get('/api/v1/todos', ({ request }) => {
        lastUrl = request.url;
        return HttpResponse.json({ items: [], page: 2, page_size: 10, total: 15 });
      }),
    );
    renderWithProviders(<TodosPage />, { route: '/?status=active&q=gin&page=2' });
    expect(await screen.findByText('Page 2 of 2')).toBeInTheDocument();
    const params = new URL(lastUrl).searchParams;
    expect(params.get('status')).toBe('active');
    expect(params.get('q')).toBe('gin');
    expect(params.get('page')).toBe('2');
  });

  it('debounces search into a single request', async () => {
    const searches: string[] = [];
    server.use(
      http.get('/api/v1/todos', ({ request }) => {
        const q = new URL(request.url).searchParams.get('q');
        if (q) searches.push(q);
        return HttpResponse.json({ items: [], page: 1, page_size: 10, total: 0 });
      }),
    );
    const user = userEvent.setup();
    renderWithProviders(<TodosPage />);
    await user.type(await screen.findByLabelText('Search todos'), 'learn gin');
    await waitFor(() => expect(searches).toEqual(['learn gin']));
  });

  it('toggles optimistically and rolls back when the API fails', async () => {
    server.use(
      http.patch('/api/v1/todos/:id', async () => {
        await new Promise((r) => setTimeout(r, 50));
        return HttpResponse.json(
          { error: { code: 'internal_error', message: 'boom' } },
          { status: 500 },
        );
      }),
    );
    const user = userEvent.setup();
    renderWithProviders(<TodosPage />);
    const checkbox = await screen.findByRole('checkbox', { name: /mark "write tests"/i });
    expect(checkbox).not.toBeChecked();

    await user.click(checkbox);
    expect(checkbox).toBeChecked(); // instantly, before the server answers

    expect(await screen.findByText('boom')).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('checkbox', { name: /mark "write tests"/i })).not.toBeChecked(),
    ); // rolled back
  });

  it('creates a todo and refetches the list', async () => {
    let created = false;
    server.use(
      http.post('/api/v1/todos', async ({ request }) => {
        created = true;
        const body = (await request.json()) as { title: string };
        return HttpResponse.json({ ...sampleTodo, id: 'new', title: body.title }, { status: 201 });
      }),
      http.get('/api/v1/todos', () =>
        HttpResponse.json({
          items: created
            ? [{ ...sampleTodo, id: 'new', title: 'Brand new' }, sampleTodo]
            : [sampleTodo],
          page: 1,
          page_size: 10,
          total: created ? 2 : 1,
        }),
      ),
    );
    const user = userEvent.setup();
    renderWithProviders(<TodosPage />);
    await user.type(await screen.findByLabelText('Todo title'), 'Brand new{Enter}');
    expect(await screen.findByText('Brand new')).toBeInTheDocument();
    expect(screen.getByText('2 todos')).toBeInTheDocument();
  });
});
