import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import type { PropsWithChildren } from "react";
import { useState } from "react";
import { AppCompositionContext } from "@/app/composition/app-composition";
import { createAppComposition } from "@/app/composition/create-app-composition";
import { createQueryClient } from "@/app/query-client";
import { ThemeEffect } from "@/app/theme-effect";
import { readStoredTheme } from "@/app/state/theme-slice";

// Apply the stored theme synchronously, before first paint, to avoid a flash.
if (typeof document !== "undefined") {
  document.documentElement.dataset.theme = readStoredTheme();
}

export function AppProviders({ children }: PropsWithChildren) {
  const [queryClient] = useState(() => createQueryClient());
  const [composition] = useState(() => createAppComposition());
  return (
    <AppCompositionContext.Provider value={composition}>
      <QueryClientProvider client={queryClient}>
        <ThemeEffect />
        {children}
      </QueryClientProvider>
    </AppCompositionContext.Provider>
  );
}
