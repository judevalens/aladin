import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAppStore } from "@/app/state/store";
import { useCitationNav } from "@/modules/copilot/hooks/use-citation-nav";

const navigate = vi.fn();
vi.mock("react-router-dom", () => ({ useNavigate: () => navigate }));
const original = useAppStore.getState();
const openArtifact = vi.fn();
const openArtifactAt = vi.fn();
const openTicker = vi.fn();

describe("copilot reference navigation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAppStore.setState({ openArtifact, openArtifactAt, openTicker });
  });
  afterEach(() => {
    useAppStore.setState({ openArtifact: original.openArtifact, openArtifactAt: original.openArtifactAt, openTicker: original.openTicker });
  });

  it.each(["page", "shard", "document", "artifact"])("opens a %s in the workspace", (kind) => {
    const { result } = renderHook(() => useCitationNav());
    act(() => result.current({ kind, id: "artifact-1", title: "Research" }));
    expect(openArtifact).toHaveBeenCalledWith("artifact-1");
    expect(navigate).toHaveBeenCalledWith("/folders");
    act(() => result.current({ kind, id: "artifact-1", title: "Research", page: 3 }));
    expect(openArtifactAt).toHaveBeenCalledWith("artifact-1", 3);
  });

  it("keeps entity and ticker navigation unchanged", () => {
    const { result } = renderHook(() => useCitationNav());
    act(() => result.current({ kind: "entity", id: "entity-1", title: "Company" }));
    expect(navigate).toHaveBeenCalledWith("/entity/entity-1");
    act(() => result.current({ kind: "ticker", id: "NVDA", title: "NVDA" }));
    expect(openTicker).toHaveBeenCalledWith("NVDA");
  });
});
