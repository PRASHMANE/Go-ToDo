import { useState, type KeyboardEvent } from 'react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { useNow } from '../../../hooks/useNow';
import type { Priority, Todo, UpdateTodoInput } from '../types';

const priorityStyles: Record<Priority, string> = {
  low: 'bg-slate-100 text-slate-600',
  medium: 'bg-amber-100 text-amber-700',
  high: 'bg-red-100 text-red-700',
};

interface TodoItemProps {
  todo: Todo;
  onToggle: (todo: Todo) => void;
  onUpdate: (todo: Todo, input: UpdateTodoInput) => void;
  onDelete: (todo: Todo) => void;
}

export function TodoItem({ todo, onToggle, onUpdate, onDelete }: TodoItemProps) {
  const now = useNow();
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState(todo.title);

  const isOverdue = !todo.completed && todo.due_date !== null && Date.parse(todo.due_date) < now;

  function startEditing() {
    setDraft(todo.title);
    setIsEditing(true);
  }

  function save() {
    const title = draft.trim();
    setIsEditing(false);
    if (title && title !== todo.title) onUpdate(todo, { title });
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') save();
    if (e.key === 'Escape') setIsEditing(false);
  }

  function handleDelete() {
    if (window.confirm(`Delete "${todo.title}"?`)) onDelete(todo);
  }

  return (
    <li className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3">
      <input
        type="checkbox"
        className="h-4 w-4 shrink-0 accent-indigo-600"
        checked={todo.completed}
        onChange={() => onToggle(todo)}
        aria-label={`Mark "${todo.title}" as ${todo.completed ? 'not done' : 'done'}`}
      />
      <div className="min-w-0 flex-1">
        {isEditing ? (
          <Input
            autoFocus
            aria-label="Edit title"
            value={draft}
            maxLength={200}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={save}
            onKeyDown={handleKeyDown}
          />
        ) : (
          <p
            onDoubleClick={startEditing}
            title="Double-click to edit"
            className={`truncate ${todo.completed ? 'text-slate-400 line-through' : ''}`}
          >
            {todo.title}
          </p>
        )}
        {todo.due_date && (
          <p className={`text-xs ${isOverdue ? 'font-medium text-red-600' : 'text-slate-500'}`}>
            Due {new Date(todo.due_date).toLocaleDateString()}
            {isOverdue && ' · overdue'}
          </p>
        )}
      </div>
      <Select
        aria-label={`Priority of "${todo.title}"`}
        value={todo.priority}
        onChange={(e) => onUpdate(todo, { priority: e.target.value as Priority })}
        className={`border-transparent py-1 text-xs font-medium ${priorityStyles[todo.priority]}`}
      >
        <option value="low">low</option>
        <option value="medium">medium</option>
        <option value="high">high</option>
      </Select>
      {!isEditing && (
        <Button variant="ghost" onClick={startEditing} aria-label={`Edit "${todo.title}"`}>
          ✎
        </Button>
      )}
      <Button variant="ghost" onClick={handleDelete} aria-label={`Delete "${todo.title}"`}>
        ✕
      </Button>
    </li>
  );
}
