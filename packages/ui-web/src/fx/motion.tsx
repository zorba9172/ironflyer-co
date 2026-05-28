import { motion, AnimatePresence, type Variants } from 'framer-motion';
import type { ReactNode } from 'react';

export { motion, AnimatePresence };
export type { Variants };

// Shared motion presets so every app animates with one rhythm.
export const presets = {
  fadeUp: {
    initial: { opacity: 0, y: 16 },
    animate: { opacity: 1, y: 0 },
    transition: { duration: 0.5, ease: [0.22, 1, 0.36, 1] },
  },
  fade: {
    initial: { opacity: 0 },
    animate: { opacity: 1 },
    transition: { duration: 0.35 },
  },
} as const;

// Reveal-on-scroll. Respects reduced-motion automatically via framer-motion.
export function Reveal({ children, delay = 0, y = 18 }: { children: ReactNode; delay?: number; y?: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: '-60px' }}
      transition={{ duration: 0.5, delay, ease: [0.22, 1, 0.36, 1] }}
    >
      {children}
    </motion.div>
  );
}
