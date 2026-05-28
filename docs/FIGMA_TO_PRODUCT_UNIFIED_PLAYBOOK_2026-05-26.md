# Ironflyer Unified Playbook — Figma to Product, End-to-End (2026-05-26)

> **גרסה מורחבת — Powerhouse Edition.**
> מסמך תפעולי אחד שמרכז את כל היכולות של מנוע Ironflyer V22:
> מ-Figma פתוח ועד מוצר חתום, ארנק, ProfitGuard, gate-stack, Visual-first cockpit,
> mobile native, ו-Finish Loop ברמת שחרור.

---

## 0) מה המסמך הזה נותן

זה ה-Operating Playbook היחיד שבונה גשר בין החזון לבין הקוד שכבר חי בריפו.
הוא מאחד שלוש שכבות:

1. **מה Ironflyer מבטיח** — חוויית קצה-לקצה: משתמש מעלה Figma + prompt חופשי
   ומקבל מוצר שעבר gates ונפרס.
2. **איך זה נאכף כלכלית** — חוקי V22 הקשיחים: אין execution בלי ארנק, אין
   reasoning יקר בלי ProfitGuard, אין scale בלי margin חיובי.
3. **איזה רכיבים אמיתיים מוכנסים לעבודה** — `finisher`, `wallet`, `ledger`,
   `profitguard`, `patch`, `appsec`, `blueprints`, `wowloop`, `mobile`,
   `runtime`, ה-agent-mesh ב-`agents.yaml`, ו-Gate Stack של 17 גייטים שכבר
   מוצהרים ב-`core/orchestrator/internal/ai/domain/types.go`.

המטרה: פחות מסמכים מקבילים, פחות פיצול, יותר תפעול אמיתי שמכוון לאותו צפון:
**Profitable Completed Execution Rate** — כמה ריצות בתשלום נסגרו בהצלחה
ועם margin חיובי.

> אם שינוי הופך execution בתשלום לקל יותר לזיוף "סיום" — הוא נגד המוצר.
> אם שינוי מהדק את ההגדרה של "סיום רווחי" — הוא עם המוצר.

---

## 1) עיקרון מארגן: לא לייצר קוד, לסגור מוצר

לבילד-אבל-אין-שחרור אין ערך כלכלי. כל זרימה במערכת חייבת לענות באותו רגע
על שלוש שאלות:

1. **מה עוד לא סגור end-to-end?** — איזה gate ממתין, איזה patch לא הוחל,
   איזה artifact חסר, איזה cost line לא נכלל בארנק.
2. **כמה זה עולה ומי משלם?** — wallet hold, ledger debits, ProfitGuard
   verdict, margin צפויה.
3. **איך זה נראה למשתמש בזמן אמת?** — מצב נוכחי, ETA, סיכון פתוח, פעולה
   הבאה — בתבנית אחידה.

עיקרון Visual-First מתורגם לכך שכל מסך תפעולי (cockpit, studio, execution,
profit, wallet) חייב לרנדר תחילה גרף-מצב חי שממפה את state המנוע, ורק
אחריו לחשוף את שכבת ה-pros (VS Code, GraphQL Sandbox, ledger CSV) דרך
toggle.

---

## 2) חוקי הברזל הכלכליים (V22 Hard Laws)

שלושת החוקים שמכופפים כל gate, resolver, ו-tick:

1. **No execution starts without budget.**
   `wallet.balance ≥ reservation` או 402 Payment Required + `top_up_url`.
   נאכף ב-`internal/business/wallet` ובמסלול ה-execution לפני שום provider call.
2. **No expensive reasoning runs without expected ROI.**
   `ProfitGuard` (`internal/business/profitguard` + `profitguardbridge`) שואל
   "האם הריצה הזאת תישאר רווחית?" לפני כל קריאת Opus/Sonnet יקרה, הקצאת
   sandbox, mobile build, Vercel deploy, retry-loop ארוך, או artifact כבד.
3. **No scale is healthy unless gross margin holds.**
   `internal/business/dashboards` מוביל קודם את ה-margin, ורק אחר כך scale.
   forecast (`business/forecast`) משווה revenue vs. provider cost כל היום.

> חוקים אלו אינם "מוצעים" — הם תנאי קיום. כל gate חדש, כל resolver, כל
> finisher recipe חייב לציית להם.

---

## 3) חוויית היעד — End-to-End User Journey

### 3.1 המשפט הפשוט

המשתמש מעלה Figma, מגדיר כוונה עסקית, ומקבל מוצר שעבר gate-stack ונפרס.

### 3.2 מה המשתמש רואה בפועל

1. **Onboarding + Top-up.** הרשמה ב-AuthShell (split-layout Base44), טעינת
   ארנק דרך Stripe Checkout (`POST /budget/webhook`).
2. **Intake.** Upload של Figma file/link + prompt חופשי + אילוצי סטאק/מובייל.
3. **Decompose.** Preview של פירוק העיצוב (מסכים, קומפוננטות, טוקנים, מצבים,
   נכסים) + רשימת ambiguity שצריך לפתור.
4. **Plan + ETA + Cost.** תכנית עבודה עם work packages, סיכונים, ETA, וצפי
   עלות מתוך ה-ledger.
5. **Approve.** המשתמש מאשר plan; ProfitGuard מגדיר reservation בארנק.
6. **Execute.** ה-Execution Mesh רץ עם דיווח אנושי בזמן אמת (SSE על
   `/executions/{id}/chat/stream` + `executionFeed` ב-GraphQL Subscriptions).
7. **Interrupt-Friendly.** אם מגיע prompt חדש, שינוי Figma, או כשל
   אינטגרציה — Interruption Layer עוצר בנקודת בטיחות, מחשב impact,
   ומפיק plan patch.
8. **Finish Loop.** ה-gate-stack רץ; כל verdict חי מתועד.
9. **Publish.** deploy artifacts נשמרים, smoke רץ דרך `scripts/smoke.sh`,
   דוח סופי קריא נמסר.
10. **Learn.** wowloop קולטת evidence ולומדת לסבב הבא.

### 3.3 קריטריון "סגור"

- כל gate חובה עבר (pass/warn מנומק).
- patches הוחלו — אף שינוי לא נכתב ישירות אלא דרך `patch.Engine.Propose`.
- ה-ledger מאוזן: revenue − providerCost − sandboxCost = margin ≥ 0.
- artifact פרוס וניתן לפתיחה ב-URL פעיל.
- דוח סופי נשלח עם evidence trail.

---

## 4) הארכיטקטורה הפעילה — מה מאיפה

```
ironflyer/
├── core/
│   ├── orchestrator/       מנוע התזמור: wallet, ledger, ProfitGuard, finisher,
│   │                       gates, providers, auth, blueprints, repair,
│   │                       dashboards, mobile, deploy, appsec, audit
│   ├── runtime/            sandboxes per-user: mock + docker drivers,
│   │                       Mobile supplier (Metro/emulator/xcodebuild)
│   └── cli/                operator CLI
├── clients/
│   ├── web/                Next.js 15 + MUI 6 — cockpit, studio, marketing
│   ├── vscode-extension/   thin client (auth via SecretStorage + URI handler)
│   └── scrcpy-bridge/      WebSocket bridge לראיית מסך מובייל
├── packages/
│   ├── design-tokens/      tokens.color.* — המקור היחיד לצבעים
│   ├── sdk/                @ironflyer/sdk — GraphQL types + SSE clients
│   └── agents/             reserved (canonical agents.yaml בתוך orchestrator)
├── infra/                  Dockerfiles (כולל mobile-runtime), compose, k8s
├── scripts/                smoke.sh — verification פוסט-deploy
├── templates/starters/     expo, android-kotlin, ios-swift
└── docs/                   V22_PLAN, ARCHITECTURE_*, ה-playbook הזה
```

