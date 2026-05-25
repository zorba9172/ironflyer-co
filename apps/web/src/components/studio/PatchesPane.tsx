"use client";

// PatchesPane — the V22 differentiator surface.
//
// Every file edit the finisher proposes lands here as a reviewable
// Patch row: status pill, per-file change list with op + symbol +
// inline content preview, and apply / rollback affordances. This is
// the user-facing answer to "what is the agent about to change, and
// can I block or undo it?".
//
// Schema notes:
//   • `patches(projectId:)` returns every patch on the project,
//     newest first. The orchestrator does not yet expose an
//     `executionID` filter on Patch, so we render the whole project
//     history — the most recent patches are the ones the active
//     execution wrote, which is what the user cares about.
//   • The schema does not surface per-file diff bodies (old vs new
//     blob). Each PatchChange carries op + path + (anchor /
//     replacement / symbol / content). We render whatever fields the
//     op exposes inside the expand panel.
//   • `rollbackPatch` is wired. There is no separate "repair" status
//     on Patch — the engine emits a new patch (with metadata
//     `source=repair`) instead. The status pill therefore shows the
//     raw lifecycle (PROPOSED / APPROVED / APPLIED / REJECTED /
//     ROLLED_BACK / CONFLICTED).
//
// Patches + apply/rollback mutations now flow through generated hooks
// (see src/lib/gql/operations/patches.graphql).

import {
  CheckCircleRounded,
  ExpandMoreRounded,
  RefreshRounded,
  UndoRounded,
  WarningAmberRounded,
} from "@mui/icons-material";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  CircularProgress,
  Snackbar,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import { useMemo, useState } from "react";
import { LoadingPanel } from "../cockpit/LoadingPanel";
import { StatusBadge } from "../cockpit/StatusBadge";
import { extractErrorMessage } from "../../lib/errors";
import { relativeTime } from "../../lib/relativeTime";
import { tokens } from "../../theme";
import {
  useApplyPatchMutation,
  usePatchesQuery,
  useRollbackPatchMutation,
  PatchStatus,
  type PatchCoreFragment,
} from "../../lib/gql/__generated__";

type Patch = PatchCoreFragment;

const TERMINAL_EXEC = new Set(["succeeded", "failed", "stopped", "killed", "refunded"]);

export interface PatchesPaneProps {
  projectID: string;
  executionStatus: string;
}

