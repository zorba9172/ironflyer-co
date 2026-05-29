import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

// Which connectors the operator has linked. Persisted locally — there is no
// third-party OAuth backend yet, so this records the operator's connect intent
// that the finisher honors; it does not store credentials.
interface IntegrationsState {
  connected: string[];
  toggle: (name: string) => void;
}

export const useIntegrations = create<IntegrationsState>()(
  persist(
    (set) => ({
      connected: [],
      toggle: (name) => set((s) => ({
        connected: s.connected.includes(name) ? s.connected.filter((n) => n !== name) : [...s.connected, name],
      })),
    }),
    { name: 'ironflyer-integrations', storage: createJSONStorage(() => localStorage) },
  ),
);
