import { create } from 'zustand';
import { persist } from 'zustand/middleware';

type AvatarState = {
  avatars: Record<string, string>;
  setAvatar: (identity: string, avatar: string) => void;
  removeAvatar: (identity: string) => void;
};

export const getAvatarIdentity = (user?: API.CurrentUser) =>
  String(user?.id ?? user?.email ?? 'anonymous');

export const getAccountLabel = (user?: API.CurrentUser) =>
  user?.name?.trim() || user?.email?.split('@')[0] || '';

export const useAvatarStore = create<AvatarState>()(
  persist(
    (set) => ({
      avatars: {},
      setAvatar: (identity, avatar) =>
        set((state) => ({
          avatars: { ...state.avatars, [identity]: avatar },
        })),
      removeAvatar: (identity) =>
        set((state) => {
          const avatars = { ...state.avatars };
          delete avatars[identity];
          return { avatars };
        }),
    }),
    { name: 'liaison.user-avatars' },
  ),
);
