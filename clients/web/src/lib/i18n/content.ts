export type Locale = "en" | "es";

export const SUPPORTED_LOCALES: Array<{ code: Locale; label: string; short: string }> = [
  { code: "en", label: "English", short: "EN" },
  { code: "es", label: "Español", short: "ES" },
];

export const DEFAULT_LOCALE: Locale = "en";
export const LOCALE_COOKIE = "ironflyer.locale";

export function normalizeLocale(value: string | null | undefined): Locale {
  return value === "es" ? "es" : DEFAULT_LOCALE;
}

export interface MarketingHeroCopy {
  eyebrow: string;
  title: string;
  titleAccent?: string;
  subhead: string;
  primary: string;
  secondary: string;
  proofChips: string[];
}

export interface HomeCopy {
  hero: {
    eyebrow: string;
    titleStart: string;
    titleAccent: string;
    titleEnd: string;
    subhead: string;
    launchNote: string;
    proofChips: string[];
  };
  proof: Array<{ label: string; value: string; sub: string }>;
  templates: {
    title: string;
    cta: string;
  };
  how: {
    title: string;
    subhead: string;
    steps: Array<{ tag: string; title: string; body: string }>;
  };
  pricing: {
    eyebrow: string;
    title: string;
    body: string;
    primary: string;
    secondary: string;
  };
  footer: {
    body: string;
    copyright: string;
  };
}

export interface ContentPlugin {
  nav: {
    product: string;
    templates: string;
    mobile: string;
    solutions: string;
    pricing: string;
    resources: string;
    enterprise: string;
    login: string;
    startProject: string;
    startShort: string;
  };
  home: HomeCopy;
  pages: {
    product: MarketingHeroCopy;
    solutions: MarketingHeroCopy;
    enterprise: MarketingHeroCopy;
    developers: MarketingHeroCopy;
    mobile: MarketingHeroCopy;
    security: MarketingHeroCopy;
  };
}

