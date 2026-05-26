// Marketing primitives — used by /pricing, /product, /solutions,
// /enterprise. Server components by default; FaqAccordion is the only
// client component because MUI Accordion owns its expansion state.

export { MarketingHero } from "./MarketingHero";
export type { MarketingHeroProps } from "./MarketingHero";
export { MarketingSection } from "./MarketingSection";
export type { MarketingSectionProps } from "./MarketingSection";
export { MechanicCard } from "./MechanicCard";
export type { MechanicCardProps } from "./MechanicCard";
export { FaqAccordion } from "./FaqAccordion";
export type { FaqAccordionProps, FaqItem } from "./FaqAccordion";
export { ComparisonTable } from "./ComparisonTable";
export type {
  ComparisonTableProps,
  ComparisonColumn,
  ComparisonRow,
} from "./ComparisonTable";
export { CtaBand } from "./CtaBand";
export type { CtaBandProps } from "./CtaBand";
export { BrandBackdrop } from "./BrandBackdrop";
export { ProductTheater } from "./ProductTheater";
export { LanguageSwitcher } from "./LanguageSwitcher";
