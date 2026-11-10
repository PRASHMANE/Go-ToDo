import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000, // data is "fresh" for 30s → no refetch on every mount
      retry: 1, // retry failed requests once
      refetchOnWindowFocus: true, // refresh when you come back to the tab
    },
  },
});
