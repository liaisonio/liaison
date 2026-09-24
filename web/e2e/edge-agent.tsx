import {createRoot} from 'react-dom/client';
import {BrowserRouter} from 'react-router-dom';
import EdgeAgent from '../src/pages/EdgeAgent';
import '../src/styles/index.css';
createRoot(document.getElementById('root')!).render(<BrowserRouter><main style={{padding:24}}><EdgeAgent/></main></BrowserRouter>);
