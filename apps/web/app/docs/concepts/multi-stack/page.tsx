// /docs/concepts/multi-stack — the production layer: twelve native
// scaffolders, monorepo subprojects, schema migrations driven by the
// Migrator agent, CI/CD scaffolds with Argo CD + K8s manifests, and
// Redis-backed distributed locks plus cross-pod rate limiting.

import type { Metadata } from 'next';
import { DocPage } from '../../../../components/docs/DocPage';
import { CodeBlock } from '../../../../components/docs/CodeBlock';

export const metadata: Metadata = {
  title: 'Multi-stack',
  description: 'Twelve native scaffolders, monorepo subprojects, schema migrations, K8s CI/CD, and Redis-backed horizontal scale.',
  openGraph: {
    title: 'Multi-stack · Ironflyer',
    description: 'The production layer behind Ironflyer: native scaffolders for twelve stacks, monorepo subprojects, reversible migrations, GitHub Actions + Argo CD + Kubernetes, and Redis distributed locks.',
    images: ['/opengraph-image'],
  },
};

const toc = [
  { id: 'twelve-scaffolders', label: 'Twelve native scaffolders' },
  { id: 'monorepo-subprojects', label: 'Monorepo subprojects' },
  { id: 'schema-evolution', label: 'Schema evolution' },
  { id: 'cicd-k8s', label: 'CI/CD + K8s' },
  { id: 'horizontal-scale', label: 'Horizontal scale' },
];

