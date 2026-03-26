import React, { useCallback, useEffect, useRef, useState } from "react";
import { useForceLayout } from "../../hooks/useForceLayout";
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  addEdge,
  ReactFlowProvider,
  ConnectionMode,
  type OnConnect,
  type NodeMouseHandler,
  type Node,
  type Edge,
  BackgroundVariant,
} from "reactflow";

import "reactflow/dist/style.css";
import type { Artifact, ArtifactType } from "../../types";
import { fetchGraph, saveGraphNode, saveGraphEdge, updateNodePosition, ingestUrl, deleteGraphNode } from "../../lib/api";
import AudioRecorder from "../AudioRecorder";

const ARTIFACT_NODE_COLORS: Record<ArtifactType, string> = {
  audio: "#f43f5e",
  link: "#3b82f6",
  text: "#f59e0b",
  file: "#10b981",
};

const ARTIFACT_ICONS: Record<ArtifactType, string> = {
  audio: "🎙️",
  link: "🔗",
  text: "📋",
  file: "📎",
};

// ── Custom artifact node ──────────────────────────────────────────────────────


type ContextMenu = { screenX: number; screenY: number } | null;

interface NodeData {
  label: string;
  type?: string;
  content?: string;
  sourceUrl?: string;
}

// ── Canvas context menu ──────────────────────────────────────────────────────

interface CanvasContextMenuProps {
  menu: NonNullable<ContextMenu>;
  onClose: () => void;
  onAddArtifact: (type: ArtifactType, label: string, content: string, sourceUrl?: string) => void;
  onMic: () => void;
}

function CanvasContextMenu({ menu, onClose, onAddArtifact, onMic }: CanvasContextMenuProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleMic = () => {
    onMic();
    onClose();
  };

  const handleLink = async () => {
    const url = prompt("Paste a URL:");
    if (!url) { onClose(); return; }
    onClose();
    try {
      const result = await ingestUrl(url);
      if (result.post) {
        const { authorName, authorHandle, content, platform } = result.post;
        const label = `${authorName} (@${authorHandle}) · ${platform}`;
        onAddArtifact("link", label, content, url);
        return;
      }
    } catch {
      // not a supported URL — fall back to raw
    }
    const label = url.replace(/^https?:\/\//, "").split("/")[0];
    onAddArtifact("link", label, url, url);
  };

  const handlePaste = async () => {
    const text = await navigator.clipboard.readText();
    if (text) {
      onAddArtifact("text", text.slice(0, 40) + (text.length > 40 ? "…" : ""), text);
    }
    onClose();
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      onAddArtifact("file", file.name, file.name);
    }
    onClose();
  };

  const items = [
    { icon: "🎙️", label: "Record audio", onClick: handleMic },
    { icon: "🔗", label: "Add link", onClick: handleLink },
    { icon: "📋", label: "Paste text", onClick: handlePaste },
    { icon: "📎", label: "Upload file", onClick: () => fileInputRef.current?.click() },
  ];

  return (
    <>
      <input ref={fileInputRef} type="file" className="hidden" onChange={handleFileChange} />
      <div
        className="fixed z-50 min-w-[160px] rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
        style={{ top: menu.screenY, left: menu.screenX }}
      >
        {items.map((item) => (
          <button
            key={item.label}
            onClick={item.onClick}
            className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
          >
            <span>{item.icon}</span>
            {item.label}
          </button>
        ))}
      </div>
    </>
  );
}

// ── Node context menu ────────────────────────────────────────────────────────

type NodeContextMenuState = { node: Node<NodeData>; screenX: number; screenY: number } | null;

function NodeContextMenu({
  menu,
  onClose,
  onDelete,
}: {
  menu: NonNullable<NodeContextMenuState>;
  onClose: () => void;
  onDelete: (nodeId: string) => void;
}) {
  const { node } = menu;
  const url = node.data.sourceUrl ?? (node.data.content?.startsWith("http") ? node.data.content : undefined);

  const items = [
    ...(url ? [{
      icon: "↗",
      label: "Go to source",
      danger: false,
      onClick: () => { window.open(url, "_blank", "noopener,noreferrer"); onClose(); },
    }] : []),
    {
      icon: "✕",
      label: "Delete node",
      danger: true,
      onClick: () => { onDelete(node.id); onClose(); },
    },
  ];

  return (
    <div
      className="fixed z-50 min-w-[160px] rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
      style={{ top: menu.screenY, left: menu.screenX }}
    >
      {items.map((item) => (
        <button
          key={item.label}
          onClick={item.onClick}
          className={`flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-gray-50 ${item.danger ? "text-red-500" : "text-gray-700"}`}
        >
          <span className="font-medium">{item.icon}</span>
          {item.label}
        </button>
      ))}
    </div>
  );
}

