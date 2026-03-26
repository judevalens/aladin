import { useCallback } from "react";
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCenter,
  forceCollide,
  type SimulationNodeDatum,
  type SimulationLinkDatum,
} from "d3-force";
import type { Node, Edge } from "reactflow";

interface SimNode extends SimulationNodeDatum {
  id: string;
}

interface SimLink extends SimulationLinkDatum<SimNode> {
  id: string;
}

const WIDTH = 900;
const HEIGHT = 700;

export function useForceLayout() {
  const runLayout = useCallback(
    <T>(
      nodes: Node<T>[],
      edges: Edge[],
      onDone: (positions: Record<string, { x: number; y: number }>) => void
    ) => {
      if (nodes.length === 0) return;

      // Split connected vs orphan nodes
      const connectedIds = new Set<string>();
      for (const e of edges) {
        connectedIds.add(e.source);
        connectedIds.add(e.target);
      }

      const connectedNodes = nodes.filter((n) => connectedIds.has(n.id));
      const orphanNodes = nodes.filter((n) => !connectedIds.has(n.id));

      const positions: Record<string, { x: number; y: number }> = {};

      // Force simulation for connected nodes only
      if (connectedNodes.length > 0) {
        const simNodes: SimNode[] = connectedNodes.map((n) => ({
          id: n.id,
          x: n.position.x,
          y: n.position.y,
        }));

        const nodeById = new Map(simNodes.map((n) => [n.id, n]));

        const simLinks: SimLink[] = edges
          .filter((e) => nodeById.has(e.source) && nodeById.has(e.target))
          .map((e) => ({
            id: e.id,
            source: nodeById.get(e.source)!,
            target: nodeById.get(e.target)!,
          }));

        const simulation = forceSimulation<SimNode>(simNodes)
          .force(
            "link",
            forceLink<SimNode, SimLink>(simLinks)
              .id((d) => d.id)
              .distance(280)
              .strength(0.4)
          )
          .force("charge", forceManyBody().strength(-800))
          .force("center", forceCenter(WIDTH / 2, HEIGHT / 2))
          .force("collide", forceCollide(120))
          .stop();

        simulation.tick(300);

        for (const n of simNodes) {
          positions[n.id] = { x: n.x ?? 0, y: n.y ?? 0 };
        }
      }

      // Orphan nodes: stack in a column to the left
      const ORPHAN_X = -220;
      const ORPHAN_START_Y = 60;
      const ORPHAN_GAP = 100;
      orphanNodes.forEach((n, i) => {
        positions[n.id] = { x: ORPHAN_X, y: ORPHAN_START_Y + i * ORPHAN_GAP };
      });

      onDone(positions);
    },
    []
  );

  return runLayout;
}
