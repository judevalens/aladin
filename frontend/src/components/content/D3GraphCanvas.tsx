import { useEffect, useRef, useCallback, useState } from "react";
import * as d3 from "d3-selection";
import * as zoom from "d3-zoom";
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCenter,
  forceCollide,
  type SimulationNodeDatum,
  type SimulationLinkDatum,
} from "d3-force";
import { fetchFullGraph, updateNodePosition, saveGraphNode, deleteGraphNode, saveGraphEdge } from "../../lib/api";
import type { Artifact, ArtifactType } from "../../types";

// ── Types ─────────────────────────────────────────────────────────────────────

interface GraphNode extends SimulationNodeDatum {
  id: string;
  label: string;
  type: string;
  content?: string;
  sourceUrl?: string;
  color: string;
  radius: number;
  pipeline?: boolean;  // true = promoted by pipeline (not user-placed)
}

interface GraphLink extends SimulationLinkDatum<GraphNode> {
  id: string;
}

// ── Constants ─────────────────────────────────────────────────────────────────

const ARTIFACT_COLORS: Record<ArtifactType, string> = {
  audio: "#94a3b8",
  link: "#94a3b8",
  text: "#94a3b8",
  file: "#94a3b8",
  feed: "#10b981",
  post: "#94a3b8",
};

const ARTIFACT_ICONS: Record<ArtifactType, string> = {
  audio: "🎙️",
  link: "🔗",
  text: "📋",
  file: "📎",
  feed: "📡",
  post: "💬",
};

const BASE_RADIUS = 4;
const BG_COLOR = "#ffffff";
const EDGE_COLOR = "rgba(0,0,0,0.1)";
const LABEL_COLOR = "#9ca3af";
const LABEL_FOCUS = "#6b7280";
const TAU = Math.PI * 2;

// ── Props ─────────────────────────────────────────────────────────────────────

interface Props {
  onNodeSelect: (nodeId: string) => void;
  artifactToAdd: Artifact | null;
  onArtifactNodeAdded: () => void;
  onAddArtifact: (artifact: Artifact) => void;
}

// ── Component ─────────────────────────────────────────────────────────────────

