import {createRoot} from 'react-dom/client';
import {BrowserRouter} from 'react-router-dom';
import AppPage from '../src/pages/App';
import '../src/styles/index.css';
createRoot(document.getElementById('root')!).render(<BrowserRouter><main style={{padding:24}}><AppPage/></main></BrowserRouter>);
