import { useQuery } from '@tanstack/react-query';
import { fetchActivity } from './api';

export const activityKeys = { all: ['activity'] as const };

export function useActivity() {
  return useQuery({
    queryKey: activityKeys.all,
    queryFn: () => fetchActivity(20),
    refetchInterval: 5_000, // events are async: poll so the feed catches up
  });
}
