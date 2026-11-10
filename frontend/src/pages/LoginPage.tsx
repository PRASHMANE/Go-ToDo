import { Link, Navigate } from 'react-router';
import { LoginForm } from '../features/auth/components/LoginForm';
import { useAuth } from '../features/auth/useAuth';

export function LoginPage() {
  const { isAuthenticated } = useAuth();
  if (isAuthenticated) return <Navigate to="/" replace />;
  return (
    <div className="mx-auto mt-24 max-w-sm rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
      <h1 className="mb-4 text-2xl font-bold">Sign in</h1>
      <LoginForm />
      <p className="mt-4 text-sm text-slate-600">
        No account?{' '}
        <Link to="/register" className="text-indigo-600 hover:underline">
          Create one
        </Link>
      </p>
    </div>
  );
}