// ── Graph inner ──────────────────────────────────────────────────────────────

interface GraphContentInnerProps {
  onAddArtifact: (artifact: Artifact) => void;
  artifactToAdd: Artifact | null;
  onArtifactNodeAdded: () => void;
  onNodeSelect: (nodeId: string) => void;
}

function GraphContentInner({ onAddArtifact, artifactToAdd, onArtifactNodeAdded, onNodeSelect }: GraphContentInnerProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<NodeData>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [contextMenu, setContextMenu] = useState<ContextMenu>(null);
  const [nodeContextMenu, setNodeContextMenu] = useState<NodeContextMenuState>(null);
  const [showRecorder, setShowRecorder] = useState(false);
  const runLayout = useForceLayout();

  const applyLayout = useCallback(
    (currentNodes: typeof nodes, currentEdges: typeof edges) => {
      runLayout(currentNodes, currentEdges, (positions) => {
        setNodes((nds) =>
          nds.map((n) =>
            positions[n.id] ? { ...n, position: positions[n.id] } : n
          )
        );
      });
    },
    [runLayout, setNodes]
  );

  // Load graph from Neo4j on mount
  useEffect(() => {
    fetchGraph().then(({ nodes: apiNodes, edges: apiEdges }) => {
      const loadedNodes = apiNodes.map((n) => ({
        id: n.id,
        data: { label: n.label },
        position: { x: n.x, y: n.y },
      }));

      const loadedEdges = apiEdges
        .filter((e, i, arr) => arr.findIndex((x) => x.id === e.id) === i)
        .map((e) => ({
          id: e.id,
          source: e.source,
          target: e.target,
          type: "straight",
          style: { stroke: "#94a3b8" },
        }));

      setNodes(loadedNodes);
      setEdges(loadedEdges);

      // Auto-layout on load when there are multiple nodes
      if (loadedNodes.length > 1) {
        applyLayout(loadedNodes, loadedEdges);
      }
    }).catch(console.error);
  }, []);

  const onNodeDragStop = useCallback((_event: unknown, node: Node<NodeData>) => {
    updateNodePosition(node.id, node.position.x, node.position.y).catch(console.error);
  }, []);

  const highlightNodeEdges = useCallback((nodeId: string | null) => {
    setEdges((eds) => eds.map((e) => {
      const active = nodeId !== null && (e.source === nodeId || e.target === nodeId);
      return { ...e, style: { stroke: active ? "#1e293b" : "#94a3b8", strokeWidth: active ? 2 : 1 } };
    }));
  }, [setEdges]);

  const onNodeClick: NodeMouseHandler = useCallback((_event, node) => {
    onNodeSelect(node.id);
    highlightNodeEdges(node.id);
    setContextMenu(null);
    setNodeContextMenu(null);
  }, [onNodeSelect, highlightNodeEdges]);

  const onEdgeClick = useCallback((_event: React.MouseEvent, edge: Edge) => {
    setEdges((eds) => eds.map((e) => ({
      ...e,
      style: { stroke: e.id === edge.id ? "#1e293b" : "#94a3b8", strokeWidth: e.id === edge.id ? 2.5 : 1 },
    })));
  }, [setEdges]);

  const handleNodeContextMenu: NodeMouseHandler = useCallback((e, node) => {
    e.preventDefault();
    setNodeContextMenu({ node, screenX: e.clientX, screenY: e.clientY });
    setContextMenu(null);
  }, []);

  const onConnect: OnConnect = useCallback(
    (connection) => {
      setEdges((eds) => {
        const exists = eds.some(
          (e) => e.source === connection.source && e.target === connection.target
        );
        if (exists) return eds;
        const edge: Edge = {
          ...connection,
          id: `e-${connection.source}-${connection.target}`,
          type: "straight",
          style: { stroke: "#94a3b8" },
        } as Edge;
        saveGraphEdge({ id: edge.id, source: edge.source!, target: edge.target! }).catch(console.error);
        return addEdge(edge, eds);
      });
    },
    [setEdges]
  );

  const handleDeleteNode = useCallback((nodeId: string) => {
    setNodes((nds) => nds.filter((n) => n.id !== nodeId));
    setEdges((eds) => eds.filter((e) => e.source !== nodeId && e.target !== nodeId));
    deleteGraphNode(nodeId).catch(console.error);
  }, [setNodes, setEdges]);

  const onPaneClick = useCallback(() => {
    setContextMenu(null);
    setNodeContextMenu(null);
    highlightNodeEdges(null);
  }, [highlightNodeEdges]);

  const onPaneContextMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setContextMenu({ screenX: e.clientX, screenY: e.clientY });
  }, []);

  const handleAddArtifact = useCallback(
    (type: ArtifactType, label: string, content: string, sourceUrl?: string) => {
      const artifact: Artifact = {
        id: `artifact-${Date.now()}`,
        type,
        label,
        content,
        sourceUrl,
        createdAt: new Date(),
      };
      onAddArtifact(artifact);
    },
    [onAddArtifact]
  );

  // Promote artifact from collection to graph
  useEffect(() => {
    if (!artifactToAdd) return;
    const color = ARTIFACT_NODE_COLORS[artifactToAdd.type];
    const position = { x: Math.random() * 400 + 100, y: Math.random() * 400 + 100 };
    const label = `${ARTIFACT_ICONS[artifactToAdd.type]} ${artifactToAdd.label}`;
    const newNode: Node<NodeData> = {
      id: artifactToAdd.id,
      data: { label, type: artifactToAdd.type, content: artifactToAdd.content, sourceUrl: artifactToAdd.sourceUrl },
      position,
    };
    setNodes((nds) => [...nds.filter((n) => n.id !== newNode.id), newNode]);
    saveGraphNode({
      id: artifactToAdd.id,
      label,
      type: artifactToAdd.type,
      content: artifactToAdd.content,
      sourceUrl: artifactToAdd.sourceUrl,
      x: position.x,
      y: position.y,
      color,
    }).catch(console.error);
    onArtifactNodeAdded();
  }, [artifactToAdd]);

  return (
    <div className="h-full w-full relative">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onNodeDragStop={onNodeDragStop}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        onNodeClick={onNodeClick}
        onEdgeClick={onEdgeClick}
        onNodeContextMenu={handleNodeContextMenu}
        onPaneClick={onPaneClick}
        onPaneContextMenu={onPaneContextMenu}
        connectionMode={ConnectionMode.Loose}
        fitView
        attributionPosition="bottom-right"
      >
        <Controls className="!bg-white !border-gray-200 !shadow-sm">
          <button
            onClick={() => applyLayout(nodes, edges)}
            title="Re-layout"
            className="react-flow__controls-button"
            style={{ fontSize: 14 }}
          >
            ⟳
          </button>
        </Controls>
        <MiniMap
          className="!bg-gray-50 !border-gray-200"
          nodeColor={(node) => (ARTIFACT_NODE_COLORS as Record<string, string>)[(node.data as NodeData).type ?? ""] ?? "#64748b"}
        />
        <Background variant={BackgroundVariant.Dots} gap={12} size={1} color="#e5e7eb" />
      </ReactFlow>

      {nodeContextMenu && (
        <NodeContextMenu
          menu={nodeContextMenu}
          onClose={() => setNodeContextMenu(null)}
          onDelete={handleDeleteNode}
        />
      )}

      {contextMenu && (
        <CanvasContextMenu
          menu={contextMenu}
          onClose={() => setContextMenu(null)}
          onAddArtifact={handleAddArtifact}
          onMic={() => setShowRecorder(true)}
        />
      )}

      {showRecorder && (
        <AudioRecorder
          onSave={(filename, label) => {
            handleAddArtifact("audio", label, `/api/audio/${filename}`);
            setShowRecorder(false);
          }}
          onCancel={() => setShowRecorder(false)}
        />
      )}
    </div>
  );
}

// ── Export ───────────────────────────────────────────────────────────────────

interface GraphContentProps {
  onAddArtifact: (artifact: Artifact) => void;
  artifactToAdd: Artifact | null;
  onArtifactNodeAdded: () => void;
  onNodeSelect: (nodeId: string) => void;
}

export default function GraphContent(props: GraphContentProps) {
  return (
    <ReactFlowProvider>
      <GraphContentInner {...props} />
    </ReactFlowProvider>
  );
}
