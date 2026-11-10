import type { FallbackProps } from 'react-error-boundary';

export function ErrorFallback({ error, resetErrorBoundary }: FallbackProps) {
  return (
    <div role="alert" className="mx-auto mt-24 max-w-md text-center">
      <h1 className="text-xl font-bold">Something broke 😵</h1>
      <pre className="mt-2 whitespace-pre-wrap text-sm text-slate-500">
        {error instanceof Error ? error.message : String(error)}
      </pre>
      <button className="mt-4 text-indigo-600 underline" onClick={resetErrorBoundary}>
        Try again
      </button>
    </div>
  );
}
