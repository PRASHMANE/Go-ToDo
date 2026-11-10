import { Spinner } from '../../components/ui/Spinner';
import { useNow } from '../../hooks/useNow';
import { useActivity } from './hooks';
import type { ActivityType } from './types';

const labels: Record<ActivityType, string> = {
  'todos.created': '🆕 Created',
  'todos.updated': '✏️ Updated',
  'todos.completed': '✅ Completed',
  'todos.deleted': '🗑️ Deleted',
};

const timeFormat = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });

function timeAgo(iso: string, now: number): string {
  const seconds = Math.round((Date.parse(iso) - now) / 1000);
  if (Math.abs(seconds) < 60) return timeFormat.format(seconds, 'second');
  if (Math.abs(seconds) < 3600) return timeFormat.format(Math.round(seconds / 60), 'minute');
  if (Math.abs(seconds) < 86400) return timeFormat.format(Math.round(seconds / 3600), 'hour');
  return timeFormat.format(Math.round(seconds / 86400), 'day');
}

export function ActivityFeed() {
  const { data, isPending, isError } = useActivity();
  const now = useNow(10_000);
  return (
    <aside aria-labelledby="activity-heading" className="rounded-lg border border-slate-200 bg-white p-4">
      <h2 id="activity-heading" className="mb-3 font-semibold">
        Activity
      </h2>
      {isPending ? (
        <Spinner />
      ) : isError ? (
        <p className="text-sm text-slate-500">Activity is unavailable right now.</p>
      ) : data.length === 0 ? (
        <p className="text-sm text-slate-500">No activity yet.</p>
      ) : (
        <ol className="flex flex-col gap-2 text-sm">
          {data.map((a) => (
            <li key={a.id} className="flex justify-between gap-2">
              <span className="truncate">
                {labels[a.type] ?? a.type} <strong>{a.title}</strong>
              </span>
              <time dateTime={a.occurred_at} className="shrink-0 text-slate-400">
                {timeAgo(a.occurred_at, now)}
              </time>
            </li>
          ))}
        </ol>
      )}
    </aside>
  );
}
