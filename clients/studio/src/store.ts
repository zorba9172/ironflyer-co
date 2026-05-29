import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import { mockProject, newProjectFromPrompt, type StudioProject, type Agent, type Crew } from './studioData';

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

export interface GeneratedFile {
  path: string;
  content: string;
  /** bumped each time the file is (re)written, so the editor can flag activity */
  rev: number;
}

// --- Chat sessions (Copilot-style history) ------------------------------
export interface ChatMsg {
  id: string;
  from: 'user' | 'agent';
  text: string;
  /** orchestrator tool/phase steps surfaced inline */
  steps?: string[];
  /** streamed chain-of-thought, shown in a collapsible reasoning block */
  thinking?: string;
  /** agent ids this user turn was routed to via @-mention */
  routedTo?: string[];
  ts: number;
}

export interface ChatSession {
  id: string;
  title: string;
  messages: ChatMsg[];
  createdAt: number;
  updatedAt: number;
  /** archived sessions drop out of the main list but stay recoverable */
  archived: boolean;
}

export type LaunchWorkMode = 'ask' | 'plan' | 'execute' | 'autopilot';

interface LaunchOptions {
  workMode?: LaunchWorkMode;
  preflight?: boolean;
}

function deriveTitle(messages: ChatMsg[]): string {
  const first = messages.find((m) => m.from === 'user');
  const t = (first?.text ?? '').trim().replace(/\s+/g, ' ');
  if (!t) return 'New chat';
  return t.length > 44 ? `${t.slice(0, 44)}…` : t;
}

export function makeChatSession(seed: ChatMsg[] = []): ChatSession {
  const now = Date.now();
  return { id: `chat_${now.toString(36)}_${Math.random().toString(36).slice(2, 6)}`, title: deriveTitle(seed), messages: seed, createdAt: now, updatedAt: now, archived: false };
}

interface StudioState {
  current: StudioProject;
  /** prompt that started the session, '' when opened from a recent project */
  initialPrompt: string;
  /** transient launch preference consumed by ChatPanel on first mount */
  initialWorkMode: LaunchWorkMode;
  /** transient launch preference for the ChatPanel preflight gate */
  initialPreflight: boolean;
  /** gate open in the inspector drawer, null when closed */
  selectedGateId: string | null;
  /** transient: an inner sub-tab a deep-link wants a workspace pane to open on
   *  (e.g. the Map's Performance facet → Quality › Performance). Read once by the
   *  target workspace on mount, then cleared. Never persisted. */
  innerTab: string | null;
  /** the project's goal / "constitution" — the rules the finisher must honor */
  constitution: string;
  /** research the user uploaded (docs/images/notes) that ground the chat */
  attachments: Attachment[];
  /** operator-created agents (the built-in roster lives in studioData AGENTS) */
  customAgents: Agent[];
  /** operator-created crews — agents grouped to run together */
  crews: Crew[];
  /** files the chat agent has written this session — shown live in the Code tab */
  generatedFiles: GeneratedFile[];
  /** backend id of the project this session edits, null until first save/open */
  liveProjectId: string | null;
  /** true once generatedFiles have been persisted to the backend with no edits since */
  saved: boolean;
  /** every chat conversation the operator has had, newest activity first */
  chatSessions: ChatSession[];
  /** the conversation currently shown in the chat panel */
  activeChatId: string | null;
  /** a prompt queued for the chat (e.g. "fix the broken preview"); the chat
   *  panel picks it up, sends it, and clears it. Transient — never persisted. */
  repairRequest: string | null;
  startFromPrompt: (prompt: string, options?: LaunchOptions) => void;
  openProject: (project: StudioProject) => void;
  /** open a real backend project: set its id, load its files, mark saved */
  openLiveProject: (project: StudioProject, backendId: string, files: { path: string; content: string }[]) => void;
  /** Instant-start: seed a runnable scaffold so the live preview renders in
   *  seconds, then the agent enhances it on top. The Base44 speed-feel, but
   *  on top of the finisher discipline (gates/cost/private). */
  startFromTemplate: (prompt: string, files: { path: string; content: string }[], options?: LaunchOptions) => void;
  setLiveProjectId: (id: string | null) => void;
  markSaved: () => void;
  selectGate: (id: string | null) => void;
  /** queue an inner sub-tab for the next workspace pane to open on, then clear */
  setInnerTab: (v: string | null) => void;
  addAttachments: (items: Attachment[]) => void;
  removeAttachment: (id: string) => void;
  updateAttachment: (id: string, text: string) => void;
  addAgent: (agent: Agent) => void;
  updateAgent: (agent: Agent) => void;
  removeAgent: (id: string) => void;
  addCrew: (crew: Crew) => void;
  updateCrew: (crew: Crew) => void;
  removeCrew: (id: string) => void;
  writeGeneratedFiles: (files: { path: string; content: string }[]) => void;
  clearGeneratedFiles: () => void;
  // chat sessions
  /** return the active chat id, creating an empty session if none exists */
  ensureChat: () => string;
  /** start a fresh conversation (optionally seeded) and make it active */
  newChat: (seed?: ChatMsg[]) => string;
  selectChat: (id: string) => void;
  /** replace a session's messages (called at message boundaries, not per token) */
  commitChat: (id: string, messages: ChatMsg[]) => void;
  renameChat: (id: string, title: string) => void;
  archiveChat: (id: string) => void;
  restoreChat: (id: string) => void;
  deleteChat: (id: string) => void;
  /** queue a prompt for the chat to send (used by the "fix preview" action) */
  requestRepair: (prompt: string) => void;
  clearRepairRequest: () => void;
}

