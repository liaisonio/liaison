import { AppLayout } from '@/components/layout/AppLayout';
import { OptionalProtocolRefresh } from '@/components/OptionalProtocolRefresh';
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
const WebSFTP = lazy(() => import('@/pages/WebSFTP'));
const WebSMB = lazy(() => import('@/pages/WebSMB'));
const WebDesktop = lazy(() => import('@/pages/WebDesktop'));
const WebData = lazy(() => import('@/pages/WebData'));
const WebS3 = lazy(() => import('@/pages/WebS3'));
const AIGateway = lazy(() => import('@/pages/AIGateway'));
const EdgeAgent = lazy(() => import('@/pages/EdgeAgent'));
const AccessEntry = lazy(() => import('@/pages/Proxy/AccessEntry'));

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
      <OptionalProtocolRefresh />
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
            <Route path="/access/agents" element={<FeatureGate code="ai.access.use"><EdgeAgent /></FeatureGate>} />
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
            <Route path="/webssh/:proxyId" element={<AccessEntry family="webssh" />} />
            <Route path="/websftp/:proxyId" element={<WebSFTP />} />
            <Route path="/webssh/sessions/:connectionReference" element={<WebSSH />} />
            <Route
              path="/webssh/:proxyId/connections/:credentialId/*"
              element={<WebSSH />}
            />
            <Route path="/webssh/:proxyId/session/*" element={<WebSSH />} />
            <Route path="/webdesktop/:proxyId" element={<AccessEntry family="webdesktop" />} />
            <Route
              path="/webdesktop/:proxyId/connections/:credentialId/*"
              element={<WebDesktop />}
            />
            <Route path="/webdesktop/:proxyId/session/*" element={<WebDesktop />} />
            <Route path="/webdata/:proxyId" element={<AccessEntry family="webdata" />} />
            <Route path="/webs3/:proxyId/connections/:credentialId/*" element={<WebS3 />} />
            <Route path="/websmb/:proxyId/connections/:credentialId/*" element={<WebSMB />} />
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
