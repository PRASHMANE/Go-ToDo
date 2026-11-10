import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { renderWithProviders } from '../../test/render';
import { server } from '../../test/server';
import { ActivityFeed } from './ActivityFeed';

describe('ActivityFeed', () => {
  it('lists activities with labels', async () => {
    server.use(
      http.get('/api/v1/activity', () =>
        HttpResponse.json({
          items: [
            {
              id: '1',
              todo_id: 't',
              type: 'todos.completed',
              title: 'Ship',
              occurred_at: new Date().toISOString(),
            },
            {
              id: '2',
              todo_id: 't',
              type: 'todos.created',
              title: 'Ship',
              occurred_at: new Date().toISOString(),
            },
          ],
        }),
      ),
    );
    renderWithProviders(<ActivityFeed />);
    expect(await screen.findByText(/✅ Completed/)).toBeInTheDocument();
    expect(screen.getByText(/🆕 Created/)).toBeInTheDocument();
  });

  it('degrades gracefully when activity-service is down', async () => {
    server.use(http.get('/api/v1/activity', () => new HttpResponse(null, { status: 502 })));
    renderWithProviders(<ActivityFeed />);
    expect(await screen.findByText('Activity is unavailable right now.')).toBeInTheDocument();
  });
});
