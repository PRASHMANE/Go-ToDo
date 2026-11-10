import { useCallback } from 'react';
import { useSearchParams } from 'react-router';
import type { Priority, StatusFilter } from './types';

const STATUSES: StatusFilter[] = ['all', 'active', 'completed'];
const PRIORITIES: Priority[] = ['low', 'medium', 'high'];

export interface TodoFilterState {
  status: StatusFilter;
  priority: Priority | undefined;
  q: string;
  page: number;
}

/** Reads/writes todo filters from the URL query string. */
export function useTodoFilters() {
  const [params, setParams] = useSearchParams();

  // Never trust the URL: validate every value.
  const rawStatus = params.get('status') as StatusFilter | null;
  const rawPriority = params.get('priority') as Priority | null;
  const filters: TodoFilterState = {
    status: rawStatus && STATUSES.includes(rawStatus) ? rawStatus : 'all',
    priority: rawPriority && PRIORITIES.includes(rawPriority) ? rawPriority : undefined,
    q: params.get('q') ?? '',
    page: Math.max(1, Number(params.get('page')) || 1),
  };

  const setFilters = useCallback(
    (next: Partial<TodoFilterState>) => {
      setParams(
        (prev) => {
          const p = new URLSearchParams(prev);
          for (const [key, value] of Object.entries(next)) {
            const isDefault =
              value === undefined ||
              value === '' ||
              (key === 'status' && value === 'all') ||
              (key === 'page' && value === 1);
            if (isDefault) p.delete(key);
            else p.set(key, String(value));
          }
          if (!('page' in next)) p.delete('page'); // any filter change → back to page 1
          return p;
        },
        { replace: true }, // don't spam browser history on every keystroke
      );
    },
    [setParams],
  );

  return { filters, setFilters };
}
