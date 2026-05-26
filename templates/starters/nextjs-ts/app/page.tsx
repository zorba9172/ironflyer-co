"use client";

import { Box, Button, Card, CardContent, Chip, Stack, Typography } from "@mui/material";
import { useEffect, useState } from "react";

export default function Home() {
  const [pong, setPong] = useState<string>("");

  useEffect(() => {
    fetch("/api/hello")
      .then((r) => r.json())
      .then((d) => setPong(d.message ?? ""))
      .catch(() => setPong("(api unreachable)"));
  }, []);

  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "grid",
        placeItems: "center",
        px: 3,
      }}
    >
      <Card sx={{ maxWidth: 560, width: "100%", bgcolor: "#161617", color: "#f5f5f4" }}>
        <CardContent>
          <Stack spacing={2}>
            <Chip label="Ironflyer scaffold {{TODAY}}" size="small" sx={{ alignSelf: "flex-start", bgcolor: "#c6ff3a", color: "#0b0b0c", fontWeight: 600 }} />
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              Welcome to {{PROJECT_NAME}}
            </Typography>
            <Typography variant="body1" sx={{ opacity: 0.8 }}>
              This is the live preview of your project. The Ironflyer finisher will keep editing
              this app through enforced gates until it ships.
            </Typography>
            <Box sx={{ p: 2, borderRadius: 1, bgcolor: "#0b0b0c", fontFamily: "monospace", fontSize: 13 }}>
              GET /api/hello → {pong || "…"}
            </Box>
            <Stack direction="row" spacing={1}>
              <Button variant="contained" sx={{ bgcolor: "#c6ff3a", color: "#0b0b0c", "&:hover": { bgcolor: "#b0ec1f" } }}>
                Continue
              </Button>
              <Button variant="outlined" sx={{ borderColor: "#3a3a3c", color: "#f5f5f4" }}>
                View gates
              </Button>
            </Stack>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  );
}
