import { type Observable } from "rxjs";
import type { ArtifactRepo } from "@/repos/artifacts/artifact-repo";
import type { WorkspaceRepo } from "@/repos/workspace/workspace-repo";
import type {
  ArtifactKind,
  Artifact,
  BrowserTreeNode,
} from "@/shared/api/models";
import { KeyedStream } from "@/shared/flow/keyed-stream";
import { LazyResultStream } from "@/shared/flow/lazy-result-stream";
import { type Result } from "@/shared/flow/result";
import type { BrowserNodeRow } from "@/repos/local-repo-types";

export class WorkspaceSyncService {
  private currentTree: BrowserTreeNode[] | null = null;
  private readonly treeStream = new LazyResultStream<BrowserTreeNode[]>(() =>
    this.workspaceRepo.getBrowserTree().then((tree) => {
      this.currentTree = tree;
      return tree;
    }),
  );
  private readonly artifactStream = new KeyedStream<string, Artifact>(
    (artifact) => artifact.id,
    (id) => this.artifactRepo.getArtifact(id),
  );

  constructor(
    private readonly workspaceRepo: WorkspaceRepo,
    private readonly artifactRepo: ArtifactRepo,
  ) {}

  tree(): Observable<Result<BrowserTreeNode[]>> {
    return this.treeStream.observable();
  }

  async publishLocalTree() {
    const tree = await this.workspaceRepo.getBrowserTree({ policy: "local-first" });
    this.pushTree(tree);
  }

  async refreshTree() {
    const tree = await this.workspaceRepo.getBrowserTree({ policy: "remote" });
    this.pushTree(tree);
  }

  artifactById(artifactId: string): Observable<Result<Artifact>> {
    return this.artifactStream.observe(artifactId);
  }

  publishArtifact(artifact: Artifact) {
    this.artifactStream.push(artifact);
    if (this.currentTree) {
      this.pushTree(patchArtifactPreview(this.currentTree, artifact));
    }
  }

  publishArtifacts(artifacts: Artifact[]) {
    artifacts.forEach((artifact) => this.artifactStream.push(artifact));
  }

  handleBrowserNodeCreated(node: BrowserNodeRow) {
    if (!this.currentTree) {
      void this.publishLocalTree();
      return;
    }
    this.pushTree(appendTreeNode(this.currentTree, node));
  }

  handleBrowserNodeUpdated(node: BrowserNodeRow) {
    if (!this.currentTree) {
      void this.publishLocalTree();
      return;
    }
    this.pushTree(patchTreeNode(this.currentTree, node));
  }

  handleBrowserNodeDeleted(id: string) {
    if (!this.currentTree) {
      void this.publishLocalTree();
      return;
    }
    this.pushTree(removeTreeNode(this.currentTree, id));
  }

  handleArtifactDeleted(id: string) {
    if (!this.currentTree) {
      void this.publishLocalTree();
      return;
    }
    this.pushTree(removeTreeNode(this.currentTree, id));
  }

  private pushTree(tree: BrowserTreeNode[]) {
    this.currentTree = tree;
    this.treeStream.push(tree);
  }
}

function removeTreeNode(tree: BrowserTreeNode[], id: string): BrowserTreeNode[] {
  return tree
    .filter((node) => node.id !== id && node.artifactId !== id)
    .map((node) => {
      if (node.children.length === 0) return node;
      return {
        ...node,
        children: removeTreeNode(node.children, id),
      };
    });
}

function appendTreeNode(
  tree: BrowserTreeNode[],
  node: BrowserNodeRow,
): BrowserTreeNode[] {
  if (containsTreeNode(tree, node.id, node.artifactId ?? null)) {
    return patchTreeNode(tree, node);
  }

  const nextNode = toBrowserTreeNode(node);
  if (!node.parentId) {
    return [...tree, nextNode];
  }

  const appended = appendIntoChildren(tree, node.parentId, nextNode);
  return appended ?? [...tree, nextNode];
}

function patchTreeNode(
  tree: BrowserTreeNode[],
  node: BrowserNodeRow,
): BrowserTreeNode[] {
  return tree.map((current) => {
    const isArtifactTarget =
      node.kind === "artifact" &&
      (current.id === node.id || current.artifactId === (node.artifactId ?? node.id));
    const isFolderTarget = node.kind === "folder" && current.kind === "folder" && current.id === node.id;

    if (isFolderTarget) {
      return {
        ...current,
        parentId: node.parentId ?? null,
        title: node.title,
      };
    }

    if (isArtifactTarget) {
      return {
        ...current,
        parentId: node.parentId ?? null,
        title: node.title,
        artifactId: node.artifactId ?? node.id,
      };
    }

    if (current.children.length === 0) return current;
    return {
      ...current,
      children: patchTreeNode(current.children, node),
    };
  });
}

function appendIntoChildren(
  tree: BrowserTreeNode[],
  parentId: string,
  node: BrowserTreeNode,
): BrowserTreeNode[] | null {
  let changed = false;
  const next = tree.map((current) => {
    if (current.kind === "folder" && current.id === parentId) {
      changed = true;
      return {
        ...current,
        children: [...current.children, node],
      };
    }
    if (current.children.length === 0) return current;
    const updatedChildren = appendIntoChildren(current.children, parentId, node);
    if (!updatedChildren) return current;
    changed = true;
    return {
      ...current,
      children: updatedChildren,
    };
  });

  return changed ? next : null;
}

function containsTreeNode(
  tree: BrowserTreeNode[],
  id: string,
  artifactId: string | null,
): boolean {
  for (const node of tree) {
    if (node.id === id || (artifactId && node.artifactId === artifactId)) {
      return true;
    }
    if (containsTreeNode(node.children, id, artifactId)) {
      return true;
    }
  }
  return false;
}

function toBrowserTreeNode(node: BrowserNodeRow): BrowserTreeNode {
  if (node.kind === "artifact") {
    return {
      id: node.id,
      parentId: node.parentId ?? null,
      kind: "artifact",
      title: node.title,
      artifactId: node.artifactId ?? node.id,
      artifactPreview: null,
      children: [],
    };
  }

  return {
    id: node.id,
    parentId: node.parentId ?? null,
    kind: "folder",
    title: node.title,
    children: [],
  };
}

function patchArtifactPreview(
  tree: BrowserTreeNode[],
  artifact: Artifact,
): BrowserTreeNode[] {
  return tree.map((node) => {
    const isArtifactTarget =
      node.kind === "artifact" &&
      (node.id === artifact.id || node.artifactId === artifact.id);

    if (isArtifactTarget) {
      return {
        ...node,
        title: artifact.title,
        artifactId: artifact.id,
        artifactPreview: {
          id: artifact.id,
          title: artifact.title,
          kind: artifact.kind as ArtifactKind,
          updatedLabel: artifact.updatedLabel ?? null,
        },
      };
    }

    if (node.children.length === 0) return node;
    return {
      ...node,
      children: patchArtifactPreview(node.children, artifact),
    };
  });
}
