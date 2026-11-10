import type { StatusFilter } from '../types';

const options: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'completed', label: 'Completed' },
];

interface TodoFiltersProps {
  status: StatusFilter;
  onStatusChange: (status: StatusFilter) => void;
}

export function TodoFilters({ status, onStatusChange }: TodoFiltersProps) {
  return (
    <div role="tablist" aria-label="Filter todos" className="inline-flex rounded-md border border-slate-300 bg-white p-0.5">
      {options.map((o) => (
        <button
          key={o.value}
          role="tab"
          aria-selected={status === o.value}
          onClick={() => onStatusChange(o.value)}
          className={`rounded px-3 py-1.5 text-sm ${status === o.value ? 'bg-indigo-600 text-white' : 'text-slate-600 hover:bg-slate-100'}`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
