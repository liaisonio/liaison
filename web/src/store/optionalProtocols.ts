import {create} from 'zustand';

// Fail closed until the authenticated server reports a compiled-in driver.
export const useOptionalProtocols = create<{dameng:boolean}>(() => ({dameng:false}));
export const optionalProtocolEnabled = (value:string) =>
  !['dameng','webdameng'].includes(value) || useOptionalProtocols.getState().dameng;
