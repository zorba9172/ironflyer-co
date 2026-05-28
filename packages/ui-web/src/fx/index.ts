// Shared interactive/effects layer for every web app. Heavy libs (React Flow,
// three.js) are lazy + client-only; all are theme/token-mapped.
export { motion, AnimatePresence, presets, Reveal, type Variants } from './motion';
export { confirmAction, toast } from './dialog';
export { Carousel } from './Carousel';
export { Lightbox } from './Lightbox';
export { FlowCanvas, type FlowNode, type FlowEdge, type NodeMouseHandler } from './FlowCanvas';
export { Scene3D } from './Scene3D';
export { useMounted } from './useMounted';
