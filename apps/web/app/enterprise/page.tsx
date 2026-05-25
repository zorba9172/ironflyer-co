import { AdminPanelSettingsRounded, SecurityRounded, SupportAgentRounded } from "@mui/icons-material";
import { ReferencePage } from "../../src/components/ReferencePage";

export const metadata = {
  title: "Enterprise - IronFlyer",
  description: "Security, governance and launch support for teams building production apps with IronFlyer.",
};

export default function EnterprisePage() {
  return (
    <ReferencePage
      eyebrow="enterprise"
      title="Governed AI app building for production teams."
      description="Give teams the speed of prompt-to-product while keeping security, roles, review, environments and launch ownership under control."
      primaryCta={{ label: "Contact sales", href: "/pricing" }}
      secondaryCta={{ label: "Review pricing", href: "/pricing" }}
      stats={[["SSO", "Ready"], ["RBAC", "Built in"], ["24/7", "Launch support"]]}
      sections={[
        {
          title: "Access control",
          body: "Role-based workspaces, team permissions, review ownership and environment separation.",
          icon: <AdminPanelSettingsRounded />,
        },
        {
          title: "Security posture",
          body: "Secrets checks, audit trails, compliance-friendly review and production deploy gates.",
          icon: <SecurityRounded />,
        },
        {
          title: "Launch support",
          body: "Architecture review, onboarding and guided rollout for high-stakes product launches.",
          icon: <SupportAgentRounded />,
        },
      ]}
      workflow={[
        ["Assess", "Map teams, data, roles and launch requirements."],
        ["Pilot", "Build the first governed workspace."],
        ["Review", "Validate code, security and deploy flow."],
        ["Scale", "Roll out templates and teams safely."],
      ]}
    />
  );
}