עיקרון: **GraphQL only** ב-orchestrator. חריגים: Stripe webhook, k8s probes,
Prometheus, ו-SSE של chat-stream. כל מסלול ה-runtime הוא REST פנימי
תחת `/v1/workspaces/{id}/...`.

---

## 5) שכבות העבודה (Layered Architecture) — עם מיפוי לרכיבים אמיתיים

### Layer A — Intent & Context Capture
תפקיד: לתרגם prompt פתוח ל-Charter ביצוע.
- מנגנון: `ai/ideaparser` שולח את ה-prompt דרך הרשת ה-router של
  `ai/providers`, מחזיר Intent JSON עם scope, constraints, ambiguity score.
- פלט: Task Charter (id, goal, success criteria, must-have vs nice-to-have,
  user persona, business constraint).
- Governance: כל פלט עובר Self-check לפי הכלל ב-`agents.yaml`
  (verdict/citations/schema).

### Layer B — Design Intake & Decomposition
תפקיד: לקחת Figma ולפרק אותו לבילד.
- סוכן אחראי: `figma-translator` (ב-`agents.yaml`).
- צרכן: ה-`scaffold` ו-`uxer` מקבלים את הפלט בפורמט מובנה.
- תוצרים: screen-map, component graph, tokens diff מול
  `packages/design-tokens`, state-matrix (loading/error/empty/success),
  asset manifest.

### Layer C — Normalization to Internal SoT
תפקיד: לקבע מקור אמת פנימי יציב, בלתי תלוי בתנודות Figma.
- פורמט מחייב: `.ironflyer/screen_map.json` + `.ironflyer/design_tokens.json`
  + `.ironflyer/component_graph.json`.
- כלל ברזל: Figma הוא **input** בלבד. אם Figma מתעדכן — Sync Layer מטפל,
  לא Build Layer.

### Layer D — Adaptive Plan Builder
תפקיד: לייצר תכנית ריצה כלכלית.
- סוכן אחראי: `planner` (enableThinking: true; reasoning+cache).
- פלט: work-packages → file list מינימלי, תלותים, מסלול קריטי, ETA per WP,
  closure criteria per WP, סיכונים מספריים, ProfitGuard reservation עם
  cost-bands.
- כלל ברזל: **אין Build לפני Plan Approval**. ProfitGuard מקבל reservation
  hold, וה-execution לא מתחיל עד שהמשתמש אישר.

### Layer E — Execution Mesh
תפקיד: לבנות בפועל.
- סוכנים: `architect` → `coder` → `reviewer` → `tester` → `critic`.
  (כללית — `tester` משלים evidence אבל **לא כותב טסטים** בהתאם לחוק
  ה-constitutional של no-tests-ever; הוא מאמת ב-smoke ובהפעלת האפליקציה.)
- חוזה: **patches mandatory** — `patch.Engine.Propose` היא הדרך היחידה
  לקוד להגיע ל-disk; ה-lifecycle gates מאשרים לפני apply.
- Streaming: כל provider חי דרך `CompleteStream`; טוקנים זורמים דרך
  `BillingGuard` ונשמרים ב-ledger.

### Layer F — Interruption & Recovery
תפקיד: לתפקד כשהמציאות זזה.
- מקור הפרעה אופייני: prompt-interrupt חי, שינוי Figma באמצע, כשל
  provider/sandbox, חסר secret.
- רצף תגובה:
  1. **Safe-Point Snapshot** ב-`execution.memory`/`execution.idempotency`.
  2. **State Summary** דרך `dashboards/adapters`.
  3. **Impact Diff** על ETA, scope, ו-margin.
  4. **Plan Patch** (work-package level) + ProfitGuard re-evaluation.
  5. **Resume** ללא איבוד טוקנים — caching של provider responses חוסך
     reasoning שכבר שולם עליו.

### Layer G — Sync & Reconciliation
תפקיד: לסנכרן Figma ↔ Code ↔ Plan.
- מנגנון: Detect → Diff → Decide (auto/manual) → Apply → Re-baseline.
- Auto-policy: drift קוסמטי (spacing token זהה ערכית, שינוי שם component
  בלי שינוי semantic) → apply.
- Manual policy: drift סמנטי (מסך נמחק, flow שונה, token חדש שאין לו
  מקור ב-`packages/design-tokens`) → escalation אנושי.

### Layer H — Finish Loop (World-Class Gate Stack)
תפקיד: החלטת שחרור אמינה.
- מנוע: `business/wowloop` (sources → builder → bundle) שמוביל את
  ה-gate-chain.
- Gate Stack מפורט בסעיף 7.

### Layer I — Publish & Learning Loop
תפקיד: לסגור deploy וללמוד.
- מסלול deploy: `operations/deploy` + Vercel/k8s targets, פוסט-deploy
  `scripts/smoke.sh`.
- artifacts: לפי `internal/operations/storage/s3client.go` עם backend
  בחירה (`aws|r2|minio`).
- Learning: `wowloop` משמר evidence per gate, ProfitGuard מתעדכן
  על variance בין צפי לעלות בפועל, dashboards מציגים accuracy.

---

## 6) Agent Mesh — 13 סוכנים פעילים

ה-agents מוגדרים ב-[`core/orchestrator/internal/ai/agents/agents.yaml`](../core/orchestrator/internal/ai/agents/agents.yaml).
כולם כפופים לכללי ה-Self-check (verdict, citations, schema, no-prose).

| Role | אחריות | Capabilities | Thinking |
|---|---|---|---|
| `planner` | פירוק רעיון ל-stories + file list מינימלי | reasoning, thinking, cache | ✓ |
| `uxer` | חוזה UI: states, flows, edge-cases, a11y | reasoning, cache | – |
| `figma-translator` | פירוק Figma → screen_map + tokens diff | reasoning, cache | – |
| `architect` | data model, interfaces, module boundaries | reasoning, cache | ✓ |
| `coder` | כתיבת patches בפועל | reasoning, fast | – |
| `migrator` | DB migrations + backfills | reasoning | – |
| `reviewer` | code review מול מדיניות הריפו | reasoning, cache | – |
| `tester` | evidence ב-smoke ובהפעלת אפליקציה (לא test-files) | fast | – |
| `security` | gate verdict ל-AppSec | reasoning, cache | – |
| `critic` | אתגור הסוף — האם זה באמת shippable? | reasoning, thinking | ✓ |
| `deployer` | deploy artifacts + post-deploy validation | reasoning | – |
| `mobile-coder` | Expo/RN/Kotlin/Swift/Flutter patches | reasoning | – |
| `mobile-deployer` | eas.json, fastlane, mobile release workflows | reasoning | – |

> ה-Voice של כל הסוכנים: senior staff engineer ב-flagship product company.
> ישיר, opinionated, ללא hedging, ללא markdown fences כש-JSON מתבקש.

