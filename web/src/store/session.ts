import { create } from 'zustand';

export type InitialState = {
  currentUser?: API.CurrentUser;
  fetchUserInfo?: () => Promise<API.CurrentUser | undefined>;
};

type SessionStore = {
  token: string | null;
  initialState: InitialState;
  setToken: (token: string | null) => void;
  setInitialState: (
    next: InitialState | ((previous: InitialState) => InitialState),
  ) => Promise<void>;
  clear: () => void;
};

export const useSession = create<SessionStore>((set) => ({
  token: localStorage.getItem('token'),
  initialState: {},
  setToken: (token) => {
    if (token) localStorage.setItem('token', token);
    else localStorage.removeItem('token');
    set({ token });
  },
  setInitialState: async (next) => {
    set((state) => ({
      initialState:
        typeof next === 'function' ? next(state.initialState) : next,
    }));
  },
  clear: () => {
    localStorage.removeItem('token');
    set({ token: null, initialState: {} });
  },
}));

export const getToken = () => useSession.getState().token;

