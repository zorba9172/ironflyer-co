"use client";

// SupportBundlePanel — read-only post-execution support bundle.
// Combines preview URL, changed files, gate report, security
// summary, and the next-best-action callout. Consumed by the Bundle
// tab on /execution/[id].

import { LaunchRounded, ShieldOutlined } from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { useExecutionSupportBundleQuery } from "../../lib/gql/__generated__";
import { formatNumber } from "../../lib/format";
import { tokens } from "../../theme";
import { ErrorPanel, LoadingPanel, StatusBadge } from "../cockpit";

export interface SupportBundlePanelProps {
  executionID: string;
}

export function SupportBundlePanel({ executionID }: SupportBundlePanelProps) {
  const { data, loading, error, refetch } = useExecutionSupportBundleQuery({
    variables: { executionID },
    fetchPolicy: "cache-and-network",
  });

  if (loading && !data) {
    return <LoadingPanel label="Loading bundle" />;
  }
  if (error) {
    return (
      <ErrorPanel
        error={error}
        title="Support bundle unavailable"
        onRetry={() => void refetch()}
      />
    );
  }
  const b = data?.executionSupportBundle;
  if (!b) return null;

  return (
    <Stack spacing={2}>
      {/* Preview / production URLs */}
      <Card sx={{ p: 2.5 }}>
        <Typography
          variant="overline"
          sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
        >
          Live surfaces
        </Typography>
        <Stack direction="row" spacing={1.5} sx={{ mt: 1 }} flexWrap="wrap">
          {b.previewURL ? (
            <Button
              component="a"
              href={b.previewURL}
              target="_blank"
              rel="noopener noreferrer"
              variant="outlined"
              endIcon={<LaunchRounded fontSize="small" />}
              sx={{
                borderColor: tokens.color.border.strong,
                color: tokens.color.text.primary,
                fontFamily: tokens.font.mono,
              }}
            >
              Open preview
            </Button>
          ) : (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
              No preview URL yet.
            </Typography>
          )}
          {b.productionURL && (
            <Button
              component="a"
              href={b.productionURL}
              target="_blank"
              rel="noopener noreferrer"
              variant="contained"
              color="primary"
              endIcon={<LaunchRounded fontSize="small" />}
              sx={{ fontFamily: tokens.font.mono }}
            >
              Open production
            </Button>
          )}
        </Stack>
        <Stack direction="row" spacing={2} sx={{ mt: 2 }} flexWrap="wrap">
          <StatusBadge status={b.status} />
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5, fontFamily: tokens.font.mono }}>
            {formatNumber(b.patchCount)} patches · {formatNumber(b.changedFiles.length)} files changed
          </Typography>
        </Stack>
      </Card>

      {/* Gate report */}
      <Card sx={{ p: 2.5 }}>
        <Stack
          direction="row"
          justifyContent="space-between"
          alignItems="baseline"
          sx={{ mb: 1.25 }}
        >
          <Typography sx={{ fontWeight: 800, fontSize: 16 }}>Gate report</Typography>
          <Typography
            sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 12 }}
          >
            completion {b.gateReport.completionScore.toFixed(2)}
          </Typography>
        </Stack>
        <Stack spacing={0.75}>
          {b.gateReport.stages.length === 0 && (
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
              No gate stages recorded.
            </Typography>
          )}
          {b.gateReport.stages.map((s) => (
            <Stack
              key={s.name}
              direction="row"
              alignItems="center"
              spacing={1.25}
              sx={{
                px: 1.25,
                py: 0.75,
                bgcolor: tokens.color.bg.inset,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: 1,
              }}
            >
              <StatusBadge status={s.status} />
              <Typography sx={{ flex: 1, fontFamily: tokens.font.mono, fontSize: 13 }}>
                {s.name}
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 12,
                  color: s.issuesCount > 0 ? tokens.color.accent.warning : tokens.color.text.muted,
                }}
              >
                {s.issuesCount} issues
              </Typography>
            </Stack>
          ))}
        </Stack>
      </Card>

      {/* Changed files */}
      <Card sx={{ p: 2.5 }}>
        <Typography sx={{ fontWeight: 800, fontSize: 16, mb: 1 }}>Changed files</Typography>
        {b.changedFiles.length === 0 ? (
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 13 }}>
            No files reported.
          </Typography>
        ) : (
          <Box
            component="ul"
            sx={{
              m: 0,
              p: 0,
              listStyle: "none",
              maxHeight: 220,
              overflow: "auto",
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: 1,
              bgcolor: tokens.color.bg.inset,
            }}
          >
            {b.changedFiles.map((path, i) => (
              <Typography
                key={`${path}-${i}`}
                component="li"
                sx={{
                  px: 1.25,
                  py: 0.5,
                  fontFamily: tokens.font.mono,
                  fontSize: 12.5,
                  color: tokens.color.text.primary,
                  borderBottom:
                    i === b.changedFiles.length - 1
                      ? "none"
                      : `1px solid ${tokens.color.border.subtle}`,
                }}
              >
                {path}
              </Typography>
            ))}
          </Box>
        )}
      </Card>

      {/* Next-best-action */}
      {b.nextBestAction && (
        <Card sx={{ p: 2.5, borderLeft: `3px solid ${tokens.color.accent.violet}` }}>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.accent.violet, letterSpacing: 1.2 }}
          >
            {b.nextBestAction.kind}
          </Typography>
          <Typography sx={{ mt: 0.5, fontWeight: 800, fontSize: 18 }}>
            {b.nextBestAction.title}
          </Typography>
          <Typography sx={{ mt: 0.5, color: tokens.color.text.secondary, fontSize: 13.5 }}>
            {b.nextBestAction.reason}
          </Typography>
          {b.nextBestAction.cta && (
            <Typography
              sx={{
                mt: 1,
                fontFamily: tokens.font.mono,
                fontSize: 12.5,
                color: tokens.color.accent.violet,
              }}
            >
              → {b.nextBestAction.cta}
            </Typography>
          )}
        </Card>
      )}

      {/* Security shortcut */}
      <Card sx={{ p: 2.5 }}>
        <Stack
          direction="row"
          alignItems="center"
          spacing={1.5}
          justifyContent="space-between"
        >
          <Stack direction="row" spacing={1.25} alignItems="center">
            <ShieldOutlined sx={{ color: tokens.color.accent.sky }} />
            <Box>
              <Typography sx={{ fontWeight: 800, fontSize: 15 }}>Security report</Typography>
              <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5 }}>
                {(b.securityReport.passRate * 100).toFixed(0)}% pass rate ·{" "}
                {b.securityReport.findings.length} findings ·{" "}
                {b.securityReport.blockedDeploy ? "deploy blocked" : "deploy clear"}
              </Typography>
            </Box>
          </Stack>
          <Button
            component={Link}
            href={`/execution/${executionID}/security`}
            variant="outlined"
            size="small"
            sx={{
              borderColor: tokens.color.border.strong,
              color: tokens.color.text.primary,
              fontFamily: tokens.font.mono,
            }}
          >
            View full report
          </Button>
        </Stack>
      </Card>
    </Stack>
  );
}
