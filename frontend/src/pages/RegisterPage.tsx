import { Link, Navigate } from 'react-router';
import { RegisterForm } from '../features/auth/components/RegisterForm';
import { useAuth } from '../features/auth/useAuth';

export function RegisterPage() {
  const { isAuthenticated } = useAuth();
  if (isAuthenticated) return <Navigate to="/" replace />;
  return (
    <div className="mx-auto mt-24 max-w-sm rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
      <h1 className="mb-4 text-2xl font-bold">Create your account</h1>
      <RegisterForm />
      <p className="mt-4 text-sm text-slate-600">
        Already have an account?{' '}
        <Link to="/login" className="text-indigo-600 hover:underline">
          Sign in
        </Link>
      </p>
    </div>
  );
}