ה-router (`ai/providers`) בוחר provider לפי capability: Anthropic Claude
4.x כברירת מחדל (Opus 4.7 ל-`thinking/reasoning/quality`, Sonnet 4.6
ל-general, Haiku 4.5 ל-`fast/inline_completion`). חלופות: OpenAI 4o/o3,
Gemini 2.5, HuggingFace, DeepSeek, Vercel AI Gateway.

---

## 7) Gate Stack — 17 הגייטים האמיתיים

מקור הקודים: `core/orchestrator/internal/ai/domain/types.go`.

### 7.1 גייטים מוצריים-לוגיים
| Gate | מה הוא בודק | Severity Block |
|---|---|---|
| `spec` | האם ה-Charter מכסה goal, persona, success criteria, constraints | Critical |
| `ux` | states, empty/error/loading, flows, a11y | High |
| `arch` | data model + module boundaries תואמים | High |
| `code` | reviewer verdict על ה-patches | High |
| `lint` | go vet + tsc --noEmit | High |
| `test` | smoke evidence, **לא test files** (חוק no-tests) | Medium |
| `budget` | wallet hold ≥ reservation; ProfitGuard verdict חיובי | Critical |
| `security` | AppSec engine (`operations/appsec` + `securityreport`) | Critical/High |
| `lighthouse` | performance + a11y budget בווב | High |
| `deploy` | artifact קיים, URL חי, smoke עבר | Critical |

### 7.2 גייטי מובייל
| Gate | מה הוא בודק |
|---|---|
| `mobile_build` | gradlew assembleDebug / xcodebuild / eas build הצליח |
| `mobile_expo_doctor` | תקינות Expo manifest + SDK |
| `mobile_size` | bundle size מתחת ל-budget |
| `mobile_bundle_analyzer` | פירוק חבילות native + JS |
| `mobile_security` | secrets לא מודלפים, certs תקינים |
| `ios_privacy_manifest` | Apple PrivacyInfo.xcprivacy תקין |
| `mobile_push_credentials` | APNs/FCM creds זמינים בלי serialization |

### 7.3 עיקרי תפעול הגייטים
- כל gate מקבל `(ctx, *GateEnv)` ורושם verdict (`pass | warn | fail`).
- `verdict: "fail"` עם severity `Critical/High` חוסם deploy.
- `verdict: "warn"` חולף ל-publish רק אם profile מאפשר (סעיף 11).
- כל verdict נשמר ב-`audit` עם evidence path; `dashboards` מציגים גרף-gate
  בזמן אמת.
- gates חדשים נרשמים ב-`finisher.DefaultGates()` ומופיעים ב-`domain.GateName`.
- **גייטי Anti-Bloat נוספים** (`reuse_check`, `dedup`, `deadcode`, `complexity`,
  `dep_graph`, `arch_boundary`, `bundle_size`, `mem_leak`, `perf_budget`,
  `vuln_scan`) מפורטים בסעיף 8.7 וחיים תחת ה-lane של Anti-Bloat Engine.

---

## 8) Anti-Bloat Engine — שימוש חוזר, אופטימיזציה ומשמעת ארכיטקטונית

### 8.1 הבעיה האמיתית בשוק כלי ה-AI Coding

הכלים המתחרים (Lovable, Base44, Bolt, Replit Agent, v0, Cursor agent mode)
כותבים קוד **לכל בעיה מחדש**. התוצאה:

- **איים מנותקים** — שתי קומפוננטות עושות אותו דבר עם שמות שונים.
- **קוד שכופל את עצמו** — אותה לוגיקה ב-3 קבצים שונים, אחד מהם עם באג.
- **ספריות נשכחות** — utils קיימים שאף אחד לא יודע עליהם, כי המודל לא
  חיפש לפני שכתב.
- **חוסר אופטימיזציה** — bundles תפוחים, re-renders מיותרים, queries
  כפולים, goroutines דולפים, allocations שלא משוחררים.
- **ניתוק ארכיטקטוני** — module שמכניס תלות בכיוון אסור; layering נשבר;
  cycles נוצרים.
- **קוד שהוא רק "scaffolding"** — מאות שורות שאף אחד באמת לא יקרא, רק
  כדי שהתוצאה תיראה "שלמה".

Ironflyer לא נכנס לזה. **קוד טוב הוא קוד שלא חוזר על עצמו, שמשתמש בכלים
שכבר קיימים, ושעובר אופטימיזציה לפני שמשחררים אותו.** ה-Anti-Bloat Engine
הוא חלק מהליבה — לא תוסף.

### 8.2 חמשת חוקי ה-Anti-Bloat

1. **Reuse-First.** לפני שנכתב patch חדש, חיפוש סמנטי של utility או
   component קיים הוא חובה. אם נמצא — ה-coder מחויב לקרוא לו במקום
   לכפול.
2. **Diff Economy.** patches נמדדים ב-net-LOC. עודף מצדיק justification
   ב-`audit`. הסוכנים מתוגמלים על delta קטן עם השפעה גדולה.
3. **No Orphans.** module/file חדש בלי inbound edge תוך 24 שעות עולה
   ל-cockpit כ-"island candidate".
4. **Boundary Honesty.** layering rules מוצהרים ונאכפים. הפרת layering
   חוסמת patch — לא warning.
5. **Optimization Before Publish.** bundle weight, cognitive complexity,
   re-render counts, ו-leak detection נמדדים ב-Finish Loop ולא ב-PR
   הבא.

### 8.3 Capability Atlas — האטלס הפנימי של הקוד

`Capability Atlas` הוא אינדקס חי של כל ה-utilities, hooks, services,
components, ו-blueprints שכבר קיימים בריפו, עם embeddings שמאפשרים
שאילתה טבעית.

- **בנייה**: ה-Atlas נבנה דרך `ai/embeddings` + `ai/memory` (backend
  pgvector מומלץ ל-scale; surreal לגרף תלויות; memory לפיתוח).
- **שאילתות**: ה-coder / architect / uxer חייבים להריץ
  `atlas.search("debounce hook")` או `atlas.search("Stripe webhook
  verifier")` לפני שהם פותחים קובץ חדש.
- **תוצרים**: רשימת hits עם path, חתימה, exports, usage count, ו-cost
  per call (אם נמדד).
- **Refresh policy**: incremental update בכל patch שמתאשר; full re-index
  לילי דרך `temporalworker`.

> ה-Atlas הוא ההבדל בין "מודל שמנחש שיש util" לבין "מערכת שיודעת".

### 8.4 Reuse-First Preflight — תהליך אכיפה

לפני כל `patch.Engine.Propose` של קובץ חדש או של פונקציה ציבורית חדשה,
ה-coder מבצע:

1. **Semantic search** ב-Capability Atlas על תיאור הפונקציה הצפויה.
2. **Top-K hits** מוחזרים (K=5).
3. **Decision JSON** מהסוכן:
   `{"reuse": "<path>" | "extend": "<path>" | "new": "<justification>"}`.
4. אם `new` — חובת justification טקסטואלי שעובר ל-`reviewer` ולגייט
   `reuse_check` (סעיף 8.7).
5. אם `reuse`/`extend` — ה-patch מתעדכן ומפנה לקיים, ולא יוצר אי חדש.

