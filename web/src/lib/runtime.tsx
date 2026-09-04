import { useEffect } from 'react';
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router-dom';
import { useSession } from '@/store/session';

let navigate: ReturnType<typeof useNavigate> | undefined;

export const history = {
  push(path: string) {
    if (navigate) navigate(path);
    else window.location.assign(path);
  },
  replace(path: string) {
    if (navigate) navigate(path, { replace: true });
    else window.location.replace(path);
  },
  back() {
    if (navigate) navigate(-1);
    else window.history.back();
  },
};

export function RuntimeBridge() {
  const routerNavigate = useNavigate();
  useEffect(() => {
    navigate = routerNavigate;
    return () => {
      navigate = undefined;
    };
  }, [routerNavigate]);
  return null;
}

export function useModel(name: '@@initialState') {
  if (name !== '@@initialState') {
    throw new Error(`Unsupported model: ${name}`);
  }
  const initialState = useSession((state) => state.initialState);
  const setInitialState = useSession((state) => state.setInitialState);
  return { initialState, setInitialState };
}

export { useLocation, useParams, useSearchParams };

