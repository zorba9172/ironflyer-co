# Next.js SaaS Starter

Multi-tenant SaaS scaffold: Next.js 15 App Router + MUI 6 + Prisma +
NextAuth v5 (credentials) + Stripe Checkout + Zod.

## What you get

- App Router with MUI 6 cache provider wired through `app/layout.tsx`.
- A landing page at `/` and a session-gated dashboard at `/dashboard`.
- NextAuth v5 credentials provider backed by Prisma + bcrypt.
- Stripe Checkout creation at `POST /api/checkout` and a verified
  webhook receiver at `POST /api/webhook` that upserts the
  `Subscription` row on `checkout.session.completed`.
- Prisma schema with `Tenant`, `User`, `Subscription`, plus the
  NextAuth `Account` / `Session` / `VerificationToken` tables.

## Quick start

```bash
cp .env.example .env
# fill DATABASE_URL, NEXTAUTH_SECRET, STRIPE_*
npm install
npx prisma migrate deploy
npm run dev
```

## Configure Stripe

1. Create a recurring price in the Stripe dashboard, copy the price ID
   into `STRIPE_PRICE_ID`.
2. `stripe listen --forward-to localhost:3000/api/webhook` and paste
   the printed webhook secret into `STRIPE_WEBHOOK_SECRET`.
3. Visit `/dashboard` while signed in and click *Upgrade to Pro*.

## Auth

The credentials provider expects a `User.passwordHash` (bcrypt). Add a
sign-up route or seed a user with
`bcrypt.hash('hunter2', 10)` before signing in.
