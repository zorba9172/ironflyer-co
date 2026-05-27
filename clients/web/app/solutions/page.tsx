import type { Metadata } from "next";
import { Suspense } from "react";
import { Base44PublicPage } from "../../src/components/marketing/Base44PublicPage";

export const metadata: Metadata = {
  title: "Solutions - IronFlyer",
  description:
    "Build customer portals, internal tools, marketplaces, mobile products and developer workflows from a prompt.",
};

export default function SolutionsPage() {
  return (
    <Suspense fallback={null}>
      <Base44PublicPage page="solutions" />
    </Suspense>
  );
}
