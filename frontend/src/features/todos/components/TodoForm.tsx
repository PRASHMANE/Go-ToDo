import { useState, type FormEvent } from 'react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import type { CreateTodoInput, Priority } from '../types';

interface TodoFormProps {
  onSubmit: (input: CreateTodoInput) => void;
  isSubmitting?: boolean;
}

export function TodoForm({ onSubmit, isSubmitting = false }: TodoFormProps) {
  const [title, setTitle] = useState('');
  const [priority, setPriority] = useState<Priority>('medium');
  const [dueDate, setDueDate] = useState(''); // "YYYY-MM-DD" from <input type="date">

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); // stop the browser's full-page form submit
    const trimmed = title.trim();
    if (!trimmed) return;
    onSubmit({
      title: trimmed,
      priority,
      due_date: dueDate ? new Date(dueDate).toISOString() : null, // Go expects RFC3339
    });
    setTitle('');
    setPriority('medium');
    setDueDate('');
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-2 sm:flex-row">
      <Input
        aria-label="Todo title"
        placeholder="What needs to be done?"
        value={title}
        maxLength={200}
        onChange={(e) => setTitle(e.target.value)}
      />
      <Select aria-label="Priority" value={priority} onChange={(e) => setPriority(e.target.value as Priority)}>
        <option value="low">Low</option>
        <option value="medium">Medium</option>
        <option value="high">High</option>
      </Select>
      <Input
        aria-label="Due date"
        type="date"
        className="sm:w-40"
        value={dueDate}
        onChange={(e) => setDueDate(e.target.value)}
      />
      <Button type="submit" disabled={isSubmitting || !title.trim()}>
        Add
      </Button>
    </form>
  );
}
