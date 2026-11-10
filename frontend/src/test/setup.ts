import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterAll, afterEach, beforeAll } from 'vitest';
import { server } from './server';

beforeAll(() => server.listen({ onUnhandledFrame: 'error' })); // unknown request = test bug
afterEach(() => {
  server.resetHandlers(); // undo per-test overrides
  cleanup(); // unmount rendered components
  localStorage.clear();
});
afterAll(() => server.close());
