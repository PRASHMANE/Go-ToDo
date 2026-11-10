import { Outlet } from 'react-router';
import { useAuth } from '../../features/auth/useAuth';
import { Button } from '../ui/Button';

export function AppLayout() {
  const { user, logout } = useAuth();
  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-2xl items-center justify-between p-4">
          <span className="text-xl font-bold">Go-ToDo</span>
          <div className="flex items-center gap-3 text-sm">
            <span className="text-slate-600">{user?.name}</span>
            <Button variant="secondary" onClick={logout}>
              Log out
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-2xl p-6">
        <Outlet />
      </main>
    </div>
  );
}