const en: ContentPlugin = {
  nav: {
    product: "Product",
    templates: "Templates",
    mobile: "Mobile",
    solutions: "Solutions",
    pricing: "Pricing",
    resources: "Resources",
    enterprise: "Enterprise",
    login: "Log in",
    startProject: "Start a project",
    startShort: "Start",
  },
  home: {
    hero: {
      eyebrow: "AI-powered product builder",
      titleStart: "Build, review and ship production apps from a",
      titleAccent: "single prompt.",
      titleEnd: "",
      subhead:
        "Ironflyer turns a plain-language idea into screens, data, code, live preview and a launch lane. It is fast enough for non-technical founders and powerful enough for teams.",
      launchNote: "No credit card · Setup in 60 seconds · SOC 2 ready · GDPR compliant",
      proofChips: [
        "No-code to production",
        "Pro-grade codebase",
        "End-to-end systems",
        "Safer by default",
      ],
    },
    proof: [
      { label: "Time to working project", value: "30s", sub: "Prompt to active Studio workspace" },
      { label: "Build velocity", value: "20x", sub: "From idea to first runnable version" },
      { label: "Patches reviewable", value: "100%", sub: "Every change lands as a diff" },
      { label: "Gates before deploy", value: "12", sub: "Security, build, typecheck, E2E" },
      { label: "Cost control", value: "Live", sub: "Wallet and ProfitGuard visible" },
    ],
    templates: {
      title: "Start from a proven blueprint",
      cta: "Browse all blueprints",
    },
    how: {
      title: "Idea -> Working Product -> Ship",
      subhead: "One workspace from prompt to production. No tool stitching.",
      steps: [
        {
          tag: "01",
          title: "Describe",
          body: "Tell Ironflyer what you want in plain language. It turns the brief into a stack, scope, budget and execution plan.",
        },
        {
          tag: "02",
          title: "Build",
          body: "Studio writes real code as reviewable patches, runs gates, streams progress and keeps the workspace alive.",
        },
        {
          tag: "03",
          title: "Ship",
          body: "Preview, deploy, rollback and ledger sit together so builders and engineers can move without losing control.",
        },
      ],
    },
    pricing: {
      eyebrow: "Wallet, not subscription",
      title: "Pay for progress, not promises.",
      body:
        "Top up once and start building. Each execution reserves a budget, releases unused funds, and keeps provider cost visible beside the work.",
      primary: "See pricing",
      secondary: "Start with $0 balance",
    },
    footer: {
      body:
        "Ironflyer is an AI execution engine for complete applications and end-to-end systems: no-code speed, professional code, gates that block, and deployments you can trust.",
      copyright: "© 2026 Ironflyer. Build the future faster.",
    },
  },
  pages: {
    product: {
      eyebrow: "Product",
      title: "The AI application builder for finished systems.",
      titleAccent: "Not demos.",
      subhead:
        "Ironflyer turns intent into a real product workspace: code, mobile, backend, gates, deploys and cost control. It feels instant for non-technical founders and serious for senior developers.",
      primary: "Start a project",
      secondary: "See the wallet model",
      proofChips: ["No-code friendly", "Senior-engineer ready", "End-to-end systems"],
    },
    solutions: {
      eyebrow: "Solutions",
      title: "One AI studio for every stack you need to ship.",
      titleAccent: "Web, mobile, internal tools.",
      subhead:
        "Launch SaaS products, Expo apps, native mobile, internal systems and marketing sites through the same professional execution engine.",
      primary: "Start a project",
      secondary: "How the engine works",
      proofChips: ["6 production stacks", "1 gate chain", "1 append-only ledger"],
    },
    enterprise: {
      eyebrow: "Enterprise",
      title: "AI-built software under enterprise control.",
      titleAccent: "Fast and governed.",
      subhead:
        "Give teams an AI builder without losing security posture: SSO, audit logs, tenant isolation, BYO keys, BYO storage and self-hosted deployment options.",
      primary: "Talk to founder",
      secondary: "How the engine works",
      proofChips: ["SSO", "Audit log export", "Self-host available"],
    },
    developers: {
      eyebrow: "Developers",
      title: "A serious API for builders who want the metal.",
      titleAccent: "GraphQL, SSE, runtime.",
      subhead:
        "Use Ironflyer as a product surface or as infrastructure: typed GraphQL, live streams, runtime APIs, workspaces, ledgers and deploy events.",
      primary: "Get an API key",
      secondary: "Read the quickstart",
      proofChips: ["POST /graphql", "SSE streams", "Runtime API"],
    },
    mobile: {
      eyebrow: "Mobile",
      title: "Build mobile apps without waiting on the mobile pipeline.",
      titleAccent: "Expo and native paths.",
      subhead:
        "From idea to emulator, QR, build history and signed artifacts: Ironflyer brings mobile execution into the same AI workspace.",
      primary: "Start mobile project",
      secondary: "See solutions",
      proofChips: ["Expo", "Android", "iOS Pro tier"],
    },
    security: {
      eyebrow: "Security",
      title: "AI speed with gates that actually block.",
      titleAccent: "Control by design.",
      subhead:
        "Owner isolation, wallet hard-blocks, reviewable patches, security scans and append-only ledger events protect every execution.",
      primary: "Start safely",
      secondary: "Read developers docs",
      proofChips: ["OwnerID isolation", "GateSecurityScan", "Append-only ledger"],
    },
  },
};