### 8.5 ספריות שאנחנו מכניסים ל-Finish Loop

זוהי רשימת הכלים שמוטמעים ישירות ב-gate-stack. כולם streaming-friendly,
deterministic, ו-budget-aware.

#### TypeScript / Next.js / React (`clients/web`, `packages/sdk`)
- **`jscpd`** — copy/paste detector cross-file. budget: ≤ 2% dup rate.
- **`knip`** — dead exports, unused files, unused deps. budget: 0 strict.
- **`ts-prune`** *(fallback ל-knip)* — unused exports בלבד.
- **`depcheck`** — unused dependencies ב-`package.json`.
- **`dependency-cruiser`** — boundary rules + no-cycles. config ב-repo.
- **`madge`** — circular dependency map; ויזואל ב-cockpit.
- **`size-limit`** + **`@next/bundle-analyzer`** — budgets per route.
- **`eslint-plugin-sonarjs`** — cognitive complexity ≤ 15 per function.
- **`oxlint`** *(מהיר; ratchet-only)* — duplicate keys, redundant code.
- **`type-coverage`** — לפחות 99% (no implicit any).
- **`why-did-you-render`** *(dev only)* — re-renders חורגים מסמנים את
  הקומפוננטה.
- **`react-scan`** *(opt-in)* — heat-map של renders.
- **Lighthouse CI** — performance + a11y budgets פר route (כבר חי דרך
  gate `lighthouse`).
- **Playwright trace** ל-smoke בלבד (no test files), evidence ל-deploy
  gate.

#### Go (`core/orchestrator`, `core/runtime`)
- **`go vet`** + **`staticcheck`** + **`golangci-lint`** — ה-baseline.
- **`gocognit`** + **`gocyclo`** — complexity budgets per function.
- **`unparam`** — פרמטרים שלא בשימוש.
- **`dupl`** — duplicated code blocks cross-package.
- **`exhaustruct`** *(selective)* — חוסם ייצור structs חסרי שדות
  באזורים קריטיים (wallet, ledger).
- **`ireturn`** — חוסם החזרת interfaces ללא צורך.
- **`govulncheck`** — vulnerability scan פר build.
- **`pprof` + `fgprof`** — CPU/heap profiling אוטומטי בריצות long-tail.
- **`goleak`** — leak detection ב-smoke runs (לא בטסטים — חוק
  no-tests-ever עומד; `goleak` משולב ב-smoke harness בלבד).
- **`benchstat`** — השוואת bench בין branches דרך smoke output.
- **`go-mod-outdated`** — דיווח על תלויות ישנות.

#### חוצה-שפות
- **`semgrep`** עם custom rules — "אותה פונקציה מוגדרת פעמיים" /
  "secret hardcoded" / "raw hex color" (מחזק את החוק החוקתי על tokens).
- **`comby`** — structural search-and-replace ל-refactors אוטומטיים.
- **`jscodeshift`** / **`ts-morph`** — codemods תוכניתיים ל-extract util.
- **`hyperfine`** — benchmark commands ב-smoke (boot time, cold start).
- **OpenTelemetry** *(כבר)* — traces חוצי-שירות; ProfitGuard צורך זאת
  למדידת cost-per-capability.

> כל הכלים האלו רצים תחת ProfitGuard. אם budget לא מאפשר את הרצת הסט
> המלא — נופלים לתת-קבוצה לפי profile (Startup / Enterprise / Regulated).

### 8.6 Refactor Proposals — אוטו-מיצוי

כשגייט `dedup` או `complexity` מסמן ממצא, המערכת **לא רק מתלוננת** —
היא מציעה patch ממוצה:

1. `coder` מקבל את ה-finding (path:line + duplicate paths).
2. רץ codemod (`ts-morph` / `comby`) שמוציא util משותף.
3. מציע patch ל-`patch.Engine.Propose` עם כותרת `[refactor:dedup]`.
4. `reviewer` חותם.
5. ProfitGuard מאשר את העלות (זול: זה patch קטן וצפוי).
6. אם המשתמש דחה — ה-finding עובר ל-warn ולא חוזר באותה ריצה.

> זה ההבדל בין "כלי שמסמן בעיה" ל-"מערכת שפותרת אותה".

### 8.7 Gates חדשים ל-Anti-Bloat Lane

נוספים ל-Gate Stack של סעיף 7:

| Gate | מה הוא בודק | חוסם? |
|---|---|---|
| `reuse_check` | האם בוצע preflight + justification ל-`new` | High אם missing |
| `dedup` | jscpd / dupl rate ≤ budget | High |
| `deadcode` | knip / ts-prune / unparam — 0 בסטריקט | Medium-High |
| `complexity` | gocognit / sonarjs ≤ budget פר function | Medium-High |
| `dep_graph` | dependency-cruiser: no-cycles + boundaries | Critical |
| `arch_boundary` | layering manifest enforcement (סעיף 8.9) | Critical |
| `bundle_size` | size-limit + bundle-analyzer per route | High |
| `mem_leak` | goleak + heap diff בסמוק | High |
| `perf_budget` | hyperfine + Lighthouse + Web Vitals budgets | High |
| `vuln_scan` | govulncheck + npm audit | Critical/High |

כולם נרשמים ב-`finisher.DefaultGates()` ומקבלים `(ctx, *GateEnv)`.
כולם תומכים `verdict: pass | warn | fail` עם evidence path.

### 8.8 יציבות זיכרון מובנית

**Go side:**
- ב-smoke harness: כל אינטראקציה מורצת תחת קריאה ל-`goleak.Find()` ידנית
  (ללא test files — קריאה ישירה מ-`scripts/smoke.sh`).
- pprof endpoints מאופשרים תחת auth ב-`/debug/pprof` רק ב-staging.
- heap snapshot לפני ואחרי כל deploy; diff חורג מסף מעלה דגל.
- finalizers + `runtime.SetFinalizer` אסורים אלא ב-cleanup paths
  מתועדים — semgrep rule.
- contexts: כל goroutine שמושק חייב לקבל `ctx`; semgrep חוסם `go func()`
  ללא ctx-binding.

**Web / Node side:**
- React: כל list rendering חייב `key` יציב; semgrep רול ל-index-as-key.
- Memo discipline: `useMemo`/`useCallback` רק כשיש benchmark; אחרת
  knip יסיר אותם.
- Observers: `IntersectionObserver`, `ResizeObserver`, `addEventListener`
  — חובה cleanup ב-`useEffect`; eslint-plugin-react-hooks אוכף.
- WebSocket / SSE clients: closeable interface מחויב; ה-cockpit מציג
  count של חיבורים פעילים פר tab.
- Heap profile של ה-vscode-extension לפני release (התוסף הוא long-lived
  — leaks מורגשים מיד).

**מובייל:**
- Expo / RN: `LeakCanary`-style reporting דרך smoke run ב-Android;
  goleak-equivalent ב-native bridges.
- iOS: heaviest allocations diff דרך xcodebuild Instruments output.
- Android: `dumpsys meminfo` snapshot ב-emulator לאחר smoke.

### 8.9 Architecture Boundary Enforcement

קובץ מניפסט יחיד: `.ironflyer/architecture.json`. הוא קובע:

