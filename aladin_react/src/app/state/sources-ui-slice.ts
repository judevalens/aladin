import type { StateCreator } from "zustand";

export interface SourcesUiState {
  addOpen: boolean;
  integrationsOpen: boolean;
  selectedSourceId: string | null;
}

export interface SourcesUiSlice {
  sourcesUi: SourcesUiState;
  setAddSourceOpen: (open: boolean) => void;
  setIntegrationsOpen: (open: boolean) => void;
  setSelectedSourceId: (sourceId: string | null) => void;
}

const initialSourcesUiState: SourcesUiState = {
  addOpen: false,
  integrationsOpen: false,
  selectedSourceId: null,
};

export const createSourcesUiSlice: StateCreator<SourcesUiSlice, [], [], SourcesUiSlice> = (set) => ({
  sourcesUi: initialSourcesUiState,
  setAddSourceOpen: (open) =>
    set((state) => ({
      sourcesUi: {
        ...state.sourcesUi,
        addOpen: open,
      },
    })),
  setIntegrationsOpen: (open) =>
    set((state) => ({
      sourcesUi: {
        ...state.sourcesUi,
        integrationsOpen: open,
      },
    })),
  setSelectedSourceId: (sourceId) =>
    set((state) => ({
      sourcesUi: {
        ...state.sourcesUi,
        selectedSourceId: sourceId,
      },
    })),
});