const es: ContentPlugin = {
  nav: {
    product: "Producto",
    templates: "Plantillas",
    mobile: "Móvil",
    solutions: "Soluciones",
    pricing: "Precios",
    resources: "Recursos",
    enterprise: "Empresa",
    login: "Entrar",
    startProject: "Crear proyecto",
    startShort: "Crear",
  },
  home: {
    hero: {
      eyebrow: "Constructor de producto con IA",
      titleStart: "Construye, revisa y lanza apps de producción desde un",
      titleAccent: "solo prompt.",
      titleEnd: "",
      subhead:
        "Ironflyer convierte una idea en lenguaje natural en pantallas, datos, código, preview en vivo y un camino de lanzamiento. Es rápido para founders no técnicos y potente para equipos.",
      launchNote: "Sin tarjeta · Configuración en 60 segundos · SOC 2 ready · GDPR compliant",
      proofChips: [
        "No-code a producción",
        "Código profesional",
        "Sistemas end-to-end",
        "Seguro por defecto",
      ],
    },
    proof: [
      { label: "Tiempo a proyecto activo", value: "30s", sub: "Del prompt al workspace de Studio" },
      { label: "Velocidad de construcción", value: "20x", sub: "De idea a primera versión ejecutable" },
      { label: "Patches revisables", value: "100%", sub: "Cada cambio llega como diff" },
      { label: "Gates antes de deploy", value: "12", sub: "Seguridad, build, tipos y E2E" },
      { label: "Control de coste", value: "Live", sub: "Wallet y ProfitGuard visibles" },
    ],
    templates: {
      title: "Empieza desde una plantilla probada",
      cta: "Ver todas las plantillas",
    },
    how: {
      title: "Idea -> Producto activo -> Deploy",
      subhead: "Un workspace desde prompt hasta producción. Sin pegar herramientas.",
      steps: [
        {
          tag: "01",
          title: "Describe",
          body: "Cuenta lo que quieres en lenguaje natural. Ironflyer lo convierte en stack, alcance, presupuesto y plan de ejecución.",
        },
        {
          tag: "02",
          title: "Construye",
          body: "Studio escribe código real como patches revisables, ejecuta gates, transmite progreso y mantiene vivo el workspace.",
        },
        {
          tag: "03",
          title: "Lanza",
          body: "Preview, deploy, rollback y ledger viven juntos para que founders e ingenieros avancen sin perder control.",
        },
      ],
    },
    pricing: {
      eyebrow: "Wallet, no suscripción",
      title: "Paga por progreso, no por promesas.",
      body:
        "Recarga una vez y empieza. Cada ejecución reserva presupuesto, libera lo no usado y muestra el coste del proveedor junto al trabajo.",
      primary: "Ver precios",
      secondary: "Empezar con balance $0",
    },
    footer: {
      body:
        "Ironflyer es un motor de ejecución con IA para aplicaciones completas y sistemas end-to-end: velocidad no-code, código profesional, gates que bloquean y deploys confiables.",
      copyright: "© 2026 Ironflyer. Construye el futuro más rápido.",
    },
  },
  pages: {
    product: {
      eyebrow: "Producto",
      title: "El constructor de aplicaciones con IA para sistemas terminados.",
      titleAccent: "No demos.",
      subhead:
        "Ironflyer convierte intención en un workspace real: código, móvil, backend, gates, deploys y control de coste. Se siente instantáneo para founders y serio para developers senior.",
      primary: "Crear proyecto",
      secondary: "Ver modelo wallet",
      proofChips: ["Amigable no-code", "Listo para seniors", "Sistemas end-to-end"],
    },
    solutions: {
      eyebrow: "Soluciones",
      title: "Un estudio de IA para cada stack que necesitas lanzar.",
      titleAccent: "Web, móvil, herramientas internas.",
      subhead:
        "Lanza SaaS, apps Expo, móvil nativo, sistemas internos y sitios de marketing con el mismo motor profesional de ejecución.",
      primary: "Crear proyecto",
      secondary: "Cómo funciona",
      proofChips: ["6 stacks de producción", "1 cadena de gates", "1 ledger append-only"],
    },
    enterprise: {
      eyebrow: "Empresa",
      title: "Software creado con IA bajo control empresarial.",
      titleAccent: "Rápido y gobernado.",
      subhead:
        "Da a tus equipos un builder con IA sin perder postura de seguridad: SSO, audit logs, aislamiento de tenant, BYO keys, BYO storage y opciones self-hosted.",
      primary: "Hablar con founder",
      secondary: "Cómo funciona",
      proofChips: ["SSO", "Export de audit logs", "Self-host disponible"],
    },
    developers: {
      eyebrow: "Developers",
      title: "Una API seria para builders que quieren control total.",
      titleAccent: "GraphQL, SSE, runtime.",
      subhead:
        "Usa Ironflyer como producto o infraestructura: GraphQL tipado, streams en vivo, runtime APIs, workspaces, ledgers y eventos de deploy.",
      primary: "Obtener API key",
      secondary: "Leer quickstart",
      proofChips: ["POST /graphql", "Streams SSE", "Runtime API"],
    },
    mobile: {
      eyebrow: "Móvil",
      title: "Construye apps móviles sin esperar al pipeline móvil.",
      titleAccent: "Expo y nativo.",
      subhead:
        "De idea a emulador, QR, historial de builds y artefactos firmados: Ironflyer trae mobile al mismo workspace de IA.",
      primary: "Crear app móvil",
      secondary: "Ver soluciones",
      proofChips: ["Expo", "Android", "iOS Pro tier"],
    },
    security: {
      eyebrow: "Seguridad",
      title: "Velocidad de IA con gates que bloquean de verdad.",
      titleAccent: "Control por diseño.",
      subhead:
        "Aislamiento OwnerID, hard-blocks de wallet, patches revisables, security scans y ledger append-only protegen cada ejecución.",
      primary: "Empezar seguro",
      secondary: "Leer docs developers",
      proofChips: ["OwnerID isolation", "GateSecurityScan", "Ledger append-only"],
    },
  },
};

const dictionaries: Record<Locale, ContentPlugin> = { en, es };

export function getContentPlugin(locale: Locale = DEFAULT_LOCALE): ContentPlugin {
  return dictionaries[locale] ?? dictionaries.en;
}
