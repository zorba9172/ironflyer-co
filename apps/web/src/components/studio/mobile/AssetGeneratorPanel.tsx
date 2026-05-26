"use client";

// AssetGeneratorPanel — single-screen surface that turns one square
// logo + a background colour into the full Android + iOS + Expo asset
// bundle (mipmaps, adaptive icon, AppIcon.appiconset, splashes,
// favicon, LaunchScreen.storyboard).
//
// Mobile apps need ~20 icon/splash variants. Generating them manually
// is the #1 friction point operators hit; Lovable/Bolt do not solve
// this. The panel is intentionally compact: file picker on the left,
// colour pickers + platform toggles in the centre, generated-manifest
// grid on the right. The grid is the visual mirror of the orchestrator
// state — every entry maps to a concrete generated asset on disk.

import { gql, useMutation } from "@apollo/client";
import {
  Box,
  Button,
  Checkbox,
  CircularProgress,
  FormControlLabel,
  Stack,
  Typography,
} from "@mui/material";
import { useCallback, useMemo, useRef, useState } from "react";
import { tokens } from "../../../theme";

// GraphQL mutation. Kept local (rather than added to operations/) so
// the panel ships without forcing a codegen run; once codegen is
// rerun, typed hooks can replace this and the manual `unknown` types
// disappear.
const GENERATE_MOBILE_ASSETS = gql`
  mutation GenerateMobileAssets($input: GenerateMobileAssetsInput!) {
    generateMobileAssets(input: $input) {
      filesCount
      totalBytes
      generatedAt
      entries {
        path
        width
        height
        sizeBytes
        purpose
      }
    }
  }
`;

interface GeneratedEntry {
  path: string;
  width: number;
  height: number;
  sizeBytes: number;
  purpose: string;
}

interface GenerateResult {
  filesCount: number;
  totalBytes: number;
  generatedAt: string;
  entries: GeneratedEntry[];
}

type Platform = "android" | "ios" | "expo";

export interface AssetGeneratorPanelProps {
  projectId: string;
}

// readFileAsBase64 strips the data-URL prefix so the resolver can
// base64-decode the raw payload without parsing a MIME header.
async function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("read failed"));
    reader.onload = () => {
      const result = String(reader.result ?? "");
      const comma = result.indexOf(",");
      resolve(comma >= 0 ? result.slice(comma + 1) : result);
    };
    reader.readAsDataURL(file);
  });
}

function humanBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

function groupForEntry(entry: GeneratedEntry): "Android" | "iOS" | "Expo" {
  if (entry.path.startsWith("android/")) return "Android";
  if (entry.path.startsWith("ios/")) return "iOS";
  return "Expo";
}