export default function D3GraphCanvas({ onNodeSelect, artifactToAdd, onArtifactNodeAdded }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Mutable refs so simulation callbacks always see latest data
  const nodesRef = useRef<GraphNode[]>([]);
  const linksRef = useRef<GraphLink[]>([]);
  const transformRef = useRef(zoom.zoomIdentity);
  const hoveredRef = useRef<string | null>(null);
  const selectedRef = useRef<string | null>(null);
  const simRef = useRef<ReturnType<typeof forceSimulation<GraphNode>> | null>(null);
  const rafRef = useRef<number>(0);
  const edgePreviewRef = useRef<{ source: GraphNode; tx: number; ty: number } | null>(null);

  // ── Draw ──────────────────────────────────────────────────────────────────

  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const { width, height } = canvas;
    const t = transformRef.current;
    const selected = selectedRef.current;
    const nodes = nodesRef.current;
    const links = linksRef.current;

    // ── Background ─────────────────────────────────────────────────────────
    ctx.clearRect(0, 0, width, height);
    ctx.fillStyle = BG_COLOR;
    ctx.fillRect(0, 0, width, height);

    ctx.save();
    ctx.translate(t.x, t.y);
    ctx.scale(t.k, t.k);

    // ── Edges ───────────────────────────────────────────────────────────────
    ctx.strokeStyle = EDGE_COLOR;
    ctx.lineWidth = 1 / t.k;
    for (const l of links) {
      const s = l.source as GraphNode;
      const tgt = l.target as GraphNode;
      if (s.x == null || tgt.x == null) continue;

      ctx.beginPath();
      ctx.moveTo(s.x!, s.y!);
      ctx.lineTo(tgt.x!, tgt.y!);
      ctx.stroke();
    }

    // ── Edge preview (shift-drag in progress) ──────────────────────────────
    const preview = edgePreviewRef.current;
    if (preview?.source.x != null) {
      const targetNode = nodesRef.current.find((n) => n.id === hoveredRef.current);
      const snapX = (targetNode && targetNode !== preview.source) ? targetNode.x! : preview.tx;
      const snapY = (targetNode && targetNode !== preview.source) ? targetNode.y! : preview.ty;
      ctx.globalAlpha = 0.6;
      ctx.strokeStyle = "#94a3b8";
      ctx.lineWidth = 1 / t.k;
      ctx.setLineDash([5 / t.k, 4 / t.k]);
      ctx.beginPath();
      ctx.moveTo(preview.source.x!, preview.source.y!);
      ctx.lineTo(snapX, snapY);
      ctx.stroke();
      ctx.setLineDash([]);
    }

    // ── Nodes ────────────────────────────────────────────────────────────────
    for (const n of nodes) {
      if (n.x == null || n.y == null) continue;
      const isSelected = n.id === selected;
      const baseColor  = n.color ?? "#94a3b8";

      ctx.globalAlpha = n.pipeline ? 0.75 : 1.0;

      ctx.beginPath();
      ctx.arc(n.x, n.y, n.radius, 0, TAU);
      ctx.fillStyle = isSelected ? "#1e293b" : baseColor;
      ctx.fill();

      // Pipeline entity/topic nodes: hollow ring style
      if (n.pipeline && !isSelected) {
        ctx.beginPath();
        ctx.arc(n.x, n.y, n.radius, 0, TAU);
        ctx.strokeStyle = baseColor;
        ctx.lineWidth   = 1.5 / t.k;
        ctx.globalAlpha = 0.4;
        ctx.fillStyle   = baseColor + "33";  // 20% opacity fill
        ctx.fill();
        ctx.stroke();
        ctx.globalAlpha = 0.75;
      }

      if (isSelected) {
        ctx.beginPath();
        ctx.arc(n.x, n.y, n.radius + 3 / t.k, 0, TAU);
        ctx.strokeStyle = baseColor;
        ctx.lineWidth   = 1.5 / t.k;
        ctx.globalAlpha = 0.8;
        ctx.stroke();
      }

      ctx.globalAlpha = 1.0;
    }

    ctx.restore();

    // ── Labels — screen space so font stays fixed-size ───────────────────────
    const fontSize = 10;
    ctx.font = `400 ${fontSize}px Inter, ui-sans-serif, sans-serif`;
    ctx.textAlign = "center";
    ctx.textBaseline = "top";

    for (const n of nodes) {
      if (n.x == null || n.y == null) continue;

      const sx = n.x * t.k + t.x;
      const sy = n.y * t.k + t.y;

      // Cull off-screen
      if (sx < -120 || sx > width + 120 || sy < -40 || sy > height + 40) continue;

      const screenR = n.radius * t.k;
      const isSelected = n.id === selected;
      ctx.fillStyle = isSelected ? LABEL_FOCUS : LABEL_COLOR;

      const maxLen = 28;
      const label = n.label.length > maxLen ? n.label.slice(0, maxLen - 1) + "…" : n.label;
      ctx.fillText(label, sx, sy + screenR + 4);
    }

    ctx.textBaseline = "alphabetic";
  }, []);

  const tick = useCallback(() => {
    draw();
    rafRef.current = requestAnimationFrame(tick);
  }, [draw]);

  // ── Init ──────────────────────────────────────────────────────────────────

  useEffect(() => {
    const container = containerRef.current;
    const canvas = canvasRef.current;
    if (!container || !canvas) return;

    // Size canvas
    const resize = () => {
      canvas.width = container.clientWidth;
      canvas.height = container.clientHeight;
    };
    resize();
    const ro = new ResizeObserver(resize);
    ro.observe(container);

    // ── Hit test helper ─────────────────────────────────────────────────────
    function getNodeAt(offsetX: number, offsetY: number): GraphNode | null {
      const t = transformRef.current;
      const mx = (offsetX - t.x) / t.k;
      const my = (offsetY - t.y) / t.k;
      return nodesRef.current.find((n) => {
        const dx = (n.x ?? 0) - mx;
        const dy = (n.y ?? 0) - my;
        return Math.sqrt(dx * dx + dy * dy) < n.radius + 4;
      }) ?? null;
    }

    // Load data — full graph (manual + pipeline-promoted nodes)
    fetchFullGraph().then(({ nodes: apiNodes, edges: apiEdges }) => {
      // Deduplicate nodes by id
      const seenIds = new Set<string>();
      const uniqueNodes = apiNodes.filter((n) => {
        if (seenIds.has(n.id)) return false;
        seenIds.add(n.id);
        return true;
      });

      // Compute degree for radius
      const degree: Record<string, number> = {};
      for (const e of apiEdges) {
        degree[e.source] = (degree[e.source] ?? 0) + 1;
        degree[e.target] = (degree[e.target] ?? 0) + 1;
      }

      nodesRef.current = uniqueNodes.map((n) => ({
        id:        n.id,
        label:     n.label,
        type:      n.type,
        content:   n.content,
        sourceUrl: n.sourceUrl,
        color:     n.color ?? "#94a3b8",
        pipeline:  !!(n as { pipeline?: boolean }).pipeline,
        radius:    BASE_RADIUS + Math.sqrt(degree[n.id] ?? 0) * 2,
        x:         n.x || Math.random() * 800,
        y:         n.y || Math.random() * 600,
      }));

      const nodeMap = new Map(nodesRef.current.map((n) => [n.id, n]));

      linksRef.current = apiEdges
        .filter((e, i, arr) => arr.findIndex((x) => x.id === e.id) === i)
        .filter((e) => nodeMap.has(e.source) && nodeMap.has(e.target))
        .map((e) => ({
          id: e.id,
          source: nodeMap.get(e.source)!,
          target: nodeMap.get(e.target)!,
        }));

      // Simulation
      simRef.current = forceSimulation<GraphNode>(nodesRef.current)
        .force("link", forceLink<GraphNode, GraphLink>(linksRef.current).id((d) => d.id).distance(80).strength(0.7))
        .force("charge", forceManyBody().strength(-400))
        .force("center", forceCenter(canvas.width / 2, canvas.height / 2).strength(0.05))
        .force("collide", forceCollide<GraphNode>((d) => d.radius + 6))
        .alphaDecay(0.02)
        .on("tick", draw);

      // Start render loop
      rafRef.current = requestAnimationFrame(tick);
    }).catch(console.error);

    // ── Zoom / Pan ──────────────────────────────────────────────────────────
    // Filter out zoom activation when the pointer is on a node (let drag handle those)
    const zoomBehavior = zoom.zoom<HTMLCanvasElement, unknown>()
      .scaleExtent([0.1, 4])
      .filter((event: Event) => {
        if (edgePreviewRef.current) return false;
        if (event.type === "mousedown") {
          const me = event as MouseEvent;
          if (getNodeAt(me.offsetX, me.offsetY)) return false;
        }
        return !(event as MouseEvent).button;
      })
      .on("zoom", (event) => {
        transformRef.current = event.transform;
      });

    const sel = d3.select(canvas);
    sel.call(zoomBehavior);

    // ── Drag + edge-drawing (manual mouse events to avoid zoom conflict) ─────
    let dragNode: GraphNode | null = null;

    const onMouseDown = (event: MouseEvent) => {
      if (event.button !== 0) return;
      const node = getNodeAt(event.offsetX, event.offsetY);
      if (!node) return;

      if (event.shiftKey) {
        // Start drawing a new edge
        const t = transformRef.current;
        edgePreviewRef.current = {
          source: node,
          tx: (event.offsetX - t.x) / t.k,
          ty: (event.offsetY - t.y) / t.k,
        };
        canvas.style.cursor = "crosshair";
        return;
      }

      // Normal drag
      if (!simRef.current) return;
      dragNode = node;
      simRef.current.alphaTarget(0.2).restart();
      dragNode.fx = dragNode.x;
      dragNode.fy = dragNode.y;
    };

    const onMouseMove = (event: MouseEvent) => {
      const t = transformRef.current;
      if (edgePreviewRef.current) {
        edgePreviewRef.current = {
          ...edgePreviewRef.current,
          tx: (event.offsetX - t.x) / t.k,
          ty: (event.offsetY - t.y) / t.k,
        };
        const n = getNodeAt(event.offsetX, event.offsetY);
        hoveredRef.current = n?.id ?? null;
        canvas.style.cursor = "crosshair";
        return;
      }
      if (dragNode) {
        dragNode.fx = (event.offsetX - t.x) / t.k;
        dragNode.fy = (event.offsetY - t.y) / t.k;
        canvas.style.cursor = "grabbing";
      } else {
        const n = getNodeAt(event.offsetX, event.offsetY);
        hoveredRef.current = n?.id ?? null;
        canvas.style.cursor = n ? "pointer" : "default";
      }
    };

    const onMouseUp = (event: MouseEvent) => {
      // Finish edge drawing
      if (edgePreviewRef.current) {
        const { source } = edgePreviewRef.current;
        edgePreviewRef.current = null;
        const target = getNodeAt(event.offsetX, event.offsetY);
        if (target && target.id !== source.id && simRef.current) {
          const edgeId = crypto.randomUUID();
          const newLink: GraphLink = { id: edgeId, source, target };
          linksRef.current = [...linksRef.current, newLink];
          (simRef.current.force("link") as ReturnType<typeof forceLink>).links(linksRef.current);
          simRef.current.alpha(0.3).restart();
          saveGraphEdge({ id: edgeId, source: source.id, target: target.id }).catch(console.error);
        }
        canvas.style.cursor = "default";
        return;
      }
      // Finish drag
      if (!dragNode || !simRef.current) { dragNode = null; return; }
      simRef.current.alphaTarget(0);
      // Only persist position for user-placed (non-pipeline) nodes
      if (!dragNode.pipeline) {
        updateNodePosition(dragNode.id, dragNode.fx!, dragNode.fy!).catch(console.error);
      }
      dragNode.fx = null;
      dragNode.fy = null;
      dragNode = null;
      canvas.style.cursor = "default";
    };

    const onMouseLeave = () => {
      hoveredRef.current = null;
      edgePreviewRef.current = null;
      if (dragNode) onMouseUp(new MouseEvent("mouseup"));
    };

    const onClick = (event: MouseEvent) => {
      if (dragNode) return;
      const n = getNodeAt(event.offsetX, event.offsetY);
      if (n) {
        selectedRef.current = n.id;
        onNodeSelect(n.id);
      } else {
        selectedRef.current = null;
      }
    };

    canvas.addEventListener("mousedown", onMouseDown);
    canvas.addEventListener("mousemove", onMouseMove);
    canvas.addEventListener("mouseup", onMouseUp);
    canvas.addEventListener("mouseleave", onMouseLeave);
    canvas.addEventListener("click", onClick);

    return () => {
      ro.disconnect();
      cancelAnimationFrame(rafRef.current);
      simRef.current?.stop();
      canvas.removeEventListener("mousedown", onMouseDown);
      canvas.removeEventListener("mousemove", onMouseMove);
      canvas.removeEventListener("mouseup", onMouseUp);
      canvas.removeEventListener("mouseleave", onMouseLeave);
      canvas.removeEventListener("click", onClick);
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Add artifact node ─────────────────────────────────────────────────────
  useEffect(() => {
    if (!artifactToAdd) return;
    const color = (ARTIFACT_COLORS as Record<string, string>)[artifactToAdd.type] ?? "#64748b";
    const icon = (ARTIFACT_ICONS as Record<string, string>)[artifactToAdd.type] ?? "";
    const label = `${icon} ${artifactToAdd.label}`;
    const canvas = canvasRef.current;
    const cx = canvas ? canvas.width / 2 : 400;
    const cy = canvas ? canvas.height / 2 : 300;

    const newNode: GraphNode = {
      id: artifactToAdd.id,
      label,
      type: artifactToAdd.type,
      content: artifactToAdd.content,
      sourceUrl: artifactToAdd.sourceUrl,
      color,
      radius: BASE_RADIUS,
      x: cx + (Math.random() - 0.5) * 200,
      y: cy + (Math.random() - 0.5) * 200,
    };

    nodesRef.current = [...nodesRef.current.filter((n) => n.id !== newNode.id), newNode];

    if (simRef.current) {
      simRef.current.nodes(nodesRef.current).alpha(0.3).restart();
    }

    saveGraphNode({
      id: artifactToAdd.id,
      label,
      type: artifactToAdd.type,
      content: artifactToAdd.content ?? "",
      sourceUrl: artifactToAdd.sourceUrl,
      x: newNode.x!,
      y: newNode.y!,
      color,
    }).catch(console.error);

    onArtifactNodeAdded();
  }, [artifactToAdd]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Context menu (right-click delete) ─────────────────────────────────────
  const menuRef = useRef<{ x: number; y: number; node: GraphNode } | null>(null);

  const handleContextMenu = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
    e.preventDefault();
    const t = transformRef.current;
    const mx = (e.nativeEvent.offsetX - t.x) / t.k;
    const my = (e.nativeEvent.offsetY - t.y) / t.k;
    const n = nodesRef.current.find((node) => {
      const dx = (node.x ?? 0) - mx;
      const dy = (node.y ?? 0) - my;
      return Math.sqrt(dx * dx + dy * dy) < node.radius + 4;
    });
    if (n) {
      menuRef.current = { x: e.clientX, y: e.clientY, node: n };
      // Force re-render for menu
      setMenu({ x: e.clientX, y: e.clientY, node: n });
    }
  }, []);

  const [menu, setMenu] = useState<{ x: number; y: number; node: GraphNode } | null>(null);

  const handleDeleteNode = useCallback((nodeId: string) => {
    nodesRef.current = nodesRef.current.filter((n) => n.id !== nodeId);
    linksRef.current = linksRef.current.filter(
      (l) => (l.source as GraphNode).id !== nodeId && (l.target as GraphNode).id !== nodeId
    );
    if (simRef.current) simRef.current.nodes(nodesRef.current).alpha(0.1).restart();
    deleteGraphNode(nodeId).catch(console.error);
    setMenu(null);
  }, []);

  return (
    <div ref={containerRef} className="w-full h-full relative" onClick={() => setMenu(null)}>
      <canvas
        ref={canvasRef}
        className="w-full h-full"
        onContextMenu={handleContextMenu}
      />

      {menu && (
        <div
          className="fixed z-50 min-w-[150px] rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
          style={{ top: menu.y, left: menu.x }}
          onClick={(e) => e.stopPropagation()}
        >
          {/* Node type label */}
          <div className="px-3 py-1.5 border-b border-gray-100">
            <p className="text-[10px] text-gray-400 uppercase tracking-wide">{menu.node.type}</p>
            <p className="text-xs font-medium text-gray-700 truncate max-w-[180px]">{menu.node.label}</p>
          </div>

          {menu.node.sourceUrl && (
            <button
              onClick={() => { window.open(menu.node.sourceUrl, "_blank", "noopener,noreferrer"); setMenu(null); }}
              className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
            >
              <span>↗</span> Go to source
            </button>
          )}
          {/* Only allow deleting user-placed nodes */}
          {!menu.node.pipeline && (
            <button
              onClick={() => handleDeleteNode(menu.node.id)}
              className="flex w-full items-center gap-2 px-3 py-2 text-sm text-red-500 hover:bg-gray-50"
            >
              <span>✕</span> Delete node
            </button>
          )}
        </div>
      )}
    </div>
  );
}

