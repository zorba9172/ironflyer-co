// Studio shared primitives — the floating-glass design language every pane
// composes from. Built strictly on the MUI theme + neon tokens (no raw
// literals) so the whole studio reads as one world, faithful to the locked
// home reference. 3D wrappers feed the neon palette into the lazy three.js
// layer in @ironflyer/ui-web/fx (heavy lib stays behind the lazy boundary).
export { GlassPanel, type GlassPanelProps } from './GlassPanel';
export { SectionHeader, type SectionHeaderProps } from './SectionHeader';
export { StatCard, type StatCardProps } from './StatCard';
export { GaugeRing, type GaugeRingProps } from './GaugeRing';
export { NeonBars3D, type NeonBars3DProps } from './NeonBars3D';
export { NeonConstellation3D, type NeonConstellation3DProps } from './NeonConstellation3D';
