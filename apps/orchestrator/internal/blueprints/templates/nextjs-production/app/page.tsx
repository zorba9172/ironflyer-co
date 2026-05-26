import Container from "@mui/material/Container";
import Typography from "@mui/material/Typography";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";

export default function HomePage() {
  return (
    <Container maxWidth="md">
      <Box sx={{ py: 10, textAlign: "center" }}>
        <Typography variant="h3" component="h1" gutterBottom>
          Ironflyer Next.js Production App
        </Typography>
        <Typography variant="body1" color="text.secondary" sx={{ mb: 4 }}>
          App Router + MUI 6 + Prisma + Postgres. Edit{" "}
          <code>app/page.tsx</code> to begin.
        </Typography>
        <Button variant="contained" href="/api/health">
          Check API
        </Button>
      </Box>
    </Container>
  );
}
