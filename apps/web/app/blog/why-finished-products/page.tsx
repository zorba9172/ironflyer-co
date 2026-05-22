// /blog/why-finished-products — manifesto-flavoured post. ~800 words.

import type { Metadata } from 'next';
import { BlogPost } from '../../../components/docs/BlogPost';

export const metadata: Metadata = {
  title: 'Why we built a finisher, not another generator — Ironflyer',
  description: 'Demos are easy. Finishing the last 20% is the product.',
  openGraph: {
    title: 'Why we built a finisher, not another generator',
    description: 'Demos are easy. Finishing the last 20% is the product.',
    images: ['/opengraph-image'],
  },
};

export default function WhyFinishedProductsPost() {
  return (
    <BlogPost
      title="Why we built a finisher, not another generator."
      subtitle="Demos are easy. Finishing the last 20% is the product."
      tag="Product"
      date="2026-05-08"
      gradient="linear-gradient(135deg, #e5ff00 0%, #79e07a 100%)"
    >
      <p>
        אנחנו עובדים על Ironflyer בערך שנה. הדבר הראשון שעשינו, באמת, היה להוריד שמונה־עשרה
        AI app builder־ים שונים ולשלוח לכל אחד את אותו prompt: <em>“בנה לי דשבורד הכנסות
        למייסדי SaaS עצמאיים, עם Stripe ו־Postgres ו־RTL.”</em> כל אחד מהם החזיר תוצאה מרשימה
        תוך דקה־שתיים. אף אחד מהם לא החזיר מוצר.
      </p>
      <p>
        ה־delta בין השניים — בין דמו ל־מוצר — היא בערך עשרים האחוזים האחרונים של העבודה.
        רובם הולכים מ־0 ל־80 בצורה מדהימה ואז נופלים. אנחנו בנינו את Ironflyer כדי לעשות את
        העשרים האחוזים האחרונים. זאת הסיבה שהכל בפלטפורמה נקרא בשם “Finisher” ולא “Generator”.
      </p>

      <h2>What the demo hides</h2>
      <p>
        Watch ten AI app builder demos in a row and you start noticing the seams. The generated
        UI is gorgeous, but the types are wrong. The test suite is missing. The deploy config is
        for a stack that does not match the framework. The secrets are hardcoded. The Hebrew text
        renders left-to-right because nobody wired <code>dir="rtl"</code>. The pricing page is a
        component the model invented from a training-data hallucination and nobody noticed.
      </p>
      <p>
        These are not small bugs. They are the exact things a code review is for. The reason every
        other AI app builder ships with these seams is structural: the model writes files directly,
        the platform serves a preview, and nobody enforces that the seams are closed before you
        click <em>Deploy</em>. So you click Deploy and find out a week later that your auth flow
        was JWT-misuse on day one.
      </p>

      <h2>Gates instead of vibes</h2>
      <p>
        Our answer is to make the platform itself enforce what <em>finished</em> means. Not as a
        marketing claim — as eight literal gates that block. Spec → UX → Architecture → Code →
        Lint → Tests → Security → Deploy. A red gate halts the loop. A green gate emits a patch
        that the next gate gets to scrutinise. The gates do not skip; they are not advisory; they
        are not opt-in by default.
      </p>
      <p>
        Some of those gates are obvious — of course we run a linter, of course we run the tests.
        The interesting ones are the gates that the rest of the industry quietly turns off. The
        Spec gate refuses to proceed if the prompt is ambiguous. The Architecture gate forces a
        stack decision before the Code gate is allowed to type a line. The Security gate refuses
        patches that hardcode secrets or wire SSRF-prone fetches. The Deploy gate makes the
        generated <code>fly.toml</code> survive a second pass through the Security gate before
        we push.
      </p>

      <h2>Patches, not file writes</h2>
      <p>
        We made one structural choice that surprised our early users: <strong>the AI never writes
        files directly.</strong> Even when the coder agent decides a single line in a single file
        needs to change, the path is the same — propose a patch, the patch engine validates, the
        security gate scans, the runtime applier mirrors the change into the sandbox. It is more
        work, and it is the entire point. The gates only catch real problems if every change has
        to pass through them.
      </p>

      <h2>The economic argument</h2>
      <p>
        We charge for the finisher loop, not for tokens. Most AI tools end up nudging you toward
        spending more — more conversation turns, more revisions, more “let me try one more time”.
        We do not want to make money that way. So we picked a hard cap per plan,
        <em> subscription − cost cap = margin floor</em>, and we publish the math at
        <code> GET /budget/vault</code>. If a run finishes in five minutes you do not get a discount,
        and if it takes thirty you do not get a surcharge.
      </p>

      <h2>What a finished product looks like</h2>
      <p>
        Internally we measure ourselves by a simple test: pick a generated project at random,
        deploy it, and try to break it the way a junior engineer on their first day would. The
        bar is that the eight gates should have caught everything in advance. So far the bar is
        not always met — every miss is a new gate rule. But the trajectory matters, and the
        platform is set up so that every fix benefits every future user.
      </p>
      <p>
        Ironflyer is, in the end, a bet that builders want to ship done. Not done in the sense of
        “the UI looks finished” but done in the sense of “there is no follow-up cleanup”. If that
        is the bar you want, this is the loop.
      </p>
    </BlogPost>
  );
}
