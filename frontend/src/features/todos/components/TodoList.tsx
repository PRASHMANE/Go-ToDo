import type { Todo, UpdateTodoInput } from '../types';
import { TodoItem } from './TodoItem';

interface TodoListProps {
  todos: Todo[];
  onToggle: (todo: Todo) => void;
  onUpdate: (todo: Todo, input: UpdateTodoInput) => void;
  onDelete: (todo: Todo) => void;
}

export function TodoList({ todos, onToggle, onUpdate, onDelete }: TodoListProps) {
  if (todos.length === 0) {
    return <p className="py-10 text-center text-slate-500">Nothing here. Add your first todo ✨</p>;
  }
  return (
    <ul className="flex flex-col gap-2">
      {todos.map((todo) => (
        <TodoItem key={todo.id} todo={todo} onToggle={onToggle} onUpdate={onUpdate} onDelete={onDelete} />
      ))}
    </ul>
  );
}
