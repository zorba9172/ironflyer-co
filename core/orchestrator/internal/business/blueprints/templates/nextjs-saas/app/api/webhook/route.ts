import { NextRequest, NextResponse } from "next/server";
import Stripe from "stripe";
import { PrismaClient } from "@prisma/client";

const stripe = new Stripe(process.env.STRIPE_SECRET_KEY ?? "", {
  apiVersion: "2024-09-30.acacia",
});
const prisma = new PrismaClient();

export const runtime = "nodejs";

export async function POST(req: NextRequest) {
  const signature = req.headers.get("stripe-signature");
  const secret = process.env.STRIPE_WEBHOOK_SECRET;
  if (!signature || !secret) {
    return NextResponse.json(
      { error: "missing stripe signature or webhook secret" },
      { status: 400 },
    );
  }

  const payload = await req.text();
  let event: Stripe.Event;
  try {
    event = stripe.webhooks.constructEvent(payload, signature, secret);
  } catch (err) {
    const message = err instanceof Error ? err.message : "invalid signature";
    return NextResponse.json({ error: message }, { status: 400 });
  }

  switch (event.type) {
    case "checkout.session.completed": {
      const session = event.data.object as Stripe.Checkout.Session;
      const email = session.customer_email ?? session.metadata?.userEmail;
      if (email) {
        await prisma.user.update({
          where: { email },
          data: {
            subscription: {
              upsert: {
                create: {
                  status: "active",
                  stripeCustomerId: String(session.customer ?? ""),
                  stripeSubId: String(session.subscription ?? ""),
                },
                update: {
                  status: "active",
                  stripeCustomerId: String(session.customer ?? ""),
                  stripeSubId: String(session.subscription ?? ""),
                },
              },
            },
          },
        });
      }
      break;
    }
    case "customer.subscription.deleted": {
      const sub = event.data.object as Stripe.Subscription;
      await prisma.subscription.updateMany({
        where: { stripeSubId: sub.id },
        data: { status: "cancelled" },
      });
      break;
    }
    default:
      break;
  }

  return NextResponse.json({ received: true });
}
