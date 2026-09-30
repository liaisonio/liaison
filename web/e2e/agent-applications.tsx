import {createRoot} from 'react-dom/client';
import {BrowserRouter} from 'react-router-dom';
import AgentApplications from '../src/pages/EdgeAgent/Applications';
import '../src/styles/index.css';
createRoot(document.getElementById('root')!).render(<BrowserRouter><main style={{padding:24}}><AgentApplications/></main></BrowserRouter>);
