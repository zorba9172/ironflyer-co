import type { Metadata } from "next";
import { Suspense } from "react";
import { Base44PublicPage } from "../../src/components/marketing/Base44PublicPage";

export const metadata: Metadata = {
  title: "VS Code Extension - IronFlyer",
  description:
    "Review IronFlyer patches, gates, previews and run output directly inside VS Code.",
};

export default function VscodePage() {
  return (
    <Suspense fallback={null}>
      <Base44PublicPage page="vscode" />
    </Suspense>
  );
}
