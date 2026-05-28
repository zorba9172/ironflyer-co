import { create } from 'zustand';
import { mockProject, newProjectFromPrompt, type StudioProject } from './studioData';

export interface Attachment {
  id: string;
  name: string;
  kind: 'image' | 'text' | 'file';
  size: number;
  /** extracted text (for text docs) — fed to the chat as research context */
  text?: string;
  /** data URL preview (for images) */
  dataUrl?: string;
}

interface StudioState {
  current: StudioProject;
  /** prompt that started the session, '' when opened from a recent project */
  initialPrompt: string;
  /** gate open in the inspector drawer, null when closed */
  selectedGateId: string | null;
  /** the project's goal / "constitution" — the rules the finisher must honor */
  constitution: string;
  /** research the user uploaded (docs/images/notes) that ground the chat */
  attachments: Attachment[];
  startFromPrompt: (prompt: string) => void;
  openProject: (project: StudioProject) => void;
  selectGate: (id: string | null) => void;
  setConstitution: (text: string) => void;
  addAttachments: (items: Attachment[]) => void;
  removeAttachment: (id: string) => void;
}

export const useStudio = create<StudioState>((set) => ({
  current: mockProject,
  initialPrompt: '',
  selectedGateId: null,
  constitution: '',
  attachments: [],
  startFromPrompt: (prompt) => set({ initialPrompt: prompt.trim(), current: newProjectFromPrompt(prompt), selectedGateId: null, constitution: prompt.trim() }),
  openProject: (project) => set({ initialPrompt: '', current: project, selectedGateId: null }),
  selectGate: (id) => set({ selectedGateId: id }),
  setConstitution: (text) => set({ constitution: text }),
  addAttachments: (items) => set((s) => ({ attachments: [...s.attachments, ...items] })),
  removeAttachment: (id) => set((s) => ({ attachments: s.attachments.filter((a) => a.id !== id) })),
}));

// Compact context block fed to the chat so the agent is grounded in the
// project's constitution + the user's research.
export function buildFocusContext(constitution: string, attachments: Attachment[]): string {
  const parts: string[] = [];
  if (constitution.trim()) parts.push(`# Project constitution (rules to honor)\n${constitution.trim()}`);
  const research = attachments.filter((a) => a.text);
  if (research.length) {
    parts.push(
      `# Research provided by the operator\n${research
        .map((a) => `## ${a.name}\n${(a.text ?? '').slice(0, 4000)}`)
        .join('\n\n')}`,
    );
  }
  const refs = attachments.filter((a) => !a.text);
  if (refs.length) parts.push(`# Attached references\n${refs.map((a) => `- ${a.name} (${a.kind})`).join('\n')}`);
  return parts.join('\n\n');
}
