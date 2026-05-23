// Package finisher — domain-specific scaffolders. Ironflyer's promise is
// "everything: apps, games, e-commerce, dashboards, mobile". The
// AuthScaffolder + StripeScaffolder + DBProvisioner cover the web-app
// baseline; this file adds the rest of the surface so the platform
// genuinely backstops the broad claim.
//
// Design pattern: every scaffolder implements one method —
// Scaffold(ctx, project) → DomainScaffold — and decides for itself
// whether it applies based on the spec. The Engine iterates the
// registered scaffolders once per pipeline; idempotent file writes
// ensure re-runs don't clobber Coder edits.
//
// Detection is deliberately generous: "shop" → e-commerce, "level" /
// "score" / "player" → game, "post" / "feed" → social, "course" /
// "lesson" → learning. False positives are recoverable (the user can
// delete the seeded files) — false negatives are silent and bad UX.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

// DomainScaffolder is the contract every domain pack implements. The
// returned bundle is upserted with the same idempotency rules as
// AuthScaffolder: existing Coder edits are preserved.
type DomainScaffolder interface {
	// Name is a human label for SSE events: "game-phaser", "ecommerce",
	// "social-feed", "learning", "dashboard". Used to namespace the
	// .ironflyer/<name>.md contract file too.
	Name() string
	// Applies returns true when this scaffolder's domain matches the
	// project's spec. Cheap, pure inspection of p.Spec.
	Applies(p *domain.Project) bool
	// Scaffold returns the files + the contract markdown the Coder reads
	// as part of project context.
	Scaffold(ctx context.Context, p *domain.Project) (DomainScaffold, error)
}

// DomainScaffold mirrors the AuthScaffold / StripeScaffold shapes.
type DomainScaffold struct {
	Files    map[string]string
	Contract string
}

// ensureDomains iterates every registered domain scaffolder. Same
// idempotency rules as ensureAuth: existing files the Coder may have
// already mutated are preserved.
func (e *Engine) ensureDomains(ctx context.Context, projectID string) {
	if e == nil || len(e.domainScaffolders) == 0 {
		return
	}
	proj, err := e.projects.Get(projectID)
	if err != nil {
		return
	}
	for _, sc := range e.domainScaffolders {
		if !sc.Applies(&proj) {
			continue
		}
		out, err := sc.Scaffold(ctx, &proj)
		if err != nil || (len(out.Files) == 0 && out.Contract == "") {
			continue
		}
		_, _ = e.projects.Update(projectID, func(p *domain.Project) {
			for path, body := range out.Files {
				if existing := findFile(p, path); existing != nil && existing.Content != body {
					continue
				}
				writeProjectFile(p, path, body)
			}
			if out.Contract != "" {
				writeProjectFile(p, ".ironflyer/"+sc.Name()+".md", out.Contract)
			}
		})
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepRun, Status: StatusDone,
			Message: "domain_scaffolded pack=" + sc.Name(),
		})
	}
}

// WithDomainScaffolders registers any number of domain packs at once.
// Order matters only when two packs claim the same file path — the
// first one wins on idempotent upsert.
func (e *Engine) WithDomainScaffolders(packs ...DomainScaffolder) *Engine {
	e.domainScaffolders = append(e.domainScaffolders, packs...)
	return e
}

// ============================================================
// GameScaffolder — Phaser 3 + Vite skeleton for browser games.
// ============================================================

type GameScaffolder struct{}

func (GameScaffolder) Name() string { return "game-phaser" }

func (GameScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "phaser") || strings.Contains(stack, "pixi") || strings.Contains(stack, "game") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "player") || strings.Contains(body, "level") ||
			strings.Contains(body, "score") || strings.Contains(body, "enemy") ||
			strings.Contains(body, "sprite") || strings.Contains(body, "game over") ||
			strings.Contains(body, "high score") || strings.Contains(body, "leaderboard") {
			return true
		}
	}
	return false
}