// A brand-new conversation made active, with prior sessions kept in history.
function freshChat(prior: ChatSession[]): { chatSessions: ChatSession[]; activeChatId: string } {
  const session = makeChatSession();
  return { chatSessions: [session, ...prior], activeChatId: session.id };
}

export const useStudio = create<StudioState>()(
  persist(
    (set, get) => ({
      current: mockProject,
      initialPrompt: '',
      initialWorkMode: 'ask',
      initialPreflight: false,
      selectedGateId: null,
      innerTab: null,
      constitution: '',
      attachments: [],
      customAgents: [],
      crews: [],
      generatedFiles: [],
      liveProjectId: null,
      saved: false,
      chatSessions: [],
      activeChatId: null,
      repairRequest: null,
      startFromPrompt: (prompt, options = {}) => set((s) => ({ initialPrompt: prompt.trim(), initialWorkMode: options.workMode ?? 'execute', initialPreflight: options.preflight ?? false, current: newProjectFromPrompt(prompt), selectedGateId: null, constitution: prompt.trim(), generatedFiles: [], liveProjectId: null, saved: false, ...freshChat(s.chatSessions) })),
      openProject: (project) => set((s) => ({ initialPrompt: '', initialWorkMode: 'ask', initialPreflight: false, current: project, selectedGateId: null, generatedFiles: [], liveProjectId: null, saved: false, ...freshChat(s.chatSessions) })),
      openLiveProject: (project, backendId, files) => set((s) => ({
        initialPrompt: '', initialWorkMode: 'ask', initialPreflight: false, current: project, selectedGateId: null, liveProjectId: backendId, saved: true,
        generatedFiles: files.map((f) => ({ path: f.path, content: f.content, rev: 1 })), ...freshChat(s.chatSessions),
      })),
      startFromTemplate: (prompt, files, options = {}) => set((s) => ({
        initialPrompt: prompt.trim(), initialWorkMode: options.workMode ?? 'execute', initialPreflight: options.preflight ?? false, current: newProjectFromPrompt(prompt), selectedGateId: null,
        constitution: prompt.trim(), liveProjectId: null, saved: false,
        generatedFiles: files.map((f) => ({ path: f.path, content: f.content, rev: 1 })),
        ...freshChat(s.chatSessions),
      })),
      setLiveProjectId: (id) => set({ liveProjectId: id }),
      markSaved: () => set({ saved: true }),
      selectGate: (id) => set({ selectedGateId: id }),
      setInnerTab: (v) => set({ innerTab: v }),
      addAttachments: (items) => set((s) => ({ attachments: [...s.attachments, ...items] })),
      removeAttachment: (id) => set((s) => ({ attachments: s.attachments.filter((a) => a.id !== id) })),
      updateAttachment: (id, text) => set((s) => ({ attachments: s.attachments.map((a) => (a.id === id ? { ...a, text } : a)) })),
      addAgent: (agent) => set((s) => ({ customAgents: [...s.customAgents, agent] })),
      updateAgent: (agent) => set((s) => ({ customAgents: s.customAgents.map((a) => (a.id === agent.id ? agent : a)) })),
      removeAgent: (id) => set((s) => ({ customAgents: s.customAgents.filter((a) => a.id !== id), crews: s.crews.map((c) => ({ ...c, memberIds: c.memberIds.filter((m) => m !== id), managerId: c.managerId === id ? undefined : c.managerId })) })),
      addCrew: (crew) => set((s) => ({ crews: [...s.crews, crew] })),
      updateCrew: (crew) => set((s) => ({ crews: s.crews.map((c) => (c.id === crew.id ? crew : c)) })),
      removeCrew: (id) => set((s) => ({ crews: s.crews.filter((c) => c.id !== id) })),
      writeGeneratedFiles: (files) => set((s) => {
        if (files.length === 0) return s;
        const map = new Map(s.generatedFiles.map((f) => [f.path, f]));
        let changed = false;
        for (const f of files) {
          const prev = map.get(f.path);
          if (!prev || prev.content !== f.content) { map.set(f.path, { path: f.path, content: f.content, rev: (prev?.rev ?? 0) + 1 }); changed = true; }
        }
        return changed ? { generatedFiles: [...map.values()], saved: false } : s;
      }),
      clearGeneratedFiles: () => set({ generatedFiles: [], saved: false }),
      ensureChat: () => {
        const { activeChatId, chatSessions } = get();
        if (activeChatId && chatSessions.some((c) => c.id === activeChatId && !c.archived)) return activeChatId;
        const live = chatSessions.find((c) => !c.archived);
        if (live) { set({ activeChatId: live.id }); return live.id; }
        const session = makeChatSession();
        set((s) => ({ chatSessions: [session, ...s.chatSessions], activeChatId: session.id }));
        return session.id;
      },
      newChat: (seed = []) => {
        const session = makeChatSession(seed);
        set((s) => ({ chatSessions: [session, ...s.chatSessions], activeChatId: session.id }));
        return session.id;
      },
      selectChat: (id) => set((s) => ({ activeChatId: s.chatSessions.some((c) => c.id === id) ? id : s.activeChatId })),
      commitChat: (id, messages) => set((s) => ({
        chatSessions: s.chatSessions.map((c) =>
          c.id === id
            ? { ...c, messages, updatedAt: Date.now(), title: c.title === 'New chat' ? deriveTitle(messages) : c.title }
            : c,
        ),
      })),
      renameChat: (id, title) => set((s) => ({ chatSessions: s.chatSessions.map((c) => (c.id === id ? { ...c, title: title.trim() || c.title } : c)) })),
      archiveChat: (id) => set((s) => {
        const chatSessions = s.chatSessions.map((c) => (c.id === id ? { ...c, archived: true } : c));
        const activeChatId = s.activeChatId === id ? (chatSessions.find((c) => !c.archived)?.id ?? null) : s.activeChatId;
        return { chatSessions, activeChatId };
      }),
      restoreChat: (id) => set((s) => ({ chatSessions: s.chatSessions.map((c) => (c.id === id ? { ...c, archived: false } : c)) })),
      deleteChat: (id) => set((s) => {
        const chatSessions = s.chatSessions.filter((c) => c.id !== id);
        const activeChatId = s.activeChatId === id ? (chatSessions.find((c) => !c.archived)?.id ?? null) : s.activeChatId;
        return { chatSessions, activeChatId };
      }),
      requestRepair: (prompt) => set({ repairRequest: prompt }),
      clearRepairRequest: () => set({ repairRequest: null }),
    }),
    {
      name: 'ironflyer-studio',
      storage: createJSONStorage(() => localStorage),
      // Persist only the work-in-progress so a refresh doesn't lose an unsaved
      // build. Transient UI (selected gate, inspector) is intentionally dropped.
      partialize: (s) => ({ current: s.current, constitution: s.constitution, attachments: s.attachments, customAgents: s.customAgents, crews: s.crews, generatedFiles: s.generatedFiles, liveProjectId: s.liveProjectId, saved: s.saved, chatSessions: s.chatSessions, activeChatId: s.activeChatId }),
    },
  ),
);

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
