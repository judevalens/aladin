import { createContext, type PropsWithChildren, useContext, useMemo } from "react";
import type { AppComposition } from "@/app/composition/create-app-composition";

const AppCompositionContext = createContext<AppComposition | null>(null);

export function AppCompositionProvider({
  composition,
  children,
}: PropsWithChildren<{ composition: AppComposition }>) {
  const value = useMemo(() => composition, [composition]);
  return <AppCompositionContext.Provider value={value}>{children}</AppCompositionContext.Provider>;
}

export function useAppComposition() {
  const context = useContext(AppCompositionContext);
  if (!context) {
    throw new Error("useAppComposition must be used inside AppCompositionProvider");
  }
  return context;
}