```jsonc
{
  "layers": [
    "domain",         // pure types, no IO
    "business",       // wallet/ledger/profitguard/execution
    "operations",     // adapters: http, graph, deploy, storage
    "interface"       // graph resolvers, http handlers
  ],
  "rules": [
    { "from": "business", "to": "operations", "allow": false },
    { "from": "domain",   "to": "*",          "allow": false },
    { "from": "operations","to": "business",  "allow": true  }
  ],
  "cycles": "deny",
  "owners": {
    "wallet":      ["business/wallet"],
    "ledger":      ["business/ledger"],
    "profitguard": ["business/profitguard"]
  }
}
```

- `dep_graph` + `arch_boundary` קוראים את המניפסט. הפרה — `fail`.
- כל patch שמוסיף import מעבר ל-rule נחסם לפני apply.
- ה-cockpit מציג גרף שכבות אוטומטית; הפרה מסומנת באדום.

### 8.10 Diff Economy — תקציב שורות לכל patch

`patch.Engine` מקבל `MaxNetLOC` per patch לפי profile:
- Startup: 400 net LOC; חריגה ⇒ split required.
- Enterprise: 200 net LOC.
- Regulated: 120 net LOC.

ProfitGuard מתעד `loc_added`, `loc_removed`, `loc_net` ב-`audit`.
ה-Profit Dashboard מציג מטריקת "LOC per Resolved Capability" — ככל
שזה יורד, המוצר משתפר.

### 8.11 Code Health Dashboard

מסך חדש ב-`clients/web` (`/cockpit/health`), נטען עם dynamic import,
מציג:
- **Duplication heatmap** — קבצים עם dup rate גבוה.
- **Dead-code panel** — exports/files/deps שאף אחד לא משתמש בהם.
- **Complexity sparkline** — gocognit / sonarjs per module לאורך זמן.
- **Bundle weight** — size-limit per route.
- **Dependency graph** — madge / dependency-cruiser ויזואלי, לחיץ.
- **Memory trend** — heap diff בין רילסים.
- **Reuse rate** — אחוז ה-patches שבחרו `reuse`/`extend` מתוך כלל
  ההזדמנויות.

כל פאנל לחיץ → drill לקבצים → לחיץ → VS Code (opt-in) → אישור
`refactor proposal` בקליק.

### 8.12 איך זה משפר את המוצר למשתמש

1. **Builds קטנים יותר**. bundle size יורד 30-60% מול מתחרים שמעתיקים
   קוד; טעינה מהירה יותר ל-user-facing apps.
2. **פחות באגים מתחבאים**. dedup מבטל הופעות-כפילות עם תיקון חסר באחת.
3. **Onboarding מהיר יותר**. מפתחים שיורשים את הקוד רואים מבנה אחיד,
   לא חזרות.
4. **עלות נמוכה יותר**. פחות LOC = פחות תחזוקה, פחות time-on-task
   לסוכנים, פחות $ ב-ProfitGuard.
5. **שקיפות חזקה**. הלקוח רואה מטריקת איכות חיה במקום להאמין למודל.
6. **Defensibility**. בעוד שמתחרים מציעים "תאר → קוד", Ironflyer מציע
   "תאר → קוד מאופטם, מוכח, ולא כפול".

### 8.13 איפה זה חי בקוד (יעד יישום)

| מודול חדש | מיקום מוצע |
|---|---|
| Capability Atlas | `core/orchestrator/internal/ai/atlas/` (חדש) |
| Reuse-First preflight | `core/orchestrator/internal/ai/agents/preflight.go` (חדש) |
| Anti-Bloat gates | `core/orchestrator/internal/business/wowloop/antibloat_*.go` |
| Architecture manifest reader | `core/orchestrator/internal/operations/arch/` (חדש) |
| Refactor proposer | `core/orchestrator/internal/ai/refactor/` (חדש) |
| Code Health adapters | `core/orchestrator/internal/business/dashboards/health.go` |
| Health UI | `clients/web/src/app/cockpit/health/` |
| Toolchain bin shims | `scripts/lint/` + `scripts/health/` |
| Toolchain manifest | `.ironflyer/architecture.json` + `.ironflyer/health-budgets.json` |

---

## 9) Workspace & Runtime — איפה הקוד באמת חי

מנוע הריצה הוא `core/runtime` — מודול Go נפרד עם `go.mod` משלו.

### 8.1 Drivers
- **Mock** — לפיתוח מקומי ולסביבת CI ללא Docker.
- **Docker** — sandbox per-user עם file API ו-PTY WebSocket לחיבור מסוף.

### 8.2 Mobile Supplier (`core/runtime/internal/suppliers/mobile`)
- Metro start/stop, Android emulator allocation (דורש KVM passthrough),
  iOS xcodebuild dispatch.
- Routes: `/v1/workspaces/{id}/mobile/...` (REST פנימי; הכלל
  GraphQL-only שייך ל-orchestrator בלבד).
- Image: `infra/Dockerfile.mobile-runtime` — Android SDK 35 + emulator +
  Expo/EAS CLIs (Flutter דרך `--build-arg WITH_FLUTTER=1`).
- Mac pool: מנוהל בנפרד (`Dockerfile.mobile-runtime-mac.md`); נדרש
  `IRONFLYER_MAC_POOL_ENABLED=1`.

### 8.3 PTY + File API
- מסוף בלייב נפתח כ-WebSocket ומחובר ל-sandbox; ה-cockpit מציג אותו
  כ-"open the hood" אבל לא כברירת מחדל.
- File API נצרך מה-vscode-extension וגם מ-clients/web (cloud IDE).

### 8.4 איזולציה
- כל workspace שייך ל-userID; כל store בודק owner.
- Reservation ב-wallet מקשרת execution → workspace → cost line.

---

## 10) חמשת ערוצי המובייל — First-Class

`StackDecision.Mobile.Kind`:

| Kind | תיאור | תלות בפול Mac | Free Tier |
|---|---|---|---|
| `expo` (recommended) | Expo Router + EAS Build | לא (EAS cloud signs) | ✓ |
| `react-native-bare` | RN שנפלט; native folders על disk | iOS דורש Mac | ✓ (אנדרואיד) |
| `android-native` | Kotlin + Jetpack Compose | לא | ✓ |
| `ios-native` | Swift + SwiftUI | חובה | **Pro** |
| `flutter` | Dart + Flutter | iOS דורש Mac | ✓ (אנדרואיד) |

### Mobile Ledger
`internal/business/ledger/mobile.go` מפריד את העלויות:
- `EntryMobileBuildMin`
- `EntryEmulatorMin`
- `EntryMacWorkspaceMin`
- `EntryEASBuildCredit`
- `EntryAppetizeMin`

ProfitGuard מפעיל reservation דרך `operations/wireup/profitguard_mobile.go`,
ויסרב להקצות Mac אם יוביל למרג'ין שלילי.

---

## 11) Visual-First Cockpit (Constitutional)

כל מסך תפעולי = מראה ויזואלית של state המנוע לפני שהמשתמש מגיע לטבלאות.

### 10.1 חוזה ראשי
- **Default view = visual.** DAG workflow, gate chips, cost bars, gauge למרג'ין,
  timeline ל-patches, graph של dependencies.
- **Mirrors, not decoration.** כל node ממופה לרכיב אמיתי (gate verdict, patch
  ID, ledger entry, artifact URL).
