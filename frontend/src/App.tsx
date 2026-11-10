import { ErrorBoundary } from 'react-error-boundary';
import { Route, Routes } from 'react-router';
import { ErrorFallback } from './components/ErrorFallback';
import { AppLayout } from './components/layout/AppLayout';
import { ProtectedRoute } from './features/auth/ProtectedRoute';
import { LoginPage } from './pages/LoginPage';
import { NotFoundPage } from './pages/NotFoundPage';
import { RegisterPage } from './pages/RegisterPage';
import { TodosPage } from './pages/TodosPage';

export default function App() {
  return (
    // Catches render errors anywhere below and shows a fallback instead of a white screen.
    <ErrorBoundary FallbackComponent={ErrorFallback}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />

        {/* guard → layout → page */}
        <Route element={<ProtectedRoute />}>
          <Route element={<AppLayout />}>
            <Route index element={<TodosPage />} />
          </Route>
        </Route>

        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </ErrorBoundary>
  );
}
