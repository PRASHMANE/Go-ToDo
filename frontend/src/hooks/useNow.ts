import { useEffect, useState } from 'react';

/**
 * Current time in ms, refreshed every `intervalMs`.
 * Reading the clock during render is "impure" (different result each render),
 * so we keep it in state and update it on a timer instead.
 */
export function useNow(intervalMs = 60_000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}
