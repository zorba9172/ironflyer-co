import { auth } from "@/app/api/auth/[...nextauth]/route";
import { redirect } from "next/navigation";
import Container from "@mui/material/Container";
import Typography from "@mui/material/Typography";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Button from "@mui/material/Button";

export default async function DashboardPage() {
  const session = await auth();
  if (!session?.user) {
    redirect("/api/auth/signin?callbackUrl=/dashboard");
  }

  return (
    <Container maxWidth="lg">
      <Box sx={{ py: 6 }}>
        <Typography variant="h4" component="h1" gutterBottom>
          Dashboard
        </Typography>
        <Typography color="text.secondary" sx={{ mb: 4 }}>
          Signed in as {session.user.email ?? session.user.name ?? "user"}.
        </Typography>
        <Paper sx={{ p: 4 }}>
          <Typography variant="h6" gutterBottom>
            Subscription
          </Typography>
          <Typography color="text.secondary" sx={{ mb: 2 }}>
            Start a Stripe Checkout session to enable the paid tier.
          </Typography>
          <form action="/api/checkout" method="post">
            <Button type="submit" variant="contained">
              Upgrade to Pro
            </Button>
          </form>
        </Paper>
      </Box>
    </Container>
  );
}
