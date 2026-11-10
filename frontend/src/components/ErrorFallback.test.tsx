import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { ErrorBoundary } from 'react-error-boundary';
import { describe, expect, it, vi } from 'vitest';
import { ErrorFallback } from './ErrorFallback';

function Bomb({ explode }: { explode: boolean }) {
  if (explode) throw new Error('kaboom');
  return <p>all good</p>;
}

function Harness() {
  const [explode, setExplode] = useState(true);
  return (
    <ErrorBoundary FallbackComponent={ErrorFallback} onReset={() => setExplode(false)}>
      <Bomb explode={explode} />
    </ErrorBoundary>
  );
}

describe('ErrorFallback', () => {
  it('shows the error instead of a white screen, and recovers on "Try again"', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {}); // React logs caught errors
    render(<Harness />);
    expect(screen.getByRole('alert')).toHaveTextContent('kaboom');
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(screen.getByText('all good')).toBeInTheDocument();
  });
});
