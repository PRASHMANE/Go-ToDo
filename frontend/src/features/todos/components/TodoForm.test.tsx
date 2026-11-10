import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { TodoForm } from './TodoForm';

describe('TodoForm', () => {
  it('submits a trimmed title and clears the input', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<TodoForm onSubmit={onSubmit} />);

    const input = screen.getByLabelText('Todo title');
    await user.type(input, '  learn vitest  ');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'learn vitest', priority: 'medium', due_date: null }),
    );
    expect(input).toHaveValue('');
  });

  it('sends the chosen priority and due date as RFC3339', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<TodoForm onSubmit={onSubmit} />);

    await user.type(screen.getByLabelText('Todo title'), 'ship it');
    await user.selectOptions(screen.getByLabelText('Priority'), 'high');
    await user.type(screen.getByLabelText('Due date'), '2026-12-01');
    await user.keyboard('{Enter}');

    expect(onSubmit).toHaveBeenCalledWith({
      title: 'ship it',
      priority: 'high',
      due_date: '2026-12-01T00:00:00.000Z',
    });
  });

  it('disables the button when the title is blank', () => {
    render(<TodoForm onSubmit={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Add' })).toBeDisabled();
  });
});