export default function MultiStackPage() {
  return (
    <DocPage
      eyebrow="Concepts"
      title="Multi-stack"
      description="Ironflyer ships native scaffolders for twelve stacks, treats a repo as a graph of subprojects, evolves database schemas through a dedicated agent, and generates the CI/CD plus Kubernetes manifests needed to put the result in production."
      toc={toc}
    >
      <h2 id="twelve-scaffolders">Twelve native scaffolders</h2>
      <p>
        Scaffolders are not boilerplate templates. Each one is a small agent with an{' '}
        <code>Applies()</code> check that inspects the spec, the existing workspace manifest, and the
        detected stack tags. When the check matches, the scaffolder lays down a coherent baseline that
        the finisher loop can then patch — directory layout, build files, dependency manifests, lint
        configuration, a runnable entrypoint, and the language-specific glue the Code gate expects to
        find.
      </p>
      <p>
        The pack covers twelve production stacks. Triggers are read from the spec and the agent
        manifest the Architect emits:
      </p>
      <ul>
        <li><strong>Rust</strong> — <code>stack=rust</code>. Cargo workspace, <code>clippy</code> + <code>rustfmt</code>, an <code>axum</code> or <code>tokio</code> entrypoint when the role demands a service.</li>
        <li><strong>Go HTTP</strong> — <code>stack=go</code> + <code>role=backend</code>. <code>chi</code> router, <code>zerolog</code>, a health endpoint, and the same Go module conventions the orchestrator itself follows.</li>
        <li><strong>Python FastAPI</strong> — <code>stack=python</code> + <code>role=api</code>. <code>pyproject.toml</code>, <code>uvicorn</code>, <code>ruff</code>, an <code>app/</code> package layout that Alembic can later target.</li>
        <li><strong>Java Spring</strong> — <code>stack=java</code> or <code>spring</code>. Maven or Gradle, Spring Boot starter, a controller, a service, and a Flyway-ready resources tree.</li>
        <li><strong>Kotlin Android</strong> — <code>stack=android</code> + <code>native</code>. Gradle Kotlin DSL, Jetpack Compose, a single Activity, and the Android manifest the Build gate expects.</li>
        <li><strong>Swift iOS</strong> — <code>stack=ios</code> + <code>native</code>. SwiftPM package, SwiftUI app entrypoint, an Info plist, and a unit-test target shape the Code gate can compile.</li>
        <li><strong>Rails</strong> — <code>stack=ruby</code> or <code>rails</code>. Standard Rails 7 layout, <code>bundle</code> manifest, ActiveRecord configured against Postgres, and the migrations folder the Migrator targets.</li>
        <li><strong>Laravel</strong> — <code>stack=php</code>. Composer manifest, Laravel app skeleton, Eloquent, and the <code>database/migrations</code> directory pinned to a working PHP version.</li>
        <li><strong>.NET</strong> — <code>stack=c#</code> or <code>dotnet</code>. <code>dotnet new</code> minimal API, <code>csproj</code>, EF Core wired against the configured database, and an <code>appsettings.json</code> shape Security can scan.</li>
        <li><strong>Next.js</strong> — the existing web pack. App Router, MUI, TypeScript strict, the dashboard pattern Ironflyer itself uses.</li>
        <li><strong>Phaser</strong> — the existing Game pack. HTML5 entrypoint, Phaser scene scaffold, asset pipeline, and a Vite build the Deploy gate can ship as a static bundle.</li>
        <li><strong>Expo</strong> — the existing Mobile pack. Expo Router, EAS build configuration, and a TypeScript baseline shared with the Next.js web app when both surfaces ship from the same repo.</li>
      </ul>
      <p>
        The Ecommerce, Social, Learning, and Dashboard packs continue to layer on top — they describe
        the domain (carts, feeds, lessons, charts) and the language pack underneath them describes the
        runtime.
      </p>

      <h2 id="monorepo-subprojects">Monorepo subprojects</h2>
      <p>
        A <code>Project</code> can declare <code>Subprojects</code>: each subproject has its own{' '}
        <code>Path</code>, <code>Stack</code>, and <code>Role</code>. A single repo can hold an Expo
        mobile app under <code>apps/mobile</code>, a Go HTTP API under <code>apps/api</code>, a
        Next.js dashboard under <code>apps/web</code>, and a Rust worker under{' '}
        <code>services/worker</code> — all driven from one spec, one budget ledger, and one finisher
        loop.
      </p>
      <p>
        Scaffolders run scoped to their subproject root, so the Java pack never spills into the Next.js
        tree and the Rust pack never edits Gradle. The Architect emits a manifest that lists each
        subproject and the role it plays; the Coder routes patches into the right subdirectory using
        that manifest. Future gates will be scoped the same way — a Lint pass in <code>apps/mobile</code>
        will not block a Code patch in <code>services/worker</code>, but a cross-subproject contract
        gate will run when the dependency graph shows them talking to each other.
      </p>

      <h2 id="schema-evolution">Schema evolution</h2>
      <p>
        The <strong>Migrator</strong> agent owns schema changes. It does not invent a migration tool —
        it picks the one the existing manifest already commits to. The selection is mechanical:
      </p>
      <ul>
        <li><code>drizzle.config.ts</code> present → <strong>Drizzle</strong> migrations.</li>
        <li><code>prisma/schema.prisma</code> present → <strong>Prisma</strong> migrations.</li>
        <li><code>alembic.ini</code> present → <strong>Alembic</strong> revisions.</li>
        <li><code>config/application.rb</code> with ActiveRecord → <strong>Rails</strong> migrations.</li>
        <li><code>sequelize</code> in <code>package.json</code> → <strong>Sequelize</strong> migrations.</li>
        <li><code>pom.xml</code> with Flyway → <strong>Flyway</strong> SQL versions.</li>
        <li><code>csproj</code> with <code>Microsoft.EntityFrameworkCore</code> → <strong>EF Core</strong> migrations.</li>
      </ul>
      <p>
        Each emitted migration is reversible: the Migrator writes both the <code>up</code> and the{' '}
        <code>down</code>, runs the dry-run on the workspace database, and surfaces the resulting diff
        to the Code gate. A migration that cannot be reversed is rejected before it lands; one that
        passes is committed as part of the same patch as the model change that motivated it. The
        ledger sees the cost like any other agent turn.
      </p>

      <h2 id="cicd-k8s">CI/CD + K8s</h2>
      <p>
        The CI/CD scaffolder lays down three language-guarded GitHub Actions workflows under{' '}
        <code>.github/workflows/</code> — <code>ci-node.yml</code>, <code>ci-go.yml</code>, and{' '}
        <code>ci-python.yml</code>. Each guards on the presence of the relevant manifest (<code>
        package.json</code>, <code>go.mod</code>, <code>pyproject.toml</code>) and runs build, lint,
        and type-check in the same way the local gates do, so a green CI run means the finisher loop
        will accept the patch.
      </p>
      <p>
        A separate <code>deploy.yml</code> builds the container image, signs it, and pushes it to{' '}
        GHCR. The image tag is the commit SHA. Argo CD picks the tag up from the manifest written
        under <code>infra/k8s/</code>:
      </p>
      <CodeBlock language="bash">{`infra/k8s/
├── application.yaml      # Argo CD Application — points at this repo
├── deployment.yaml       # Deployment with the signed GHCR image
├── service.yaml          # ClusterIP service
├── ingress.yaml          # TLS ingress with cert-manager annotation
└── hpa.yaml              # HorizontalPodAutoscaler — CPU + memory targets`}</CodeBlock>
      <p>
        The Deploy gate verifies that the manifests apply against a dry-run cluster and that the
        Argo CD Application is healthy before the project is marked finished. The same manifest tree
        runs locally with <code>kustomize build</code> for review.
      </p>

      <h2 id="horizontal-scale">Horizontal scale</h2>
      <p>
        Once the orchestrator is run as more than one pod, two pieces have to move out of process
        memory: the per-project lock that prevents two finisher runs from racing on the same
        workspace, and the rate limiter that protects providers and the platform itself.
      </p>
      <p>
        Both move to <strong>Redis</strong> when <code>IRONFLYER_REDIS_ENABLED</code> is set. The
        distributed lock is a <code>SET NX EX 30m</code> on <code>ironflyer:lock:project:&lt;id&gt;</code>
        with a fencing token; the rate limit is a Redis-backed token bucket keyed by user and route,
        so the same cap is enforced across every pod. With Redis off, the orchestrator falls back to
        the in-process mutex and the local rate limiter — fine for single-node deployments, never
        used in production.
      </p>
      <CodeBlock language="bash">{`IRONFLYER_REDIS_ENABLED=true
IRONFLYER_REDIS_URL=redis://redis:6379/0`}</CodeBlock>
      <p>
        Horizontal scale is opt-in but not retrofitted — the lock and rate-limit interfaces are the
        same in both modes, so a deployment can grow from one pod to many without touching code.
      </p>
    </DocPage>
  );
}
