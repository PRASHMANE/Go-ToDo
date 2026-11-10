import { Navigate, Outlet, useLocation } from 'react-router';
import { Spinner } from '../../components/ui/Spinner';
import { useAuth } from './useAuth';

export function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) return <Spinner label="Checking your session…" />;
  if (!isAuthenticated) {
    // remember where the user wanted to go
    return <Navigate to="/login" replace state={{ from: location }} />;
  }
  return <Outlet />;
}