export function AssetGeneratorPanel({ projectId }: AssetGeneratorPanelProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [logoFile, setLogoFile] = useState<File | null>(null);
  const [logoPreview, setLogoPreview] = useState<string | null>(null);
  const [backgroundColor, setBackgroundColor] = useState("#050612");
  const [splashFgColor, setSplashFgColor] = useState("#ffffff");
  const [appName, setAppName] = useState("");
  const [platforms, setPlatforms] = useState<Record<Platform, boolean>>({
    android: true,
    ios: true,
    expo: true,
  });
  const [result, setResult] = useState<GenerateResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [runGenerate, { loading }] = useMutation<
    { generateMobileAssets: GenerateResult },
    {
      input: {
        projectId: string;
        logoPngBase64: string;
        backgroundColor: string;
        splashForegroundColor?: string;
        platforms: string[];
      };
    }
  >(GENERATE_MOBILE_ASSETS);

  const selectedPlatforms = useMemo(
    () =>
      (Object.entries(platforms) as Array<[Platform, boolean]>)
        .filter(([, on]) => on)
        .map(([k]) => k),
    [platforms],
  );

  const onPickFile = useCallback((file: File) => {
    setLogoFile(file);
    const url = URL.createObjectURL(file);
    setLogoPreview(url);
  }, []);

  const onSubmit = useCallback(async () => {
    setError(null);
    if (!logoFile) {
      setError("Pick a square PNG logo first.");
      return;
    }
    if (selectedPlatforms.length === 0) {
      setError("Choose at least one platform.");
      return;
    }
    try {
      const base64 = await readFileAsBase64(logoFile);
      const r = await runGenerate({
        variables: {
          input: {
            projectId,
            logoPngBase64: base64,
            backgroundColor,
            splashForegroundColor: splashFgColor || undefined,
            platforms: selectedPlatforms,
          },
        },
      });
      const out = r.data?.generateMobileAssets;
      if (out) setResult(out);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
    }
  }, [
    backgroundColor,
    logoFile,
    projectId,
    runGenerate,
    selectedPlatforms,
    splashFgColor,
  ]);

  // Group manifest entries by platform for the visual mirror.
  const grouped = useMemo(() => {
    const g: Record<string, GeneratedEntry[]> = {};
    for (const e of result?.entries ?? []) {
      const key = groupForEntry(e);
      if (!g[key]) g[key] = [];
      g[key].push(e);
    }
    return g;
  }, [result]);

  // Read app name from the file-name hint when nothing is set yet.
  const onPick = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const f = e.target.files?.[0];
      if (!f) return;
      onPickFile(f);
      if (!appName) {
        const stem = f.name.replace(/\.[^.]+$/, "");
        setAppName(stem.replace(/[-_]+/g, " "));
      }
    },
    [appName, onPickFile],
  );

  return (
    <Stack spacing={2}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={2}
        alignItems="stretch"
      >
        <Box
          sx={{
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            flex: 1,
            minWidth: 0,
            p: 2,
          }}
        >
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 1.2,
              mb: 1.5,
              textTransform: "uppercase",
            }}
          >
            Source logo
          </Typography>
          <Stack direction="row" spacing={2} alignItems="center">
            <Box
              sx={{
                alignItems: "center",
                aspectRatio: "1 / 1",
                bgcolor: tokens.color.bg.inset,
                border: `1px dashed ${tokens.color.border.strong}`,
                borderRadius: 1,
                color: tokens.color.text.muted,
                cursor: "pointer",
                display: "flex",
                fontFamily: tokens.font.mono,
                fontSize: 10.5,
                justifyContent: "center",
                letterSpacing: 1.2,
                overflow: "hidden",
                textTransform: "uppercase",
                width: 120,
              }}
              onClick={() => fileInputRef.current?.click()}
            >
              {logoPreview ? (
                <Box
                  component="img"
                  src={logoPreview}
                  alt="Source logo"
                  sx={{ display: "block", height: "100%", width: "100%" }}
                />
              ) : (
                "Click to upload"
              )}
            </Box>
            <Stack spacing={0.75} sx={{ minWidth: 0 }}>
              <Typography
                sx={{ color: tokens.color.text.primary, fontSize: 13 }}
              >
                {logoFile?.name ?? "PNG or JPG, square, at least 1024×1024"}
              </Typography>
              <Typography
                sx={{
                  color: tokens.color.text.secondary,
                  fontSize: 12,
                  lineHeight: 1.55,
                }}
              >
                Sub-1024 logos render aliased at the 1024×1024 marketing
                slot. Use the raw vector export from your design tool.
              </Typography>
            </Stack>
          </Stack>
          <Box
            component="input"
            ref={fileInputRef}
            type="file"
            accept="image/png,image/jpeg"
            onChange={onPick}
            sx={{ display: "none" }}
          />
        </Box>

        <Box
          sx={{
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: 1,
            flex: 1,
            minWidth: 0,
            p: 2,
          }}
        >
          <Typography
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 1.2,
              mb: 1.5,
              textTransform: "uppercase",
            }}
          >
            Colors & platforms
          </Typography>
          <Stack spacing={1.5}>
            <ColorRow
              label="Background"
              value={backgroundColor}
              onChange={setBackgroundColor}
            />
            <ColorRow
              label="Splash foreground"
              value={splashFgColor}
              onChange={setSplashFgColor}
            />
            <Stack direction="row" spacing={1.5} flexWrap="wrap">
              {(["android", "ios", "expo"] as Platform[]).map((p) => (
                <FormControlLabel
                  key={p}
                  control={
                    <Checkbox
                      size="small"
                      checked={platforms[p]}
                      onChange={(_, v) =>
                        setPlatforms((cur) => ({ ...cur, [p]: v }))
                      }
                      sx={{
                        color: tokens.color.border.strong,
                        "&.Mui-checked": {
                          color: tokens.color.accent.violet,
                        },
                      }}
                    />
                  }
                  label={
                    <Typography
                      sx={{
                        color: tokens.color.text.primary,
                        fontSize: 13,
                        textTransform: "capitalize",
                      }}
                    >
                      {p}
                    </Typography>
                  }
                  sx={{ m: 0 }}
                />
              ))}
            </Stack>
          </Stack>
        </Box>
      </Stack>

      <Stack
        direction="row"
        spacing={1.5}
        alignItems="center"
        justifyContent="space-between"
      >
        <Typography
          sx={{ color: tokens.color.text.secondary, fontSize: 12.5 }}
        >
          {result
            ? `${result.filesCount} files · ${humanBytes(result.totalBytes)}`
            : "Generate to fill the manifest grid below."}
        </Typography>
        <Button
          variant="contained"
          color="primary"
          onClick={onSubmit}
          disabled={loading || !logoFile}
          startIcon={
            loading ? <CircularProgress size={14} color="inherit" /> : null
          }
        >
          {loading ? "Generating…" : "Generate assets"}
        </Button>
      </Stack>

      {error ? (
        <Box
          sx={{
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.accent.coral}`,
            borderRadius: 1,
            color: tokens.color.accent.coral,
            fontFamily: tokens.font.mono,
            fontSize: 12,
            p: 1.5,
          }}
        >
          {error}
        </Box>
      ) : null}

      {result ? (
        <Stack spacing={2}>
          {Object.entries(grouped).map(([platform, entries]) => (
            <Box
              key={platform}
              sx={{
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: 1,
                p: 2,
              }}
            >
              <Stack
                direction="row"
                spacing={1}
                alignItems="baseline"
                sx={{ mb: 1.5 }}
              >
                <Typography
                  sx={{
                    color: tokens.color.text.primary,
                    fontSize: 13,
                    fontWeight: 700,
                  }}
                >
                  {platform}
                </Typography>
                <Typography
                  sx={{
                    color: tokens.color.text.muted,
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                  }}
                >
                  {entries.length} files
                </Typography>
              </Stack>
              <Box
                sx={{
                  display: "grid",
                  gap: 1,
                  gridTemplateColumns:
                    "repeat(auto-fill, minmax(180px, 1fr))",
                }}
              >
                {entries.map((e) => (
                  <Box
                    key={e.path}
                    sx={{
                      bgcolor: tokens.color.bg.inset,
                      border: `1px solid ${tokens.color.border.subtle}`,
                      borderRadius: 1,
                      p: 1,
                    }}
                  >
                    <Typography
                      sx={{
                        color: tokens.color.accent.violet,
                        fontFamily: tokens.font.mono,
                        fontSize: 10.5,
                        letterSpacing: 0.8,
                        textTransform: "uppercase",
                      }}
                    >
                      {e.purpose}
                    </Typography>
                    <Typography
                      sx={{
                        color: tokens.color.text.primary,
                        fontFamily: tokens.font.mono,
                        fontSize: 11.5,
                        my: 0.5,
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}
                      title={e.path}
                    >
                      {e.path.split("/").pop()}
                    </Typography>
                    <Typography
                      sx={{
                        color: tokens.color.text.muted,
                        fontFamily: tokens.font.mono,
                        fontSize: 10.5,
                      }}
                    >
                      {e.width > 0 && e.height > 0
                        ? `${e.width}×${e.height} · `
                        : ""}
                      {humanBytes(e.sizeBytes)}
                    </Typography>
                  </Box>
                ))}
              </Box>
            </Box>
          ))}
        </Stack>
      ) : null}
    </Stack>
  );
}

function ColorRow({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (next: string) => void;
}) {
  // The native colour input value is the only place a raw hex string
  // is legal in this component — those hexes are the operator's chosen
  // colour, not a design token. The chrome around it (label, surface,
  // border) goes through tokens.color.* per the design-reference law.
  return (
    <Stack direction="row" spacing={1.5} alignItems="center">
      <Typography
        sx={{
          color: tokens.color.text.secondary,
          fontSize: 12.5,
          minWidth: 150,
        }}
      >
        {label}
      </Typography>
      <Box
        component="input"
        type="color"
        value={value}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          onChange(e.target.value)
        }
        sx={{
          background: "transparent",
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          cursor: "pointer",
          height: 28,
          padding: 0,
          width: 48,
        }}
      />
      <Box
        component="input"
        type="text"
        value={value}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          onChange(e.target.value)
        }
        spellCheck={false}
        sx={{
          background: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          color: tokens.color.text.primary,
          fontFamily: tokens.font.mono,
          fontSize: 12.5,
          outline: "none",
          padding: "4px 8px",
          width: 120,
          "&:focus": { borderColor: tokens.color.accent.violet },
        }}
      />
    </Stack>
  );
}
