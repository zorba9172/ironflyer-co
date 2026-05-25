"use client";

// SecurityReportHeader — status pill + overall score + blocked-deploy
// chip. Renders above the FindingsTable on /execution/[id]/security.

import { Card, Chip, Stack, Typography } from "@mui/material";
import type { ExecutionSecurityReportQuery } from "../../lib/gql/__generated__";
import { formatDateTime } from "../../lib/format";
import { tokens } from "../../theme";
import { StatusBadge } from "../cockpit";

export interface SecurityReportHeaderProps {
  report: ExecutionSecurityReportQuery["executionSecurityReport"];
}

export function SecurityReportHeader({ report }: SecurityReportHeaderProps) {
  const score = Math.round(Math.max(0, Math.min(1, report.overallScore)) * 100);
  const scoreColor =
    score >= 80
      ? tokens.color.accent.success
      : score >= 60
        ? tokens.color.accent.warning
        : tokens.color.accent.danger;

  return (
    <Card sx={{ p: 2.5 }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={2}
        justifyContent="space-between"
        alignItems={{ md: "center" }}
      >
        <Stack direction="row" spacing={1.5} alignItems="center" flexWrap="wrap">
          <StatusBadge status={report.status} />
          {report.blockedDeploy && (
            <Chip
              size="small"
              label="DEPLOY BLOCKED"
              sx={{
                bgcolor: `${tokens.color.accent.danger}1f`,
                color: tokens.color.accent.danger,
                border: `1px solid ${tokens.color.accent.danger}66`,
                fontFamily: tokens.font.mono,
                fontWeight: 800,
                fontSize: 10.5,
                letterSpacing: 0.8,
                height: 22,
                borderRadius: 0.75,
                "& .MuiChip-label": { px: 1 },
              }}
            />
          )}
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 11.5,
            }}
          >
            generated {formatDateTime(report.generatedAt)}
          </Typography>
        </Stack>
        <Stack direction="row" spacing={1} alignItems="baseline">
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontWeight: 800,
              fontSize: 32,
              lineHeight: 1,
              color: scoreColor,
            }}
          >
            {score}%
          </Typography>
          <Typography
            variant="overline"
            sx={{ color: tokens.color.text.muted, letterSpacing: 1.2 }}
          >
            overall score
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}
