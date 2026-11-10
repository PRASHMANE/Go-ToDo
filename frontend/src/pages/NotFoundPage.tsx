import { Link } from 'react-router';

export function NotFoundPage() {
  return (
    <div className="mx-auto mt-24 max-w-sm text-center">
      <h1 className="text-2xl font-bold">Page not found</h1>
      <p className="mt-2 text-slate-600">The page you are looking for doesn't exist.</p>
      <Link to="/" className="mt-4 inline-block text-indigo-600 hover:underline">
        Back to my todos
      </Link>
    </div>
  );
}
