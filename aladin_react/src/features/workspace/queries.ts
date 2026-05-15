import { useQuery } from "@tanstack/react-query";
import { api, toArtifact, toBrowserTreeNode } from "@/shared/api/client";

export const browserTreeQueryKey = ["browser-tree"];

export function useBrowserTreeQuery() {
  return useQuery({
    queryKey: browserTreeQueryKey,
    queryFn: async () => {
      const tree = await api.getBrowserTree();
      return tree.map(toBrowserTreeNode);
    },
  });
}

export function useArtifactQuery(artifactId: string | null) {
  return useQuery({
    queryKey: ["artifact", artifactId],
    enabled: Boolean(artifactId),
    queryFn: async () => {
      const artifact = await api.getUserArtifact(artifactId!);
      return toArtifact(artifact);
    },
  });
}
