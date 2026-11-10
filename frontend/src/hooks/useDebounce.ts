import { useEffect, useState } from 'react';

/** Returns `value`, but only after it has stopped changing for `delay` ms. */
export function useDebounce<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id); // value changed again → cancel the previous timer
  }, [value, delay]);
  return debounced;
}
