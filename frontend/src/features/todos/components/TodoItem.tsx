import { Button } from '../../../components/ui/Button';
import { useNow } from '../../../hooks/useNow';
import type { Priority, Todo } from '../types';

const priorityStyles: Record<Priority, string> = {
  low: 'bg-slate-100 text-slate-600',
  medium: 'bg-amber-100 text-amber-700',
  high: 'bg-red-100 text-red-700',
};

interface TodoItemProps {
  todo: Todo;
  onToggle: (todo: Todo) => void;
  onDelete: (todo: Todo) => void;
}

export function TodoItem({ todo, onToggle, onDelete }: TodoItemProps) {
  const now = useNow();
  const isOverdue = !todo.completed && todo.due_date !== null && Date.parse(todo.due_date) < now;

  return (
    <li className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3">
      <input
        type="checkbox"
        className="h-4 w-4 accent-indigo-600"
        checked={todo.completed}
        onChange={() => onToggle(todo)}
        aria-label={`Mark "${todo.title}" as ${todo.completed ? 'not done' : 'done'}`}
      />
      <div className="min-w-0 flex-1">
        <p className={`truncate ${todo.completed ? 'text-slate-400 line-through' : ''}`}>{todo.title}</p>
        {todo.due_date && (
          <p className={`text-xs ${isOverdue ? 'font-medium text-red-600' : 'text-slate-500'}`}>
            Due {new Date(todo.due_date).toLocaleDateString()}
            {isOverdue && ' · overdue'}
          </p>
        )}
      </div>
      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${priorityStyles[todo.priority]}`}>
        {todo.priority}
      </span>
      <Button variant="ghost" onClick={() => onDelete(todo)} aria-label={`Delete "${todo.title}"`}>
        ✕
      </Button>
    </li>
  );
}
