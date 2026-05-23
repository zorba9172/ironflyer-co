export type BrandAssetUse =
  | 'hero-motion'
  | 'evidence-motion'
  | 'product-surface'
  | 'capability-icon'
  | 'background'
  | 'reference-only';

export interface BrandAsset {
  src: string;
  poster?: string;
  alt: string;
  use: BrandAssetUse;
  source: string;
  guidance: string;
}

export const brandAssets = {
  motion: {
    finisherHero: {
      src: '/brand/motion/finisher-os-hero-loop.mp4',
      poster: '/brand/motion/finisher-os-hero-loop-poster.jpg',
      alt: 'Animated Ironflyer data flow background',
      use: 'hero-motion',
      source: 'assets/market/blue-big-data-flow-technology-background-2026-02-24-01-46-26-utc/Data-Analytics.jpg',
      guidance: 'Use full-bleed behind product copy and prompt surfaces with a dark overlay.',
    },
    gateEvidence: {
      src: '/brand/motion/gate-evidence-flow.mp4',
      poster: '/brand/motion/gate-evidence-flow-poster.jpg',
      alt: 'Animated evidence signal field',
      use: 'evidence-motion',
      source: 'assets/market/fluid-mapping-neon-contour-waves-2026-02-24-01-22-32-utc/03.jpg',
      guidance: 'Use for gate verdict, risk, security, or blocker moments. Keep copy short.',
    },
  },
  stills: {
    dataFlow: {
      src: '/brand/data-flow.jpg',
      alt: 'Ironflyer data flow field',
      use: 'background',
      source: 'assets/market/blue-big-data-flow-technology-background-2026-02-24-01-46-26-utc/Data-Analytics.jpg',
      guidance: 'Fallback poster and dense product surfaces.',
    },
    aiReplyCard: {
      src: '/brand/ai-reply-motion-card.png',
      alt: 'AI reply motion card',
      use: 'product-surface',
      source: 'assets/market/AI Replies/AI Replies/AI Replies 01/large.png',
      guidance: 'Use as a proof/reply object, never as the primary hero by itself.',
    },
    aiReplyThread: {
      src: '/brand/ai-reply-thread-card.png',
      alt: 'AI reply thread card',
      use: 'product-surface',
      source: 'assets/market/AI Replies/AI Replies/AI Replies 04/large.png',
      guidance: 'Use near chat, patch, or agent orchestration surfaces.',
    },
    systemOsField: {
      src: '/brand/system-os-field.jpg',
      alt: 'Dark modular system field',
      use: 'background',
      source: 'assets/market/abstract-background-geometric-set-geo_data3-2026-02-24-01-34-34-utc/10.png',
      guidance: 'Use for premium operating-system moments: final CTA, enterprise, and private-cloud surfaces.',
    },
  },
  glyphs: {
    codeCircuit: {
      src: '/brand/glyphs/code-circuit.svg',
      alt: 'Code circuit glyph',
      use: 'capability-icon',
      source: 'assets/market/Development/code-circuit.svg',
      guidance: 'Code gate, architecture, and generated app surfaces.',
    },
    terminal: {
      src: '/brand/glyphs/terminal-rectangle.svg',
      alt: 'Terminal glyph',
      use: 'capability-icon',
      source: 'assets/market/Development/terminal-rectangle.svg',
      guidance: 'Runtime, shell, logs, and workspace surfaces.',
    },
    shieldTick: {
      src: '/brand/glyphs/shield-tick.svg',
      alt: 'Shield tick glyph',
      use: 'capability-icon',
      source: 'assets/market/Development/shield-tick.svg',
      guidance: 'Security, compliance, and safe deploy moments.',
    },
    radar: {
      src: '/brand/glyphs/radar.svg',
      alt: 'Radar glyph',
      use: 'capability-icon',
      source: 'assets/market/Development/radar.svg',
      guidance: 'Detection, blockers, and visual QA.',
    },
    timeline: {
      src: '/brand/glyphs/timeline-alt.svg',
      alt: 'Timeline glyph',
      use: 'capability-icon',
      source: 'assets/market/Development/timeline-alt.svg',
      guidance: 'Audit log, project memory, and gate history.',
    },
    pullRequest: {
      src: '/brand/glyphs/code-pull-request.svg',
      alt: 'Pull request glyph',
      use: 'capability-icon',
      source: 'assets/market/Development/code-pull-request.svg',
      guidance: 'Patch lifecycle and reviewable diffs.',
    },
    cubes: {
      src: '/brand/glyphs/cubes.svg',
      alt: 'Cubes glyph',
      use: 'capability-icon',
      source: 'assets/market/Development/cubes.svg',
      guidance: 'Multi-stack, runtime, and deploy architecture.',
    },
    webSearch: {
      src: '/brand/glyphs/web-search.svg',
      alt: 'Web search glyph',
      use: 'capability-icon',
      source: 'assets/market/Development/web-search.svg',
      guidance: 'Research, docs lookup, and Context7-assisted work.',
    },
  },
} as const;

export type BrandAssets = typeof brandAssets;