export function PatchesPane({ projectID, executionStatus }: PatchesPaneProps) {
  const isTerminalExec = TERMINAL_EXEC.has(executionStatus);
  const query = usePatchesQuery({
    variables: { projectId: projectID },
    skip: !projectID,
    fetchPolicy: "cache-and-network",
    pollInterval: isTerminalExec ? 0 : 5000,
  });

  const [applyPatch] = useApplyPatchMutation();
  const [rollbackPatch] = useRollbackPatchMutation();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [snack, setSnack] = useState<string | null>(null);

  const patches = useMemo(() => query.data?.patches ?? [], [query.data]);

  // Headline counts: how many patches landed, how many distinct files
  // were touched, and how many are currently APPLIED vs PENDING.
  const stats = useMemo(() => {
    const files = new Set<string>();
    let applied = 0;
    let pending = 0;
    let rolled = 0;
    let conflicted = 0;
    for (const p of patches) {
      for (const c of p.changes) files.add(c.path);
      const s = p.status.toUpperCase();
      if (s === "APPLIED") applied++;
      else if (s === "ROLLED_BACK") rolled++;
      else if (s === "CONFLICTED") conflicted++;
      else if (s === "PROPOSED" || s === "APPROVED") pending++;
    }
    return { files: files.size, applied, pending, rolled, conflicted };
  }, [patches]);

  const onApply = async (id: string) => {
    setBusyId(id);
    try {
      await applyPatch({ variables: { id } });
      setSnack("Patch applied.");
      await query.refetch();
    } catch (e) {
      setSnack(extractErrorMessage(e));
    } finally {
      setBusyId(null);
    }
  };

  const onRollback = async (id: string) => {
    setBusyId(id);
    try {
      await rollbackPatch({ variables: { id } });
      setSnack("Patch rolled back.");
      await query.refetch();
    } catch (e) {
      setSnack(extractErrorMessage(e));
    } finally {
      setBusyId(null);
    }
  };

  if (query.loading && patches.length === 0) {
    return <LoadingPanel label="Loading patches…" minHeight="100%" />;
  }

  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.base,
        display: "flex",
        flexDirection: "column",
        height: "100%",
        minHeight: 0,
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{
          alignItems: "center",
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.surface,
          px: 1.5,
          py: 1,
        }}
      >
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.secondary, flex: 1 }}
        >
          Patches
        </Typography>
        <StatPill label="patches" value={patches.length} />
        <StatPill label="files" value={stats.files} />
        {stats.applied > 0 ? (
          <StatPill label="applied" value={stats.applied} tone="success" />
        ) : null}
        {stats.pending > 0 ? (
          <StatPill label="pending" value={stats.pending} tone="warning" />
        ) : null}
        {stats.conflicted > 0 ? (
          <StatPill label="conflict" value={stats.conflicted} tone="danger" />
        ) : null}
        <Tooltip title="Refresh" arrow>
          <Button
            size="small"
            onClick={() => void query.refetch()}
            startIcon={<RefreshRounded sx={{ fontSize: 14 }} />}
            sx={{
              color: tokens.color.text.secondary,
              fontFamily: tokens.font.family,
              fontSize: 11,
              fontWeight: 700,
              minWidth: 0,
              px: 1,
              textTransform: "none",
            }}
          >
            Refresh
          </Button>
        </Tooltip>
      </Stack>

      <Box sx={{ flex: 1, minHeight: 0, overflowY: "auto", p: 1.5 }}>
        {patches.length === 0 ? (
          <Stack
            alignItems="center"
            justifyContent="center"
            spacing={1}
            sx={{
              color: tokens.color.text.muted,
              height: "100%",
              p: 4,
              textAlign: "center",
            }}
          >
            <Typography variant="overline" sx={{ color: tokens.color.text.secondary }}>
              No patches yet
            </Typography>
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13, maxWidth: 360 }}>
              The agent is still planning. Every file change goes through a
              patch — they will land here as soon as the first gate evaluates
              them.
            </Typography>
          </Stack>
        ) : (
          <Stack spacing={1}>
            {patches.map((p) => (
              <PatchRow
                key={p.id}
                patch={p}
                busy={busyId === p.id}
                onApply={() => void onApply(p.id)}
                onRollback={() => void onRollback(p.id)}
              />
            ))}
          </Stack>
        )}
      </Box>

      <Snackbar
        open={!!snack}
        autoHideDuration={4000}
        onClose={() => setSnack(null)}
        message={snack ?? ""}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      />
    </Box>
  );
}

