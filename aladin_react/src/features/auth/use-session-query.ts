import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";

export const sessionQueryKey = ["auth", "session"];

export function useSessionQuery() {
  return useQuery({
    queryKey: sessionQueryKey,
    queryFn: async () => {
      const response = await api.me();
      return response.user;
    },
    retry: false,
  });
}
