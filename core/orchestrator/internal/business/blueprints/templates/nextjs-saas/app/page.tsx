import Container from "@mui/material/Container";
import Typography from "@mui/material/Typography";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Link from "next/link";

export default function LandingPage() {
  return (
    <Container maxWidth="md">
      <Box sx={{ py: 12, textAlign: "center" }}>
        <Typography variant="h2" component="h1" gutterBottom>
          Ship your SaaS
        </Typography>
        <Typography variant="h6" color="text.secondary" sx={{ mb: 5 }}>
          Multi-tenant auth, billing, and a gated dashboard — wired and
          ready. Edit <code>app/page.tsx</code> to customise the landing.
        </Typography>
        <Stack direction="row" spacing={2} justifyContent="center">
          <Button
            variant="contained"
            size="large"
            component={Link}
            href="/dashboard"
          >
            Open dashboard
          </Button>
          <Button
            variant="outlined"
            size="large"
            component={Link}
            href="/api/auth/signin"
          >
            Sign in
          </Button>
        </Stack>
      </Box>
    </Container>
  );
}