- **What's not closed end-to-end.** מסך הריצה חייב לציין במפורש איזה gate
  פתוח / patch לא הוחל / cost line לא תוקצב.
- **Collapsible.** ברירת מחדל גלאנס-בלבד; expand on hover/click.
- **Code is opt-in.** VS Code, GraphQL Sandbox, JSON timeline — נגישים
  בקליק אחד אך לא הנחיתה.

### 10.2 חוזה ויזואלי
- כל chart דרך `chartPalette` ב-`clients/web/src/components/charts/EChart.tsx`
  ועל `tokens.color.*` בלבד.
- ספריות כבדות (echarts, @xyflow/react, three.js) — `next/dynamic({ ssr:false })`
  בלבד.
- **Lime banned from CTA/chrome.** primary = violet; CTAs = coral→magenta→purple
  gradient; live/success = mint.
- **Login = Base44 split-layout.** `AuthShell.tsx` בלבד; אין centered card.

### 10.3 מסכים מובילים
- **Cockpit Home** — אוסף ריצות חיות כ-DAG, "what's open end-to-end" צמוד.
- **Studio** — Figma → tokens diff → screen map; כל קומפוננטה ניתנת להרחבה.
- **Execution** — phase nodes + gate chips + cost panel; smoke status חי.
- **Profit Dashboard** — margin first; scale gauges שנייה.
- **Wallet** — hold/debit/release timeline.

---

## 12) פרופילי הפעלה — Adaptive Governance

המערכת מותאמת לפי customer profile (נטען מ-`internal/customer`):

### 11.1 Startup Speed Mode
- Gates חובה: spec, code, lint, security, deploy, smoke.
- Warn → publish מותר.
- ProfitGuard: aggressive caching, Haiku-first fallback.
- אודיט: רק verdicts קריטיים.

### 11.2 Enterprise Control Mode
- Gates חובה: כל הסט המוצרי + lighthouse.
- Warn → publish רק עם הסבר אנושי כתוב.
- ProfitGuard: balanced; ועדיין דורש margin חיובי.
- אודיט מלא ב-`operations/audit` + export ל-`auditexport`.

### 11.3 Regulated Mode
- Gates חובה: כולל security ייעודי, ios_privacy_manifest, mobile_security,
  ו-policy gates נוספים מ-`operations/policy`.
- Warn → publish חסום אלא בחתימה כפולה.
- ProfitGuard: conservative; כל reservation עם מקדם safety.
- אודיט: גם evidence + chain-of-custody + retention policy.

---

## 13) Open Prompt Governance — חופש בתחום משמעת

המערכת מקבלת prompt פתוח אך מפעילה משמעת:

1. **Intent extraction** דרך `ai/ideaparser`.
2. **Ambiguity scoring** — מתחת לסף ⇒ ממשיכים; מעל ⇒ clarification מינימלי.
3. **Clarification policy** — לכל היותר שאלה אחת בכל סבב, רק כש-ambiguity
   חוסם את ה-Charter.
4. **Explainability** — כל החלטה אוטומטית מתועדת ב-`audit` עם הציטוט
   המקורי מה-prompt + ה-rule שהוחל.
5. **Schema discipline** — sub-agents מחזירים JSON-only, verdicts תמיד עם
   `verdict + citations + path/line`.

מטרה: gimble אבל לא chaos. המשתמש מרגיש חופשי; המערכת לא מאבדת שליטה.

---

## 14) דיווח אנושי בזמן אמת

תבנית עדכון אחידה (מופצת דרך `executionFeed` ב-GraphQL Subscriptions
ו-SSE על chat-stream):

1. **מצב נוכחי** — איזה שלב, איזה work-package, איזה gate פעיל.
2. **מה הושלם** — מאז העדכון הקודם.
3. **סיכון פתוח** — מה הולך לפיל / מה עלול לפיל.
4. **פעולה הבאה** — מי הסוכן הבא, איזה patch מתוכנן.
5. **ETA + Cost so far** — דקות עד הבא, $ עד עכשיו, צפי ל-EOF.

### דוגמה
> "השלמתי פירוק 12 מסכים ל-4 קומפוננטות ליבה ב-figma-translator. זיהיתי
> חוסר עקביות בטוקני spacing — תיקנתי לפי `packages/design-tokens`. כרגע
> ה-coder בונה auth flow; reviewer בתור. סיכון פתוח: secret של APNs
> חסר עבור gate `mobile_push_credentials`. ETA: 11 דק'. עלות עד כה:
> $0.42 (Sonnet 4.6 × 38%). margin צפויה לריצה הזו: +$1.14."

---

## 15) Closure Intelligence — הערכת סגירה חיה

בכל רגע המערכת עונה: כמה מההתחייבות נסגר, מה במסלול הקריטי, מה הסיכוי
לעמוד ב-ETA, ומה ה-margin הצפויה.

מודל בסיסי:

$$
\text{Closure Score} = \text{Scope Completion} \times \text{Quality Confidence}
\times \text{Integration Stability} \times \text{Margin Health}
$$

- **Scope Completion** — אחוז work-packages שהושלמו (`execution.service`).
- **Quality Confidence** — ממוצע gate verdicts משוקלל ב-severity.
- **Integration Stability** — יציבות חיבורי runtime + providers ב-5 דקות
  אחרונות.
- **Margin Health** — `(revenue − cost_so_far − cost_remaining_est)
  / revenue`. תחת 0.10 — דגל אדום.

המטריקה מוצגת ב-cockpit כ-gauge יחיד, עם drill-down לכל פקטור.

---

## 16) ProfitGuard — איך זה באמת עובד

`internal/business/profitguard` הוא ה-circuit breaker הכלכלי:

1. **Reservation** לפני ריצה: `hold = est_provider_cost + est_sandbox_cost
   + safety_margin`.
2. **Decision** לפני כל קריאה יקרה: על בסיס model tier, prompt length,
   workspace state, deploy target.
3. **Verdict** — `allow | downgrade | defer | refuse`:
   - `downgrade` ⇒ נופלים מ-Opus 4.7 ל-Sonnet 4.6, או מ-Mac pool ל-EAS
     cloud.
   - `defer` ⇒ ממתינים ל-cache או ל-batch.
   - `refuse` ⇒ עוצרים את ה-execution עם 402 מנומק.
4. **Reconciliation** בסוף ריצה: ה-ledger מסגיר את ה-hold מול הצריכה
   בפועל; variance עובר ל-`forecast` ל-tuning.

> ProfitGuard אינו "סוף". הוא מתערב בכל patch step, gate step, deploy
> step. החלטות שלו מתועדות ב-`audit` כדי שיהיה אפשר לבדוק היסטוריה.

---

## 17) Memory & Knowledge Graph

`internal/ai/memory` חושף `Store` יחיד עם 3 backends:
- `IRONFLYER_MEMORY_BACKEND=memory` — ring buffer in-process (default).
- `surreal` — SurrealDB knowledge graph (דורש `IRONFLYER_DB_DRIVER=surreal|hybrid`).
- `pgvector` — Postgres + pgvector (דורש migration `00017_pgvector_memory.sql`).

Embeddings: BAAI/bge-m3 כברירת מחדל דרך `ai/embeddings`.

