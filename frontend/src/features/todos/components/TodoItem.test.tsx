import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { sampleTodo } from '../../../test/handlers';
import type { Todo } from '../types';
import { TodoItem } from './TodoItem';

function renderItem(overrides: Partial<Todo> = {}) {
  const props = { onToggle: vi.fn(), onUpdate: vi.fn(), onDelete: vi.fn() };
  const todo = { ...sampleTodo, ...overrides };
  render(
    <ul>
      <TodoItem todo={todo} {...props} />
    </ul>,
  );
  return { todo, ...props };
}

afterEach(() => vi.restoreAllMocks());

describe('TodoItem', () => {
  it('calls onToggle when the checkbox is clicked', async () => {
    const { todo, onToggle } = renderItem();
    await userEvent.click(screen.getByRole('checkbox', { name: /mark "write tests" as done/i }));
    expect(onToggle).toHaveBeenCalledWith(todo);
  });

  it('marks overdue todos', () => {
    renderItem({ due_date: '2000-01-01T00:00:00Z' });
    expect(screen.getByText(/overdue/)).toBeInTheDocument();
  });

  it('does not mark completed todos as overdue', () => {
    renderItem({ due_date: '2000-01-01T00:00:00Z', completed: true });
    expect(screen.queryByText(/overdue/)).not.toBeInTheDocument();
  });

  it('saves an edited title on Enter', async () => {
    const user = userEvent.setup();
    const { todo, onUpdate } = renderItem();
    await user.click(screen.getByRole('button', { name: /edit "write tests"/i }));
    const input = screen.getByLabelText('Edit title');
    await user.clear(input);
    await user.type(input, 'Write more tests{Enter}');
    expect(onUpdate).toHaveBeenCalledWith(todo, { title: 'Write more tests' });
    expect(screen.queryByLabelText('Edit title')).not.toBeInTheDocument();
  });

  it('cancels editing on Escape without saving', async () => {
    const user = userEvent.setup();
    const { onUpdate } = renderItem();
    await user.dblClick(screen.getByText('Write tests'));
    await user.type(screen.getByLabelText('Edit title'), ' changed{Escape}');
    expect(onUpdate).not.toHaveBeenCalled();
    expect(screen.getByText('Write tests')).toBeInTheDocument();
  });

  it('changes priority', async () => {
    const { todo, onUpdate } = renderItem();
    await userEvent.selectOptions(screen.getByLabelText(/priority of/i), 'low');
    expect(onUpdate).toHaveBeenCalledWith(todo, { priority: 'low' });
  });

  it('deletes only after confirmation', async () => {
    const confirm = vi
      .spyOn(window, 'confirm')
      .mockReturnValueOnce(false)
      .mockReturnValueOnce(true);
    const { todo, onDelete } = renderItem();
    const btn = screen.getByRole('button', { name: /delete "write tests"/i });
    await userEvent.click(btn);
    expect(onDelete).not.toHaveBeenCalled();
    await userEvent.click(btn);
    expect(onDelete).toHaveBeenCalledWith(todo);
    expect(confirm).toHaveBeenCalledTimes(2);
  });
});
