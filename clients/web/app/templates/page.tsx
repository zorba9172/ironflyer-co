import type { Metadata } from "next";
import { Suspense } from "react";
import { Base44PublicPage } from "../../src/components/marketing/Base44PublicPage";

export const metadata: Metadata = {
  title: "Templates - IronFlyer",
  description:
    "Start from proven app templates for portals, dashboards, marketplaces, internal tools and mobile products.",
};

export default function TemplatesPage() {
  return (
    <Suspense fallback={null}>
      <Base44PublicPage page="templates" />
    </Suspense>
  );
}
