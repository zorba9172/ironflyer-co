import { create } from 'zustand';
import { mockProject, newProjectFromPrompt, type StudioProject } from './studioData';

interface StudioState {
  current: StudioProject;
  /** prompt that started the session, '' when opened from a recent project */
  initialPrompt: string;
  /** create a project from the home composer and make it current */
  startFromPrompt: (prompt: string) => void;
  /** open an existing (mock) project */
  openProject: (project: StudioProject) => void;
}

export const useStudio = create<StudioState>((set) => ({
  current: mockProject,
  initialPrompt: '',
  startFromPrompt: (prompt) => set({ initialPrompt: prompt.trim(), current: newProjectFromPrompt(prompt) }),
  openProject: (project) => set({ initialPrompt: '', current: project }),
}));
