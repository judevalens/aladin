import type { StateCreator } from "zustand";

// Live state for the Signals surface (the claim feed). The "signal.changed" realtime event
// bumps `nonce` (so an open Signals view refetches) and lights `unread` (the rail dot). Ephemeral.
export interface SignalsSlice {
  signalsUnread: boolean;
  signalsNonce: number;
  bumpSignals: () => void;
  clearSignalsUnread: () => void;
}

export const createSignalsSlice: StateCreator<SignalsSlice, [], [], SignalsSlice> = (set) => ({
  signalsUnread: false,
  signalsNonce: 0,
  bumpSignals: () => set((state) => ({ signalsUnread: true, signalsNonce: state.signalsNonce + 1 })),
  clearSignalsUnread: () => set({ signalsUnread: false }),
});
