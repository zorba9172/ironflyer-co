import { AutoStoriesRounded, CodeRounded, SettingsSuggestRounded } from "@mui/icons-material";
import { ReferencePage } from "../../src/components/ReferencePage";

export const metadata = {
  title: "Resources - IronFlyer",
  description: "Guides, docs, templates and product education for IronFlyer builders.",
};

export default function ResourcesPage() {
  return (
    <ReferencePage
      eyebrow="resources"
      title="Guides and references for serious builders."
      description="Use resources to understand product architecture, templates, integration patterns, deployment flow and review standards."
      primaryCta={{ label: "Open docs", href: "/resources" }}
      secondaryCta={{ label: "View templates", href: "/templates" }}
      sections={[
        {
          title: "Guides",
          body: "Practical build patterns for portals, dashboards, marketplaces and workflow products.",
          icon: <AutoStoriesRounded />,
        },
        {
          title: "Developer handoff",
          body: "Understand generated code, ownership, export paths and review expectations.",
          icon: <CodeRounded />,
        },
        {
          title: "Operations",
          body: "Plan environments, secrets, deploy checks, rollback and ongoing product health.",
          icon: <SettingsSuggestRounded />,
        },
      ]}
      workflow={[
        ["Learn", "Pick the guide that matches your build."],
        ["Apply", "Start with the right template or prompt."],
        ["Review", "Use the checklist before launch."],
        ["Ship", "Move from preview to production."],
      ]}
    />
  );
}
