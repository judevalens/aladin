import { QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import type { PropsWithChildren } from "react";
import { useState } from "react";
import { AppCompositionProvider } from "@/app/composition/app-composition";
import { createAppComposition } from "@/app/composition/create-app-composition";
import { createQueryClient } from "@/app/query-client";

export function AppProviders({ children }: PropsWithChildren) {
  const [queryClient] = useState(() => createQueryClient());
  const [composition] = useState(() => createAppComposition());

  return (
    <AppCompositionProvider composition={composition}>
      <QueryClientProvider client={queryClient}>
        {children}
        <ReactQueryDevtools initialIsOpen={false} />
      </QueryClientProvider>
    </AppCompositionProvider>
  );
}
