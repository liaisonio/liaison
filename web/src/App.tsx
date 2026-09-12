import { AppLayout } from '@/components/layout/AppLayout';
import { RuntimeBridge } from '@/lib/runtime';
import { FeatureGate, PermissionRefresh } from '@/store/permissions';
import { useSession } from '@/store/session';
import { lazy, Suspense, type ReactNode } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';

const Login = lazy(() => import('@/pages/Login'));
const CliAuth = lazy(() => import('@/pages/CliAuth'));
const Dashboard = lazy(() => import('@/pages/Dashboard'));
const Home = lazy(() => import('@/pages/ManagementAgent'));
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
const AIGateway = lazy(() => import('@/pages/AIGateway'));

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
  return token ? <Navigate to="/" replace /> : <>{children}</>;
}

export default function App() {
  return (
    <>
      <RuntimeBridge />
      <PermissionRefresh />
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
            <Route index element={<FeatureGate code="ai.home.use" home><Home /></FeatureGate>} />
            <Route path="/agent/sessions/:agentSessionId" element={<FeatureGate code="ai.home.use"><Home /></FeatureGate>} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/proxy" element={<Proxy />} />
            <Route path="/ai/:proxyId" element={<AIGateway />} />
            <Route path="/ai/applications/:applicationId" element={<AIGateway />} />
            <Route path="/resource/device" element={<Device />} />
            <Route path="/resource/app" element={<Application />} />
            <Route path="/connector" element={<Connector />} />
            <Route path="/logs/management" element={<FeatureGate code="audit.read"><ManagementLog /></FeatureGate>} />
            <Route path="/logs/audit" element={<FeatureGate code="audit.read"><Audit /></FeatureGate>} />
            <Route path="/audit" element={<Navigate to="/logs/audit" replace />} />
            <Route path="/users" element={<User />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="/webssh/:proxyId" element={<WebSSH />} />
            <Route path="/webssh/sessions/:connectionReference" element={<WebSSH />} />
            <Route
              path="/webssh/:proxyId/connections/:credentialId/*"
              element={<WebSSH />}
            />
            <Route path="/webssh/:proxyId/session/*" element={<WebSSH />} />
            <Route path="/webdesktop/:proxyId" element={<WebDesktop />} />
            <Route
              path="/webdesktop/:proxyId/connections/:credentialId/*"
              element={<WebDesktop />}
            />
            <Route path="/webdesktop/:proxyId/session/*" element={<WebDesktop />} />
            <Route path="/webdata/:proxyId" element={<WebData />} />
            <Route
              path="/webdata/:proxyId/connections/:credentialId/*"
              element={<WebData />}
            />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Suspense>
    </>
  );
}
