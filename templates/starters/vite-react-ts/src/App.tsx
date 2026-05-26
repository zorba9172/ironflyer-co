import { Box, Button, Card, CardContent, Chip, Stack, Typography } from "@mui/material";
import { useState } from "react";

export default function App() {
  const [count, setCount] = useState(0);
  return (
    <Box sx={{ minHeight: "100vh", display: "grid", placeItems: "center", px: 3 }}>
      <Card sx={{ maxWidth: 560, width: "100%", bgcolor: "#161617", color: "#f5f5f4" }}>
        <CardContent>
          <Stack spacing={2}>
            <Chip
              label="Ironflyer scaffold {{TODAY}}"
              size="small"
              sx={{ alignSelf: "flex-start", bgcolor: "#c6ff3a", color: "#0b0b0c", fontWeight: 600 }}
            />
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              Welcome to {{PROJECT_NAME}}
            </Typography>
            <Typography variant="body1" sx={{ opacity: 0.8 }}>
              This Vite + React + MUI app is live. HMR is on — the Ironflyer Coder will edit it
              in place as gates progress.
            </Typography>
            <Stack direction="row" spacing={1}>
              <Button
                variant="contained"
                onClick={() => setCount((c) => c + 1)}
                sx={{ bgcolor: "#c6ff3a", color: "#0b0b0c", "&:hover": { bgcolor: "#b0ec1f" } }}
              >
                count is {count}
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