מה נשמר: Charter, screen-map, decisions, gate verdicts, patches החלים,
provider responses (לקאשינג), ו-evidence trail.

מה לא נשמר: PII של משתמשים מעבר למינימום נדרש; secrets לעולם לא
נשמרים — רק references ל-secret store של ה-project (`Project.Secrets`).

---

## 18) Storage Split

- **Postgres** — ACID לארנק, ledger, executions, users. כל ה-state
  הכלכלי חי כאן.
- **SurrealDB** — knowledge graph לפרויקטים (`IRONFLYER_DB_DRIVER=surreal|hybrid`).
- **S3-compatible** — artifacts, deploy outputs, audit exports.
  `s3client.go` תומך `aws | r2 | minio`. R2 = zero egress, מומלץ
  לארטיפקטים גדולים.
- **Redis** (אופציונלי) — `operations/redisbus` ל-event fan-out מהיר.

---

## 19) Security, Abuse, Hardening

- **AppSec engine** — `operations/appsec` רץ ב-gate `security`. מכסה
  OWASP top-10, secret scanning, dependency audit, SAST.
- **Security report** — `operations/securityreport` מצרף evidence
  ל-audit export.
- **GraphQL Hardening** — `operations/gqlhardening` מגביל depth, complexity,
  introspection בפרודקשן.
- **Rate limiting + Abuse** — `operations/ratelimit` + `operations/abuse`
  עוצרים ניסיונות שימוש לרעה לפני שמגיעים ל-provider.
- **Policy** — `operations/policy` שומר policy decisions per-project.
- **Audit** — `operations/audit` + `auditexport` חתום למסלולי compliance.

---

## 20) Pricing & Wallet

מודל V22:
- **Stripe Checkout** ⇒ wallet top-up. ה-webhook היחיד שנשאר REST.
- **Hold-on-start** ⇒ debit-as-incurred ⇒ release-unused-on-commit.
- **Tiers** — Free (Expo / Android native / web), Pro (iOS native, Mac
  pool, scale).
- **Margin formula** — `revenue − providerCost − sandboxCost = margin`.
  Vault snapshot היא ה-SoT הפלטפורמטית.
- **402 Payment Required** עם `top_up_url` חוזר על כל execution שאין לה
  כיסוי.

---

## 21) Telemetry & KPI

KPIs ליבה (`internal/business/dashboards`):
1. **Profitable Completed Execution Rate (PCER)** — הצפון.
2. **Time to First Runnable** — מ-prompt ועד preview חי.
3. **Plan-to-Delivery Accuracy** — ETA צפוי / בפועל.
4. **First-pass Finish Loop Pass Rate** — כמה ריצות עברו gate-stack בלי
   replan.
5. **Interrupt Recovery Time** — זמן עד resume אחרי interrupt.
6. **Change Adaptation Latency** — זמן מ-Figma drift עד re-baseline.
7. **Human Escalation Rate** — כמה ריצות דרשו אדם.
8. **Post-release Defect Leakage** — bugs שדלפו אחרי deploy.
9. **Margin per Execution** — ממוצע ו-p10/p90.
10. **Top-up to First Run** — funnel coverage.

Metrics זורמים דרך Prometheus (`/metrics`), trace דרך OpenTelemetry
(`operations/tracing`), שגיאות ל-Sentry (`operations/sentryext`).

---

## 22) Mobile Visualization & Mirroring

`clients/scrcpy-bridge` מספק WebSocket bridge ל-Android device/emulator
real-time. ה-cockpit מציג אותו לצד ה-DAG, כך שהמשתמש רואה את ה-build
במסך אמיתי לפני שה-gate `mobile_build` חותם pass.

Appetize.io / EAS preview משולבים כ-fallback לפי tier.

---

## 23) Finish Loop פרקטי — מה רץ בכל delivery

### חובה תמיד
1. `lint` (go vet + tsc).
2. `security` (AppSec).
3. `budget` (ProfitGuard verdict).
4. `code` (reviewer verdict).
5. `deploy` (artifact + URL חי).
6. Smoke קצר על ה-flow המרכזי (`scripts/smoke.sh`).
7. Visual drift מול baseline (screenshots ב-`design-handoff-screenshots/`).

### חובה אם web
8. `lighthouse` (performance + a11y budget).

### חובה אם mobile
9. `mobile_build` + `mobile_size` + `mobile_bundle_analyzer`.
10. `mobile_security` + `ios_privacy_manifest` (אם iOS) +
    `mobile_push_credentials` (אם push).

### לא חובה בכל build
- E2E רחב; matrix של דפדפנים; בדיקות עומק יקרות בכל commit קטן.
- כל בדיקה עם cost > $X ללא justification ProfitGuard.

מדיניות: יותר guardrails חכמים, פחות ניפוח. **אין כתיבת/הרצת test files**
(constitutional). evidence היא מהאפליקציה החיה, מ-curl, ומ-smoke.

---

## 24) SLA & תחזית זמן

| מקטע | יעד | מקור מדידה |
|---|---|---|
| Plan ראשוני | 3–8 דקות | `planner` latency p50 |
| First runnable | 10–25 דקות | TTFR metric |
| Finish loop קצר (web בלבד) | +5–12 דקות | wowloop tracer |
| Finish loop מלא (web + mobile) | +20–40 דקות | wowloop tracer |
| Interrupt recovery | < 60 שנ' | execution.idempotency |
| Replan after Figma drift | < 90 שנ' | Sync layer |

SLA אלה הם יעדים; אם חורגים, ProfitGuard מסמן ב-`dashboards/adapters`
ו-`forecast` מעדכן מודלים.

---

## 25) Backlog יישום מדורג

### P0 — חייב לקרות עכשיו (Operating Foundation)
1. לקבע flow אחיד: upload → decompose → plan approval → execution →
   finish loop → publish.
2. לקבע חוזה פנימי: `.ironflyer/{screen_map,design_tokens,component_graph}.json`.
3. תבנית דיווח אנושית אחידה זורמת על `executionFeed`.
4. שחרור מותנה ב-gate stack המלא; אין override שקט.
5. ProfitGuard hooked לפני כל קריאה יקרה (כבר חי — לוודא כיסוי 100%).
6. **Anti-Bloat baseline**: הקמת Capability Atlas מאינדוקס ראשוני של הריפו,
   חיווט `reuse_check` + `dedup` + `deadcode` + `dep_graph` ל-Finish Loop,
   ו-publishing של `.ironflyer/architecture.json` עם layering הקיים.

### P1 — חיזוק תפעולי
1. Replan אוטומטי חכם ב-interrupts (Sync layer + plan-patch).
2. ניהול baseline ויזואלי לפי גרסאות (`design-handoff-screenshots/`).
3. אבחנה ברורה בין כשל מוצרי לכשל תשתיתי בלוגים.
4. Mac pool מאוטומט עם quota per tier.
5. Knowledge graph (Surreal) ב-Pro כברירת מחדל.
6. **Refactor Proposer** — codemod אוטומטי שמייצר patch-extract כשהגייט
   `dedup` או `complexity` מאתר ממצא חוזר, עם confirm-in-one-click.
7. **Code Health Dashboard** ב-`/cockpit/health` עם heatmaps, sparklines
   ו-reuse rate live.
