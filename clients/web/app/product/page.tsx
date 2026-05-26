import { CodeRounded, DataObjectRounded, RocketLaunchRounded } from "@mui/icons-material";
import { ReferencePage } from "../../src/components/ReferencePage";

export const metadata = {
  title: "Product - IronFlyer",
  description: "Build, review and ship production apps from one connected IronFlyer Studio flow.",
};

export default function ProductPage() {
  return (
    <ReferencePage
      eyebrow="product"
      title="A production app builder connected end to end."
      description="IronFlyer turns a plain-language product request into plan, screens, data models, code, review, preview and deploy inside one Studio workspace."
      sections={[
        {
          title: "AI Product Architect",
          body: "Translate intent into roles, flows, data, screens and acceptance criteria before code starts.",
          icon: <DataObjectRounded />,
        },
        {
          title: "Code you own",
          body: "Generate production React and TypeScript that remains visible, reviewable and exportable.",
          icon: <CodeRounded />,
        },
        {
          title: "Launch lane",
          body: "Preview, gate, deploy and roll back from the same workspace without stitching tools together.",
          icon: <RocketLaunchRounded />,
        },
      ]}
      workflow={[
        ["Prompt", "Describe the product in natural language."],
        ["Plan", "Lock structure, roles and data models."],
        ["Build", "Generate UI, code, integrations and mobile states."],
        ["Deploy", "Review gates and ship with confidence."],
      ]}
    />
  );
}
