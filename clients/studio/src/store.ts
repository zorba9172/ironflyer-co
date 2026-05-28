import { create } from 'zustand';
import { mockProject, newProjectFromPrompt, type StudioProject } from './studioData';

interface StudioState {
  current: StudioProject;
  /** prompt that started the session, '' when opened from a recent project */
  initialPrompt: string;
  /** gate open in the inspector drawer, null when closed */
  selectedGateId: string | null;
  /** create a project from the home composer and make it current */
  startFromPrompt: (prompt: string) => void;
  /** open an existing (mock) project */
  openProject: (project: StudioProject) => void;
  selectGate: (id: string | null) => void;
}

export const useStudio = create<StudioState>((set) => ({
  current: mockProject,
  initialPrompt: '',
  selectedGateId: null,
  startFromPrompt: (prompt) => set({ initialPrompt: prompt.trim(), current: newProjectFromPrompt(prompt), selectedGateId: null }),
  openProject: (project) => set({ initialPrompt: '', current: project, selectedGateId: null }),
  selectGate: (id) => set({ selectedGateId: id }),
}));
