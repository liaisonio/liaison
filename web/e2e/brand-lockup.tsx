import React from 'react';
import { createRoot } from 'react-dom/client';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AppLayout } from '../src/components/layout/AppLayout';
import { RuntimeBridge } from '../src/lib/runtime';
import { useSession } from '../src/store/session';
import { applyThemeOnBoot } from '../src/store/theme';
import '../src/styles/index.css';

if (!import.meta.env.DEV) throw Error('Development fixture only');
applyThemeOnBoot();
useSession.getState().setToken('brand-fixture');
void useSession.getState().setInitialState({
  currentUser: { id: 1, name: 'Brand preview' } as API.CurrentUser,
});
createRoot(document.getElementById('root')!).render(
  <MemoryRouter initialEntries={['/']}>
    <RuntimeBridge />
    <Routes><Route element={<AppLayout />}><Route path="/" element={<div />} /></Route></Routes>
  </MemoryRouter>,
);
