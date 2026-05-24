import { createContext, useContext } from "react";
import type { AppComposition } from "@/app/composition/create-app-composition";

export const AppCompositionContext = createContext<AppComposition | null>(null);

export function useAppComposition() {
  const context = useContext(AppCompositionContext);
  if (!context) {
    throw new Error("useAppComposition must be used inside AppProviders");
  }
  return context;
}
