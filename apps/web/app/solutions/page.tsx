import { Diversity3Rounded, Inventory2Outlined, TravelExploreRounded } from "@mui/icons-material";
import { ReferencePage } from "../../src/components/ReferencePage";

export const metadata = {
  title: "Solutions - IronFlyer",
  description: "Product patterns for founders, teams, agencies and enterprise builders.",
};

export default function SolutionsPage() {
  return (
    <ReferencePage
      eyebrow="solutions"
      title="Build the product pattern your team actually needs."
      description="Start from proven flows for client portals, internal tools, marketplaces, education apps, dashboards and customer operations."
      primaryCta={{ label: "Browse templates", href: "/templates" }}
      secondaryCta={{ label: "Start in Studio", href: "/studio" }}
      sections={[
        {
          title: "Founders",
          body: "Move from idea to live preview quickly with product planning, auth, billing and admin flows.",
          icon: <TravelExploreRounded />,
        },
        {
          title: "Agencies",
          body: "Prototype client portals and operational products with reusable foundations and reviewable code.",
          icon: <Inventory2Outlined />,
        },
        {
          title: "Teams",
          body: "Keep roles, approvals, deploy checks and ownership clear as the product grows.",
          icon: <Diversity3Rounded />,
        },
      ]}
      workflow={[
        ["Choose pattern", "Pick the closest product shape."],
        ["Generate", "Let Studio build the first working version."],
        ["Review", "Inspect code, UI and launch gates."],
        ["Extend", "Iterate with team feedback and deploy."],
      ]}
    />
  );
}
