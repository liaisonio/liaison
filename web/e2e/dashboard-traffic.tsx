import React from 'react';
import {createRoot} from 'react-dom/client';
import {MemoryRouter} from 'react-router-dom';
import Dashboard from '../src/pages/Dashboard';
import {applyThemeOnBoot} from '../src/store/theme';
import '../src/styles/index.css';
if(!import.meta.env.DEV)throw Error('Development only');
applyThemeOnBoot();
createRoot(document.getElementById('root')!).render(<MemoryRouter><main style={{padding:24}}><Dashboard/></main></MemoryRouter>);