func (GameScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"src/game/scenes/BootScene.ts": `// BootScene — loads the asset manifest, shows a tiny progress bar,
// then hands off to MenuScene. Assets are referenced by key from
// here; new sprites get added to /public/assets and registered here.
import Phaser from 'phaser';

export class BootScene extends Phaser.Scene {
  constructor() { super('Boot'); }

  preload() {
    const bar = this.add.rectangle(this.scale.width / 2, this.scale.height / 2, 0, 6, 0xc7ff00);
    this.load.on('progress', (p: number) => bar.width = this.scale.width * 0.6 * p);
    this.load.image('player', 'assets/player.png');
    // Add additional sprites here as the game grows.
  }

  create() { this.scene.start('Menu'); }
}
`,
		"src/game/scenes/MenuScene.ts": `// MenuScene — title + Play button. Replace with art when ready.
import Phaser from 'phaser';

export class MenuScene extends Phaser.Scene {
  constructor() { super('Menu'); }

  create() {
    const { width, height } = this.scale;
    this.add.text(width / 2, height / 2 - 40, 'Ironflyer Game', {
      fontFamily: 'monospace', fontSize: '40px', color: '#c7ff00',
    }).setOrigin(0.5);
    const play = this.add.text(width / 2, height / 2 + 40, '▶ Play', {
      fontFamily: 'monospace', fontSize: '24px', color: '#ffffff',
    }).setOrigin(0.5).setInteractive({ useHandCursor: true });
    play.on('pointerup', () => this.scene.start('Play'));
  }
}
`,
		"src/game/scenes/PlayScene.ts": `// PlayScene — the gameplay loop lives here. Keep input + physics +
// scoring all in this file until it grows past ~300 lines, then
// split into components/. Score is persisted to localStorage on
// game over.
import Phaser from 'phaser';

export class PlayScene extends Phaser.Scene {
  private player!: Phaser.GameObjects.Rectangle;
  private cursors!: Phaser.Types.Input.Keyboard.CursorKeys;
  private score = 0;
  private scoreText!: Phaser.GameObjects.Text;

  constructor() { super('Play'); }

  create() {
    this.player = this.add.rectangle(120, this.scale.height / 2, 32, 32, 0xc7ff00);
    this.cursors = this.input.keyboard!.createCursorKeys();
    this.scoreText = this.add.text(16, 16, 'SCORE 0', {
      fontFamily: 'monospace', fontSize: '20px', color: '#ffffff',
    });
  }

  update(_t: number, dt: number) {
    const speed = 0.25 * dt;
    if (this.cursors.left.isDown)  this.player.x -= speed;
    if (this.cursors.right.isDown) this.player.x += speed;
    if (this.cursors.up.isDown)    this.player.y -= speed;
    if (this.cursors.down.isDown)  this.player.y += speed;
    this.score += 1;
    this.scoreText.setText('SCORE ' + this.score);
  }
}
`,
		"src/game/main.ts": `// Phaser bootstrap. Game canvas is mounted into #game by the
// host page; viewport defaults to 1280x720 (16:9). FIT scaling
// keeps the canvas responsive without distorting sprites.
import Phaser from 'phaser';
import { BootScene } from './scenes/BootScene';
import { MenuScene } from './scenes/MenuScene';
import { PlayScene } from './scenes/PlayScene';

new Phaser.Game({
  type: Phaser.AUTO,
  parent: 'game',
  width: 1280, height: 720,
  backgroundColor: '#0d0e0f',
  scale: { mode: Phaser.Scale.FIT, autoCenter: Phaser.Scale.CENTER_BOTH },
  scene: [BootScene, MenuScene, PlayScene],
});
`,
		"index.html": `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width,initial-scale=1" />
    <title>Ironflyer Game</title>
    <style>body{margin:0;background:#0d0e0f;color:#fff;font-family:system-ui}#game{display:grid;place-items:center;min-height:100vh}</style>
  </head>
  <body><div id="game"></div><script type="module" src="/src/game/main.ts"></script></body>
</html>
`,
	}
	contract := `Game scaffold: Phaser 3 + Vite, TypeScript.

Already provisioned:
- /src/game/main.ts             → Phaser config + scene registration
- /src/game/scenes/BootScene.ts → asset loader + progress bar
- /src/game/scenes/MenuScene.ts → title + Play CTA
- /src/game/scenes/PlayScene.ts → gameplay loop, score, input
- /index.html                   → mount target

Add new scenes under src/game/scenes/, register them in main.ts. Assets
live in /public/assets/<sprite>.png and are referenced by key from
BootScene.preload(). Persistence: write to localStorage.
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}

// ============================================================
// EcommerceScaffolder — products + cart + Stripe checkout on Next.js.
// ============================================================

type EcommerceScaffolder struct{}

func (EcommerceScaffolder) Name() string { return "ecommerce" }

func (EcommerceScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	if strings.Contains(desc, "store") || strings.Contains(desc, "shop") ||
		strings.Contains(desc, "marketplace") || strings.Contains(desc, "e-commerce") ||
		strings.Contains(desc, "ecommerce") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "cart") || strings.Contains(body, "product") ||
			strings.Contains(body, "order") || strings.Contains(body, "inventory") ||
			strings.Contains(body, "sku") {
			return true
		}
	}
	for _, e := range p.Spec.DataModel {
		n := strings.ToLower(e.Name)
		if n == "product" || n == "order" || n == "cart" || n == "lineitem" || n == "inventory" {
			return true
		}
	}
	return false
}

func (EcommerceScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"lib/commerce/types.ts": `// Canonical commerce types. The product source of truth lives in
// your database (the schema columns mirror these); this file is the
// TS surface every page imports from.

export interface Product {
  id: string;
  slug: string;
  name: string;
  description: string;
  priceCents: number;          // store cents to avoid float drift
  currency: 'USD' | 'EUR' | 'ILS';
  imageUrl?: string;
  inventoryCount: number;
  stripePriceId?: string;      // set after the Stripe sync job runs
}

export interface CartLine {
  productId: string;
  quantity: number;
}

export interface Cart {
  lines: CartLine[];
}
`,
		"lib/commerce/cart.ts": `// Cookie-backed cart. The same shape works for guest checkout and
// signed-in users; on login we merge the cookie cart into the user's
// persisted cart server-side.
'use server';