function PatchRow({
  patch,
  busy,
  onApply,
  onRollback,
}: {
  patch: Patch;
  busy: boolean;
  onApply: () => void;
  onRollback: () => void;
}) {
  const upper = patch.status.toUpperCase();
  const canApply = upper === "PROPOSED" || upper === "APPROVED";
  const canRollback = upper === "APPLIED";
  const isConflict = upper === "CONFLICTED";
  const title = patch.title ?? `Patch ${patch.id.slice(0, 8)}`;
  const filesTouched = useMemo(
    () => new Set(patch.changes.map((c) => c.path)),
    [patch.changes],
  );
  return (
    <Accordion
      disableGutters
      square
      sx={{
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${
          isConflict ? `${tokens.color.accent.danger}55` : tokens.color.border.subtle
        }`,
        borderRadius: 1,
        boxShadow: "none",
        "&:before": { display: "none" },
        "&.Mui-expanded": { my: 0 },
      }}
    >
      <AccordionSummary
        expandIcon={
          <ExpandMoreRounded sx={{ color: tokens.color.text.muted, fontSize: 18 }} />
        }
        sx={{
          minHeight: 48,
          "& .MuiAccordionSummary-content": { my: 0.75 },
        }}
      >
        <Stack direction="row" spacing={1.25} sx={{ alignItems: "center", width: "100%" }}>
          {isConflict ? (
            <WarningAmberRounded
              sx={{ color: tokens.color.accent.danger, fontSize: 16 }}
            />
          ) : null}
          <Stack spacing={0.25} sx={{ flex: 1, minWidth: 0 }}>
            <Typography
              sx={{
                color: tokens.color.text.primary,
                fontSize: 13,
                fontWeight: 700,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {title}
            </Typography>
            <Typography
              sx={{
                color: tokens.color.text.muted,
                fontFamily: tokens.font.mono,
                fontSize: 10.5,
                letterSpacing: 0.4,
              }}
            >
              {filesTouched.size} file{filesTouched.size === 1 ? "" : "s"} ·{" "}
              {patch.changes.length} change{patch.changes.length === 1 ? "" : "s"} ·{" "}
              {relativeTime(patch.createdAt)}
              {patch.author ? ` · by ${patch.author}` : ""}
              {patch.appliedAt ? ` · applied ${relativeTime(patch.appliedAt)}` : ""}
            </Typography>
          </Stack>
          <StatusBadge status={patch.status} />
        </Stack>
      </AccordionSummary>
      <AccordionDetails sx={{ borderTop: `1px solid ${tokens.color.border.subtle}` }}>
        <Stack spacing={1.5}>
          {patch.summary ? (
            <Typography
              sx={{ color: tokens.color.text.secondary, fontSize: 12.5, lineHeight: 1.55 }}
            >
              {patch.summary}
            </Typography>
          ) : null}
          <Stack spacing={0.75}>
            {patch.changes.map((c, i) => (
              <ChangeRow key={`${c.path}_${i}`} change={c} />
            ))}
          </Stack>
          <Stack direction="row" spacing={1} sx={{ justifyContent: "flex-end" }}>
            {canRollback ? (
              <Tooltip title="Revert this patch on disk" arrow>
                <span>
                  <Button
                    size="small"
                    onClick={onRollback}
                    disabled={busy}
                    startIcon={
                      busy ? (
                        <CircularProgress
                          size={12}
                          thickness={6}
                          sx={{ color: tokens.color.accent.warning }}
                        />
                      ) : (
                        <UndoRounded sx={{ fontSize: 14 }} />
                      )
                    }
                    sx={{
                      border: `1px solid ${tokens.color.accent.warning}55`,
                      color: tokens.color.accent.warning,
                      fontFamily: tokens.font.family,
                      fontSize: 11.5,
                      fontWeight: 700,
                      px: 1.25,
                      textTransform: "none",
                      "&:hover": { bgcolor: `${tokens.color.accent.warning}14` },
                    }}
                  >
                    Roll back
                  </Button>
                </span>
              </Tooltip>
            ) : (
              <Tooltip
                title={
                  upper === "ROLLED_BACK"
                    ? "Already rolled back"
                    : "Roll back only available once the patch has been applied"
                }
                arrow
              >
                <span>
                  <Button
                    size="small"
                    disabled
                    startIcon={<UndoRounded sx={{ fontSize: 14 }} />}
                    sx={{
                      border: `1px solid ${tokens.color.border.subtle}`,
                      color: tokens.color.text.muted,
                      fontFamily: tokens.font.family,
                      fontSize: 11.5,
                      fontWeight: 700,
                      px: 1.25,
                      textTransform: "none",
                    }}
                  >
                    Roll back
                  </Button>
                </span>
              </Tooltip>
            )}
            {canApply ? (
              <Button
                size="small"
                onClick={onApply}
                disabled={busy}
                startIcon={
                  busy ? (
                    <CircularProgress
                      size={12}
                      thickness={6}
                      sx={{ color: tokens.color.text.inverse }}
                    />
                  ) : (
                    <CheckCircleRounded sx={{ fontSize: 14 }} />
                  )
                }
                sx={{
                  bgcolor: tokens.color.accent.violet,
                  color: tokens.color.text.inverse,
                  fontFamily: tokens.font.family,
                  fontSize: 11.5,
                  fontWeight: 700,
                  px: 1.25,
                  textTransform: "none",
                  "&:hover": {
                    bgcolor: tokens.color.accent.violet,
                    filter: "brightness(0.95)",
                  },
                  "&.Mui-disabled": {
                    bgcolor: tokens.color.bg.surfaceHover,
                    color: tokens.color.text.muted,
                  },
                }}
              >
                Apply
              </Button>
            ) : null}
          </Stack>
        </Stack>
      </AccordionDetails>
    </Accordion>
  );
}

function ChangeRow({ change }: { change: Patch["changes"][number] }) {
  const [open, setOpen] = useState(false);
  const c = change;
  const preview = c.replacement ?? c.content ?? "";
  const showPreview = preview.trim().length > 0;
  return (
    <Box
      sx={{
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 0.75,
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{
          alignItems: "center",
          cursor: showPreview ? "pointer" : "default",
          px: 1,
          py: 0.5,
          "&:hover": showPreview ? { bgcolor: tokens.color.bg.surface } : undefined,
        }}
        onClick={() => showPreview && setOpen((v) => !v)}
      >
        <Box
          sx={{
            color: opColor(c.op),
            fontFamily: tokens.font.mono,
            fontSize: 10,
            fontWeight: 800,
            letterSpacing: 0.6,
            minWidth: 96,
            textTransform: "uppercase",
          }}
        >
          {c.op}
        </Box>
        <Typography
          sx={{
            color: tokens.color.text.primary,
            fontFamily: tokens.font.mono,
            fontSize: 11.5,
            flex: 1,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {c.path}
        </Typography>
        {c.symbol ? (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
            }}
          >
            {c.symbol}
          </Typography>
        ) : null}
        {showPreview ? (
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10,
              letterSpacing: 0.4,
              textTransform: "uppercase",
            }}
          >
            {open ? "hide" : "show"}
          </Typography>
        ) : null}
      </Stack>
      {open && showPreview ? (
        <Box
          sx={{
            borderTop: `1px solid ${tokens.color.border.subtle}`,
            maxHeight: 280,
            overflow: "auto",
            px: 1,
            py: 0.75,
          }}
        >
          {c.anchor ? (
            <Box sx={{ mb: 0.75 }}>
              <Typography
                sx={{
                  color: tokens.color.text.muted,
                  fontFamily: tokens.font.mono,
                  fontSize: 9.5,
                  letterSpacing: 0.6,
                  mb: 0.25,
                  textTransform: "uppercase",
                }}
              >
                anchor
              </Typography>
              <PreBlock
                content={c.anchor}
                color={tokens.color.accent.danger}
                bg={`${tokens.color.accent.danger}1a`}
                prefix="-"
              />
            </Box>
          ) : null}
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 9.5,
              letterSpacing: 0.6,
              mb: 0.25,
              textTransform: "uppercase",
            }}
          >
            {c.anchor || c.symbol ? "replacement" : "content"}
          </Typography>
          <PreBlock
            content={preview}
            color={tokens.color.accent.success}
            bg={`${tokens.color.accent.success}1a`}
            prefix="+"
          />
        </Box>
      ) : null}
    </Box>
  );
}

function PreBlock({
  content,
  color,
  bg,
  prefix,
}: {
  content: string;
  color: string;
  bg: string;
  prefix: "+" | "-";
}) {
  const lines = useMemo(() => content.split("\n"), [content]);
  return (
    <Box
      component="pre"
      sx={{
        bgcolor: bg,
        borderRadius: 0.5,
        color,
        fontFamily: tokens.font.mono,
        fontSize: 11,
        lineHeight: 1.55,
        m: 0,
        overflowX: "auto",
        p: 0.75,
      }}
    >
      {lines.map((ln, i) => (
        <Box key={i} component="span" sx={{ display: "block", whiteSpace: "pre" }}>
          <Box
            component="span"
            sx={{ color, opacity: 0.75, mr: 0.75, userSelect: "none" }}
          >
            {prefix}
          </Box>
          {ln.length === 0 ? " " : ln}
        </Box>
      ))}
    </Box>
  );
}

function StatPill({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "success" | "warning" | "danger";
}) {
  const color =
    tone === "success"
      ? tokens.color.accent.success
      : tone === "warning"
      ? tokens.color.accent.warning
      : tone === "danger"
      ? tokens.color.accent.danger
      : tokens.color.text.secondary;
  return (
    <Box
      sx={{
        alignItems: "center",
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 0.75,
        color,
        display: "inline-flex",
        fontFamily: tokens.font.mono,
        fontSize: 10.5,
        fontWeight: 800,
        gap: 0.4,
        letterSpacing: 0.4,
        px: 0.75,
        py: 0.2,
        textTransform: "uppercase",
      }}
    >
      <Box component="span" sx={{ color }}>
        {value}
      </Box>
      <Box component="span" sx={{ color: tokens.color.text.muted }}>
        {label}
      </Box>
    </Box>
  );
}

function opColor(op: string): string {
  switch (op) {
    case "CREATE":
      return tokens.color.accent.success;
    case "DELETE":
      return tokens.color.accent.danger;
    case "REPLACE":
    case "ANCHOR_REPLACE":
    case "SYMBOL_REPLACE":
      return tokens.color.accent.violet;
    case "INSERT_BEFORE":
    case "INSERT_AFTER":
      return tokens.color.accent.sky;
    default:
      return tokens.color.text.muted;
  }
}