8. **Memory stability lane** — `goleak` ב-smoke, heap diff פר deploy,
   `react-scan` opt-in לקומפוננטות חמות.

### P2 — Scale, Brand, Defensibility
1. פרופילי הפעלה מלאים (Startup / Enterprise / Regulated).
2. חיזוי ETA לפי היסטוריה (`business/forecast`).
3. אופטימיזציית עלות לפי תבניות שימוש (cache-first, Haiku-first).
4. Marketplace blueprints (`business/blueprints`) — סטארטרים שמתאימים
   ל-Figma intents.
5. Public proof board — gates + verdicts פתוחים כראייה תחרותית.
6. **Cross-project Atlas** — Capability Atlas חוצה-tenants (אנונימי) כדי
   להציע reuse גם מ-blueprints ציבוריים, לא רק מהריפו של הלקוח.
7. **Diff Economy public metric** — "LOC per Resolved Capability" כמטריקה
   ציבורית מול מתחרים.

---

## 26) תרחיש קצה-לקצה — סיפור עבודה אחד

1. **Top-up.** משתמש חדש נכנס דרך AuthShell, פותח wallet עם $20.
2. **Intake.** מעלה Figma של dashboard SaaS עם 14 מסכים + prompt:
   "תפתחו את זה בתור Next.js עם backend Go ומובייל Expo מינימלי".
3. **Decompose.** `figma-translator` מחלץ screen-map, מזהה 4 קומפוננטות
   מרכזיות, מוצא 3 token mismatches מול הרפרנס הנעול.
4. **Plan.** `planner` מציע 11 work-packages עם ETA 38 דק' ועלות צפויה
   $7.40. `architect` מאשר data model.
5. **Approve.** המשתמש מאשר; ProfitGuard hold $9.20 (כולל safety).
6. **Execute.** `coder` מתחיל; `reviewer` חותם patches; `mobile-coder`
   מטפל ב-Expo.
7. **Interrupt.** דקה 14 — המשתמש משנה את ה-Figma (הוסיף onboarding
   step). המערכת קולטת ב-Sync layer, מציעה plan-patch (+5 דק', +$0.90),
   המשתמש מאשר ב-קליק.
8. **Gates.** `lint`, `code`, `security`, `lighthouse` — pass.
   `mobile_build` — warn (Expo SDK 51 deprecation). `deploy` — pass.
9. **Publish.** Vercel preview URL חי; APK preview דרך EAS.
   `scripts/smoke.sh` ירוק.
10. **Report.** דוח סופי: 11/11 WPs נסגרו, 1 warn מתועד, ETA דויק
    ב-92%, margin בפועל +$1.83.

---

## 27) מיפוי לקוד אמיתי — איפה זה חי

| נושא | מיקום |
|---|---|
| חוקי הברזל | `core/orchestrator/internal/business/{wallet,ledger,profitguard,profitguardbridge}` |
| Gate definitions | `core/orchestrator/internal/ai/domain/types.go` |
| Finisher engine | `core/orchestrator/internal/business/wowloop/{sources,builder,bundle}.go` |
| Patches | `core/orchestrator/internal/operations/patch` |
| Execution | `core/orchestrator/internal/business/execution/{service,memory,idempotency}.go` |
| Agents | `core/orchestrator/internal/ai/agents/agents.yaml` + `load.go` |
| Providers + router | `core/orchestrator/internal/ai/providers` |
| Memory backends | `core/orchestrator/internal/ai/memory` |
| AppSec | `core/orchestrator/internal/operations/{appsec,securityreport}` |
| Audit | `core/orchestrator/internal/operations/{audit,auditexport}` |
| GraphQL schema | `core/orchestrator/internal/operations/graph/schema/*.graphql` |
| Resolvers | `core/orchestrator/internal/operations/graph/resolver/` |
| Deploy | `core/orchestrator/internal/operations/deploy` |
| Mobile (orchestrator side) | `core/orchestrator/internal/operations/mobile` |
| Mobile (runtime side) | `core/runtime/internal/suppliers/mobile` |
| Blueprints | `core/orchestrator/internal/business/blueprints` |
| Dashboards | `core/orchestrator/internal/business/dashboards` |
| Forecast | `core/orchestrator/internal/business/forecast` |
| Storage / S3 | `core/orchestrator/internal/operations/storage/s3client.go` |
| Cockpit UI | `clients/web/src/...` (Cockpit, Studio, Execution, Profit, Wallet) |
| VSCode thin client | `clients/vscode-extension/` |
| Mobile mirror | `clients/scrcpy-bridge/` |
| Tokens | `packages/design-tokens/index.ts` |
| SDK | `packages/sdk/` |
| Starters | `templates/starters/{react-native-expo,android-kotlin,ios-swift}` |
| Smoke | `scripts/smoke.sh` |

---

## 28) הצבעת איכות תחרותית

Lovable, Base44, Bolt, Replit Agent, v0 מוכרים "תאר רעיון, קבל אפליקציה".
Ironflyer מוכר את **המשמעת הפרודקציונית** שחסרה להם:

1. **Gates שחוסמים** — verdict אמיתי, לא אנימציה.
2. **Patches שמותר לסקור** — אף שינוי לא נכנס בלי lifecycle approve.
3. **Live cost visibility** — שקיפות עד הסנט.
4. **Wallet prepaid** — אין surprises בסוף החודש.
5. **Real Linux workspaces** + Mac pool — לא רק browser sandbox.
6. **ProfitGuard** — circuit breaker כלכלי לפני כל קריאה יקרה.
7. **Native mobile** — iOS, Android, Expo, Flutter כ-first-class, לא wrap.
8. **Constitutional design** — אין drift; tokens מובילים.
9. **No-tests-ever** — evidence חי, לא test-theater.
10. **Visual-first cockpit** — מראה לאופרטור מה לא סגור end-to-end.
11. **Anti-Bloat Engine** — Reuse-First preflight, Capability Atlas,
    dedup/deadcode/complexity/arch_boundary gates, Refactor Proposer,
    ו-Code Health Dashboard. מתחרים כותבים עוד קוד; אנחנו כותבים פחות
    ויותר טוב.

קונקרטית, שפת המוצר היא של מהנדס בכיר: `gate verdict`, `patch`, `wallet`,
`ledger entry`, `Docker workspace`, `owner check`, `deploy artifact`,
`completion score`, `blueprint`, `repair recipe`, `ProfitGuard decision`.

---

## 29) סיכום חד

החזון מתממש ברגע שמפסיקים לחשוב "פיצ'ר" ומתחילים לחשוב **"מערכת תפעול מוצר רווחית"**:

- המשתמש מעלה Figma + prompt חופשי.
- המערכת מתכננת, בונה, מסתגלת, מסנכרנת, מדווחת, סוגרת — ורושמת כל סנט.
- שחרור נקבע לפי gate-stack חכם.
- ProfitGuard מבטיח שכל קריאה יקרה משאירה את ה-margin חיובי.
- הכל גמיש לפרויקט, לכלים, ולמציאות משתנה — אבל הכללים החוקתיים לא זזים.

> **Profitable Completed Execution Rate** הוא המדד היחיד שחשוב.
> כל שאר המסמכים, ההחלטות, וה-features שירותים שלו.

זה המסלול ל-End-to-End אמיתי, ברמה עולמית, ועם משמעת כלכלית.
