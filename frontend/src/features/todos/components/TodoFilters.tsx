import { useEffect, useState } from 'react';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { useDebounce } from '../../../hooks/useDebounce';
import type { Priority, StatusFilter } from '../types';
import type { TodoFilterState } from '../useTodoFilters';

const statusOptions: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'completed', label: 'Completed' },
];

interface TodoFiltersProps {
  filters: TodoFilterState;
  onChange: (next: Partial<TodoFilterState>) => void;
}

export function TodoFilters({ filters, onChange }: TodoFiltersProps) {
  const [search, setSearch] = useState(filters.q);
  const debouncedSearch = useDebounce(search.trim(), 300);

  useEffect(() => {
    if (debouncedSearch !== filters.q) onChange({ q: debouncedSearch });
  }, [debouncedSearch, filters.q, onChange]);

  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
      <div
        role="tablist"
        aria-label="Filter by status"
        className="inline-flex shrink-0 rounded-md border border-slate-300 bg-white p-0.5"
      >
        {statusOptions.map((o) => (
          <button
            key={o.value}
            role="tab"
            aria-selected={filters.status === o.value}
            onClick={() => onChange({ status: o.value })}
            className={`rounded px-3 py-1.5 text-sm ${filters.status === o.value ? 'bg-indigo-600 text-white' : 'text-slate-600 hover:bg-slate-100'}`}
          >
            {o.label}
          </button>
        ))}
      </div>
      <Select
        aria-label="Filter by priority"
        value={filters.priority ?? ''}
        onChange={(e) => onChange({ priority: (e.target.value || undefined) as Priority | undefined })}
      >
        <option value="">Any priority</option>
        <option value="high">High</option>
        <option value="medium">Medium</option>
        <option value="low">Low</option>
      </Select>
      <Input
        type="search"
        aria-label="Search todos"
        placeholder="Search…"
        value={search}
        maxLength={100}
        onChange={(e) => setSearch(e.target.value)}
      />
    </div>
  );
}
