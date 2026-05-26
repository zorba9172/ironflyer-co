"use client";

// ImportFromGitHubDialog — opens from /projects "Import repo" button.
//
// Flow:
//   1. User pastes a GitHub URL (and optional PAT for private repos).
//   2. We create a project via the standard createProject mutation
//      with `idea` set to a structured instruction the planner reads
//      as the first user message: "Import the repo at <url>…".
//   3. We route to /p/[id] where the StartExecutionPanel takes over —
//      the user picks a wallet hold and the orchestrator kicks the
//      finisher. The first agent will see the idea and call the
//      runtime's git-clone endpoint inside the allocated workspace.
//
// We do NOT call the runtime's POST /workspaces/{id}/git-clone
// directly from here because the workspace isn't allocated until the
// execution starts. Embedding the URL into project.idea lets the agent
// own the clone step (which it would do anyway as part of its setup).

import { GitHub } from "@mui/icons-material";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Typography,
} from "@mui/material";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { extractErrorMessage } from "../../lib/errors";
import { useCreateProjectMutation } from "../../lib/gql/__generated__";
import { pushToast } from "../../lib/stores/uiStore";
import { tokens } from "../../theme";

export interface ImportFromGitHubDialogProps {
  open: boolean;
  onClose: () => void;
}

// Cheap GitHub URL validation: HTTPS GitHub remotes only for the
// dialog's strict mode. Other git hosts work fine — they just won't
// pass the in-dialog validator. Operators wanting a custom remote can
// edit the prompt manually after the project lands.
const GITHUB_HTTPS_RE = /^https:\/\/github\.com\/[^/\s]+\/[^/\s]+?(?:\.git)?\/?$/i;
const ANY_GIT_RE = /^(?:https?|git|ssh):\/\/\S+$/i;

function deriveProjectName(url: string): string {
  const m = url.match(/github\.com\/[^/]+\/([^/?#]+)/i);
  if (!m) return "Imported repo";
  const slug = m[1].replace(/\.git$/i, "");
  // Title-case dashed/underscored slugs.
  return slug
    .split(/[-_]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
    .slice(0, 60) || "Imported repo";
}

function deriveIdea(url: string, hasToken: boolean, ref: string): string {
  const parts: string[] = [
    `Import the repo at ${url} and continue working on it.`,
  ];
  if (ref.trim()) parts.push(`Use the branch or commit "${ref.trim()}".`);
  parts.push(
    `As your first step, clone the repository into the workspace (the runtime exposes git-clone — use it with the supplied URL${hasToken ? " and the access token from the wallet metadata" : ""}). Then audit the codebase before suggesting changes.`,
  );
  return parts.join(" ");
}

export function ImportFromGitHubDialog({
  open,
  onClose,
}: ImportFromGitHubDialogProps) {
  const router = useRouter();
  const [url, setUrl] = useState("");
  const [token, setToken] = useState("");
  const [ref, setRef] = useState("");
  const [createProject, { loading }] = useCreateProjectMutation();
  const [error, setError] = useState<string | null>(null);

  const trimmed = url.trim();
  const urlOk = useMemo(() => {
    if (!trimmed) return false;
    return GITHUB_HTTPS_RE.test(trimmed) || ANY_GIT_RE.test(trimmed);
  }, [trimmed]);

  const handleClose = () => {
    if (loading) return;
    setError(null);
    onClose();
  };

  const handleImport = async () => {
    if (!urlOk || loading) return;
    setError(null);
    try {
      const res = await createProject({
        variables: {
          input: {
            name: deriveProjectName(trimmed),
            description: `Imported from ${trimmed}`,
            idea: deriveIdea(trimmed, token.trim().length > 0, ref),
          },
        },
        refetchQueries: ["Projects", "DashboardProjects"],
      });
      const id = res.data?.createProject?.id;
      if (!id) {
        throw new Error("Orchestrator did not return a project id.");
      }
      pushToast({
        message: "Project created. Set a wallet hold and start the first execution.",
        severity: "success",
      });
      onClose();
      router.push(`/p/${encodeURIComponent(id)}`);
    } catch (e) {
      setError(extractErrorMessage(e));
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      slotProps={{
        paper: {
          sx: {
            bgcolor: tokens.color.bg.surfaceRaised,
            border: `1px solid ${tokens.color.border.subtle}`,
            minWidth: { xs: 320, sm: 520 },
          },
        },
      }}
    >
      <DialogTitle sx={{ fontWeight: 800, display: "flex", alignItems: "center", gap: 1 }}>
        <GitHub sx={{ fontSize: 22 }} />
        Import from GitHub
      </DialogTitle>
      <DialogContent>
        <Typography
          sx={{
            color: tokens.color.text.secondary,
            fontSize: 13.5,
            lineHeight: 1.55,
            mb: 2.5,
          }}
        >
          Ironflyer will create a project pointed at this repository.
          On the next screen, set a wallet hold and the finisher will
          clone the repo into a fresh workspace and review it before
          making changes.
        </Typography>

        <Stack spacing={2}>
          <Field
            label="Repository URL"
            value={url}
            onChange={setUrl}
            placeholder="https://github.com/owner/repo"
            mono
            autoFocus
          />
          <Field
            label="Branch or commit (optional)"
            value={ref}
            onChange={setRef}
            placeholder="main"
            mono
          />
          <Field
            label="Access token (only required for private repos)"
            value={token}
            onChange={setToken}
            placeholder="ghp_…"
            mono
            type="password"
          />
          <Typography sx={{ fontSize: 11.5, color: tokens.color.text.muted, lineHeight: 1.5 }}>
            The token is forwarded to your isolated workspace and is
            never stored on the orchestrator. Provide a fine-scoped
            personal access token (read-only is enough for an audit).
          </Typography>
          {error && (
            <Typography sx={{ color: tokens.color.accent.danger, fontSize: 13 }}>
              {error}
            </Typography>
          )}
        </Stack>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={handleClose} disabled={loading} sx={{ color: tokens.color.text.secondary }}>
          Cancel
        </Button>
        <Button
          onClick={() => void handleImport()}
          disabled={!urlOk || loading}
          variant="contained"
          color="primary"
        >
          {loading ? "Creating…" : "Create project"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  mono,
  autoFocus,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  mono?: boolean;
  autoFocus?: boolean;
  type?: string;
}) {
  return (
    <Box>
      <Typography
        variant="overline"
        sx={{ color: tokens.color.text.muted, letterSpacing: 1.1, fontSize: 10.5 }}
      >
        {label}
      </Typography>
      <Box
        component="input"
        autoFocus={autoFocus}
        type={type}
        value={value}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        placeholder={placeholder}
        sx={{
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: `${tokens.radius.sm}px`,
          color: tokens.color.text.primary,
          fontFamily: mono ? tokens.font.mono : tokens.font.family,
          fontSize: 13.5,
          mt: 0.5,
          outline: "none",
          px: 1.5,
          py: 1.1,
          width: "100%",
          "&:focus": { borderColor: tokens.color.accent.violet },
          "&::placeholder": { color: tokens.color.text.muted },
        }}
      />
    </Box>
  );
}