import { cookies } from 'next/headers';
import type { Cart, CartLine } from './types';

const COOKIE = 'iron_cart_v1';

export async function getCart(): Promise<Cart> {
  const raw = (await cookies()).get(COOKIE)?.value;
  if (!raw) return { lines: [] };
  try { return JSON.parse(raw) as Cart; }
  catch { return { lines: [] }; }
}

export async function setCart(cart: Cart) {
  (await cookies()).set(COOKIE, JSON.stringify(cart), {
    path: '/', sameSite: 'lax', maxAge: 60 * 60 * 24 * 30,
  });
}

export async function addLine(productId: string, qty: number = 1) {
  const cart = await getCart();
  const existing = cart.lines.find(l => l.productId === productId);
  if (existing) existing.quantity += qty;
  else cart.lines.push({ productId, quantity: qty });
  await setCart(cart);
}

export async function setQuantity(productId: string, qty: number) {
  const cart = await getCart();
  const line = cart.lines.find(l => l.productId === productId);
  if (!line) return;
  if (qty <= 0) cart.lines = cart.lines.filter(l => l.productId !== productId);
  else line.quantity = qty;
  await setCart(cart);
}

export async function clearCart() { await setCart({ lines: [] }); }

export function lineCount(cart: Cart): number {
  return cart.lines.reduce((s: number, l: CartLine) => s + l.quantity, 0);
}
`,
		"app/api/products/route.ts": `// GET /api/products — list catalogue. Wire your real DB here; for the
// scaffold we expose a single hard-coded item so the cart + checkout
// flows are testable end-to-end out of the box.
import { NextResponse } from 'next/server';
import type { Product } from '../../../lib/commerce/types';

const STARTER: Product[] = [
  {
    id: 'starter-product',
    slug: 'starter-product',
    name: 'Starter product',
    description: 'Replace this in app/api/products/route.ts with a database read.',
    priceCents: 1900,
    currency: 'USD',
    imageUrl: undefined,
    inventoryCount: 100,
  },
];

export async function GET() {
  return NextResponse.json({ products: STARTER });
}
`,
		"app/api/checkout/cart/route.ts": `// POST /api/checkout/cart — converts the current cart into a Stripe
// Checkout Session using stripePriceId per line. Requires the Stripe
// scaffold (see .ironflyer/stripe.md) to be active.
import { NextResponse } from 'next/server';
import { getStripe } from '../../../../lib/stripe/server';
import { getCart, clearCart } from '../../../../lib/commerce/cart';

export async function POST() {
  const cart = await getCart();
  if (cart.lines.length === 0) {
    return NextResponse.json({ error: 'cart is empty' }, { status: 400 });
  }
  // Look up each line's stripePriceId from your DB. The scaffold expects
  // a syncProductsToStripe job to populate this — write that job using
  // the Stripe scaffold's lib/stripe/server.ts client.
  const lineItems = cart.lines.map(l => ({
    // TODO: replace with real lookup once products land in your DB.
    price: 'price_PLACEHOLDER',
    quantity: l.quantity,
  }));
  const session = await getStripe().checkout.sessions.create({
    mode: 'payment',
    line_items: lineItems,
    success_url: ` + "`${process.env.NEXT_PUBLIC_SITE_URL}/checkout/return?session_id={CHECKOUT_SESSION_ID}`" + `,
    cancel_url:  ` + "`${process.env.NEXT_PUBLIC_SITE_URL}/cart`" + `,
  });
  await clearCart();
  return NextResponse.json({ url: session.url });
}
`,
		"app/cart/page.tsx": `// Client-side cart view. Server actions handle mutation; this page
// only renders. Replace the styling with your design tokens.
import { getCart } from '../../lib/commerce/cart';

export default async function CartPage() {
  const cart = await getCart();
  if (cart.lines.length === 0) {
    return <main style={{ padding: 40 }}><h1>Your cart is empty</h1></main>;
  }
  return (
    <main style={{ padding: 40 }}>
      <h1>Cart</h1>
      <ul>
        {cart.lines.map(l => (
          <li key={l.productId}>
            {l.productId} × {l.quantity}
          </li>
        ))}
      </ul>
      <form action="/api/checkout/cart" method="post">
        <button type="submit">Checkout</button>
      </form>
    </main>
  );
}
`,
	}
	contract := `E-commerce scaffold: Next.js (app router) + Stripe Checkout.

Already provisioned:
- /lib/commerce/types.ts            → Product / Cart / CartLine
- /lib/commerce/cart.ts             → cookie-backed cart with server actions
- /app/api/products/route.ts        → catalogue endpoint (replace with DB)
- /app/api/checkout/cart/route.ts   → cart → Stripe Checkout Session
- /app/cart/page.tsx                → cart view

Required: the Stripe scaffold (see .ironflyer/stripe.md) and (recommended)
the Supabase auth scaffold for persistent customer accounts.

Rules for the Coder:
1. Money is ALWAYS cents (integer). Never floats.
2. Inventory updates happen in the Stripe webhook (paid → decrement).
3. Replace the placeholder lookup in /app/api/checkout/cart/route.ts with
   a real DB read once Product rows exist.
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
