import { apiClient } from '../../lib/api-client';
import type { Activity } from './types';

export async function fetchActivity(limit = 20): Promise<Activity[]> {
  const { data } = await apiClient.get<{ items: Activity[] }>('/activity', { params: { limit } });
  return data.items;
}
