import { AppLayout } from '@/components/layout/AppLayout';
import { RuntimeBridge } from '@/lib/runtime';
import { useSession } from '@/store/session';
import { lazy, Suspense, type ReactNode } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';

const Login = lazy(() => import('@/pages/Login'));
const CliAuth = lazy(() => import('@/pages/CliAuth'));
const Dashboard = lazy(() => import('@/pages/Dashboard'));
const Proxy = lazy(() => import('@/pages/Proxy'));
const Device = lazy(() => import('@/pages/Device'));
const Application = lazy(() => import('@/pages/App'));
const Connector = lazy(() => import('@/pages/Connector'));
const Audit = lazy(() => import('@/pages/Audit'));
const ManagementLog = lazy(() => import('@/pages/ManagementLog'));
const User = lazy(() => import('@/pages/User'));
const Settings = lazy(() => import('@/pages/Settings'));
const WebSSH = lazy(() => import('@/pages/WebSSH'));
const WebDesktop = lazy(() => import('@/pages/WebDesktop'));
const WebData = lazy(() => import('@/pages/WebData'));

function RequireAuth({ children }: { children: ReactNode }) {
  const token = useSession((state) => state.token);
  const location = useLocation();
  if (!token) {
    const redirect = `${location.pathname}${location.search}`;
    return (
      <Navigate
        to={`/login?redirect=${encodeURIComponent(redirect)}`}
        replace
      />
    );
  }
  return <>{children}</>;
}

function PublicOnly({ children }: { children: ReactNode }) {
  const token = useSession((state) => state.token);
  return token ? <Navigate to="/dashboard" replace /> : <>{children}</>;
}

export default function App() {
  return (
    <>
      <RuntimeBridge />
      <Suspense
        fallback={
          <div className="liaison-route-loading">
            <span />
          </div>
        }
      >
        <Routes>
          <Route
            path="/login"
            element={
              <PublicOnly>
                <Login />
              </PublicOnly>
            }
          />
          <Route path="/cli-auth" element={<CliAuth />} />
          <Route
            element={
              <RequireAuth>
                <AppLayout />
              </RequireAuth>
            }
          >
            <Route index element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/proxy" element={<Proxy />} />
            <Route path="/resource/device" element={<Device />} />
            <Route path="/resource/app" element={<Application />} />
            <Route path="/connector" element={<Connector />} />
            <Route path="/logs/management" element={<ManagementLog />} />
            <Route path="/logs/audit" element={<Audit />} />
            <Route path="/audit" element={<Navigate to="/logs/audit" replace />} />
            <Route path="/users" element={<User />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="/webssh/:proxyId" element={<WebSSH />} />
            <Route
              path="/webssh/:proxyId/connections/:credentialId"
              element={<WebSSH />}
            />
            <Route path="/webssh/:proxyId/session" element={<WebSSH />} />
            <Route path="/webdesktop/:proxyId" element={<WebDesktop />} />
            <Route
              path="/webdesktop/:proxyId/connections/:credentialId"
              element={<WebDesktop />}
            />
            <Route path="/webdesktop/:proxyId/session" element={<WebDesktop />} />
            <Route path="/webdata/:proxyId" element={<WebData />} />
            <Route
              path="/webdata/:proxyId/connections/:credentialId"
              element={<WebData />}
            />
          </Route>
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </Suspense>
    </>
  );
}
