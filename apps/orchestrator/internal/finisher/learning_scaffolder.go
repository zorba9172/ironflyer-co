// LearningScaffolder — LMS-style course/lesson/quiz/enrollment pack
// on Next.js. Plays nicely with the Stripe scaffold for paid courses
// and the auth scaffold for user identity.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type LearningScaffolder struct{}

func (LearningScaffolder) Name() string { return "learning-courses" }

func (LearningScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	combined := stack + " " + desc
	if strings.Contains(combined, "course") || strings.Contains(combined, "lesson") ||
		strings.Contains(combined, "quiz") || strings.Contains(combined, "lms") ||
		strings.Contains(combined, "education") || strings.Contains(combined, "e-learning") ||
		strings.Contains(combined, "elearning") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "enrol") || strings.Contains(body, "enroll") ||
			strings.Contains(body, "progress") || strings.Contains(body, "certificate") ||
			strings.Contains(body, "course") || strings.Contains(body, "lesson") ||
			strings.Contains(body, "quiz") {
			return true
		}
	}
	for _, e := range p.Spec.DataModel {
		n := strings.ToLower(e.Name)
		if n == "course" || n == "lesson" || n == "quiz" || n == "enrollment" || n == "progress" {
			return true
		}
	}
	return false
}

func (LearningScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"lib/learning/types.ts": `// Canonical learning-platform types. Lesson.position orders lessons
// inside a Course; Progress is keyed (userId, lessonId) so resuming
// is O(1). Certificate eligibility = every lesson in the course has
// a Progress row with completedAt set.

export interface Course {
  id: string;
  slug: string;
  title: string;
  summary: string;
  coverImageUrl?: string;
  priceCents: number;           // 0 = free
  currency: 'USD' | 'EUR' | 'ILS';
  stripePriceId?: string;       // set after Stripe sync, paid courses only
  published: boolean;
}

export interface Lesson {
  id: string;
  courseId: string;
  slug: string;
  position: number;             // 1-based ordering inside the course
  title: string;
  bodyMarkdown: string;
  videoUrl?: string;
  estimatedMinutes: number;
}

export interface Quiz {
  id: string;
  lessonId: string;
  questions: QuizQuestion[];
  passingScore: number;         // 0..100
}

export interface QuizQuestion {
  id: string;
  prompt: string;
  options: string[];
  correctIndex: number;
}

export interface Enrollment {
  id: string;
  userId: string;
  courseId: string;
  enrolledAt: string;
  // PaidViaStripeSessionId is set when the enrollment was created
  // through the Stripe Checkout webhook — kept for receipts + refunds.
  paidViaStripeSessionId?: string;
}

export interface Progress {
  userId: string;
  lessonId: string;
  startedAt: string;
  completedAt?: string;
  quizScore?: number;           // 0..100, present once the quiz is taken
}
`,
		"app/courses/page.tsx": `// /courses — public catalogue. Lists every published course.
import Link from 'next/link';
import type { Course } from '../../lib/learning/types';

async function loadCourses(): Promise<Course[]> {
  const res = await fetch((process.env.NEXT_PUBLIC_SITE_URL ?? '') + '/api/courses', {
    cache: 'no-store',
  });
  if (!res.ok) return [];
  const { courses } = (await res.json()) as { courses: Course[] };
  return courses;
}

export default async function CatalogPage() {
  const courses = await loadCourses();
  return (
    <main style={{ padding: 40, color: '#fff', background: '#0d0e0f', minHeight: '100vh' }}>
      <h1 style={{ color: '#c7ff00' }}>Courses</h1>
      {courses.length === 0 ? (
        <p>No courses published yet.</p>
      ) : (
        <ul style={{ display: 'grid', gap: 16, padding: 0, listStyle: 'none' }}>
          {courses.map((c) => (
            <li key={c.id} style={{ padding: 16, border: '1px solid #1a1b1d', borderRadius: 8 }}>
              <Link href={'/courses/' + c.slug} style={{ color: '#c7ff00' }}>
                <h2 style={{ margin: 0 }}>{c.title}</h2>
              </Link>
              <p style={{ marginTop: 8 }}>{c.summary}</p>
              <p style={{ color: '#888' }}>
                {c.priceCents === 0 ? 'Free' : (c.priceCents / 100).toFixed(2) + ' ' + c.currency}
              </p>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
`,
		"app/courses/[slug]/page.tsx": `// /courses/[slug] — course detail + lesson outline + enrol CTA.
import Link from 'next/link';
import { notFound } from 'next/navigation';
import type { Course, Lesson } from '../../../lib/learning/types';

async function loadCourse(slug: string): Promise<{ course: Course; lessons: Lesson[] } | null> {
  const res = await fetch((process.env.NEXT_PUBLIC_SITE_URL ?? '') + '/api/courses?slug=' + slug, {
    cache: 'no-store',
  });
  if (!res.ok) return null;
  return (await res.json()) as { course: Course; lessons: Lesson[] };
}

export default async function CourseDetail({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const data = await loadCourse(slug);
  if (!data) notFound();
  const { course, lessons } = data;
  return (
    <main style={{ padding: 40, color: '#fff', background: '#0d0e0f', minHeight: '100vh' }}>
      <h1 style={{ color: '#c7ff00' }}>{course.title}</h1>
      <p>{course.summary}</p>
      <form action="/api/enrollments" method="post">
        <input type="hidden" name="courseId" value={course.id} />
        <button type="submit">
          {course.priceCents === 0 ? 'Enrol for free' : ('Buy for ' + (course.priceCents / 100).toFixed(2) + ' ' + course.currency)}
        </button>
      </form>
      <h2 style={{ marginTop: 32 }}>Lessons</h2>
      <ol>
        {lessons.map((l) => (
          <li key={l.id}>
            <Link href={'/courses/' + course.slug + '/lessons/' + l.id} style={{ color: '#c7ff00' }}>
              {l.title}
            </Link>{' '}
            <span style={{ color: '#888' }}>· {l.estimatedMinutes} min</span>
          </li>
        ))}
      </ol>
    </main>
  );
}
`,
		"app/courses/[slug]/lessons/[lessonId]/page.tsx": `// /courses/[slug]/lessons/[lessonId] — single lesson view. Marks
// progress on mount via a server action; renders markdown and an
// optional video.
import { notFound } from 'next/navigation';
import type { Lesson } from '../../../../../lib/learning/types';

async function loadLesson(courseSlug: string, lessonId: string): Promise<Lesson | null> {
  const res = await fetch(
    (process.env.NEXT_PUBLIC_SITE_URL ?? '') + '/api/courses?slug=' + courseSlug + '&lessonId=' + lessonId,
    { cache: 'no-store' },
  );
  if (!res.ok) return null;
  const { lesson } = (await res.json()) as { lesson?: Lesson };
  return lesson ?? null;
}

export default async function LessonPage({
  params,
}: {
  params: Promise<{ slug: string; lessonId: string }>;
}) {
  const { slug, lessonId } = await params;
  const lesson = await loadLesson(slug, lessonId);
  if (!lesson) notFound();
  return (
    <main style={{ padding: 40, color: '#fff', background: '#0d0e0f', minHeight: '100vh' }}>
      <h1 style={{ color: '#c7ff00' }}>{lesson.title}</h1>
      {lesson.videoUrl ? (
        <video src={lesson.videoUrl} controls style={{ width: '100%', maxWidth: 720, borderRadius: 8 }} />
      ) : null}
      <article style={{ whiteSpace: 'pre-wrap', marginTop: 24 }}>{lesson.bodyMarkdown}</article>
      <form action={'/api/enrollments?lessonId=' + lesson.id + '&action=complete'} method="post">
        <button type="submit" style={{ marginTop: 24 }}>Mark complete</button>
      </form>
    </main>
  );
}
`,
		"app/api/courses/route.ts": `// GET /api/courses           → list published courses
// GET /api/courses?slug=...  → course + lessons (or single lesson with lessonId)
import { NextRequest, NextResponse } from 'next/server';
import { Pool } from 'pg';
import type { Course, Lesson } from '../../../lib/learning/types';

let pool: Pool | null = null;
function db() {
  if (!pool) pool = new Pool({ connectionString: process.env.DATABASE_URL });
  return pool;
}

export async function GET(req: NextRequest) {
  const slug = req.nextUrl.searchParams.get('slug');
  const lessonId = req.nextUrl.searchParams.get('lessonId');
  if (slug && lessonId) {
    const r = await db().query<Lesson>('SELECT * FROM lessons WHERE id = $1', [lessonId]);
    return NextResponse.json({ lesson: r.rows[0] ?? null });
  }
  if (slug) {
    const c = await db().query<Course>('SELECT * FROM courses WHERE slug = $1 AND published = true', [slug]);
    const course = c.rows[0];
    if (!course) return NextResponse.json({ error: 'not found' }, { status: 404 });
    const l = await db().query<Lesson>(
      'SELECT * FROM lessons WHERE course_id = $1 ORDER BY position ASC',
      [course.id],
    );
    return NextResponse.json({ course, lessons: l.rows });
  }
  const r = await db().query<Course>(
    'SELECT * FROM courses WHERE published = true ORDER BY title ASC',
  );
  return NextResponse.json({ courses: r.rows });
}
`,
		"app/api/enrollments/route.ts": `// POST /api/enrollments                              → enrol in courseId
// POST /api/enrollments?lessonId=...&action=complete  → mark lesson done
//
// Free courses enrol immediately. Paid courses redirect into a Stripe
// Checkout Session whose success_url is /api/enrollments/confirm —
// the webhook (see .ironflyer/stripe.md) is the source of truth.
import { NextRequest, NextResponse } from 'next/server';
import { Pool } from 'pg';
import { createClient } from '../../../lib/supabase/server';
import { getStripe } from '../../../lib/stripe/server';
import type { Course } from '../../../lib/learning/types';

let pool: Pool | null = null;
function db() {
  if (!pool) pool = new Pool({ connectionString: process.env.DATABASE_URL });
  return pool;
}

export async function POST(req: NextRequest) {
  const supabase = await createClient();
  const { data: { user } } = await supabase.auth.getUser();
  if (!user) return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });

  const lessonId = req.nextUrl.searchParams.get('lessonId');
  const action = req.nextUrl.searchParams.get('action');
  if (lessonId && action === 'complete') {
    await db().query(
      "INSERT INTO progress (user_id, lesson_id, started_at, completed_at) " +
      "VALUES ($1, $2, now(), now()) " +
      "ON CONFLICT (user_id, lesson_id) DO UPDATE SET completed_at = now()",
      [user.id, lessonId],
    );
    return NextResponse.redirect(new URL(req.headers.get('referer') ?? '/courses', req.url));
  }

  const form = await req.formData();
  const courseId = String(form.get('courseId') ?? '');
  if (!courseId) return NextResponse.json({ error: 'courseId is required' }, { status: 400 });

  const c = await db().query<Course>('SELECT * FROM courses WHERE id = $1', [courseId]);
  const course = c.rows[0];
  if (!course) return NextResponse.json({ error: 'course not found' }, { status: 404 });

  if (course.priceCents === 0) {
    await db().query(
      "INSERT INTO enrollments (user_id, course_id, enrolled_at) VALUES ($1, $2, now()) " +
      "ON CONFLICT (user_id, course_id) DO NOTHING",
      [user.id, course.id],
    );
    return NextResponse.redirect(new URL('/courses/' + course.slug, req.url));
  }

  if (!course.stripePriceId) {
    return NextResponse.json({ error: 'course is paid but has no stripePriceId' }, { status: 500 });
  }
  const session = await getStripe().checkout.sessions.create({
    mode: 'payment',
    line_items: [{ price: course.stripePriceId, quantity: 1 }],
    metadata: { courseId: course.id, userId: user.id },
    success_url: (process.env.NEXT_PUBLIC_SITE_URL ?? '') + '/courses/' + course.slug,
    cancel_url: (process.env.NEXT_PUBLIC_SITE_URL ?? '') + '/courses/' + course.slug,
  });
  return NextResponse.redirect(session.url ?? '/courses');
}
`,
	}
	contract := `Learning scaffold: Next.js (app router) + Postgres + Stripe + Supabase auth.

Already provisioned:
- /lib/learning/types.ts                                    → Course / Lesson / Quiz / Enrollment / Progress
- /app/courses/page.tsx                                     → public catalogue
- /app/courses/[slug]/page.tsx                              → course detail + outline + enrol CTA
- /app/courses/[slug]/lessons/[lessonId]/page.tsx           → lesson view
- /app/api/courses/route.ts                                 → catalogue + detail API
- /app/api/enrollments/route.ts                             → enrol + progress + Stripe redirect

Progress tracking:
- progress(user_id, lesson_id) is the unique key. completed_at = now()
  marks completion. "Mark complete" on the lesson page POSTs to
  /api/enrollments?lessonId=...&action=complete.

Certificate gating:
- A user qualifies for a certificate on a course when every Lesson in
  the course has a Progress row with completed_at set for that user.
  Implement /api/certificates/[courseId] as a single SELECT EXISTS
  guard against missing progress rows; render the certificate as a
  server-side React → image (satori) or a printable PDF page.

Paid courses (Stripe wiring):
- A course with priceCents > 0 must have stripePriceId set. Run a
  syncCoursesToStripe job (use lib/stripe/server.ts) the first time a
  course is published. The Stripe webhook for checkout.session.completed
  is the place to INSERT the enrollments row — do NOT trust the
  success_url redirect.

Required schema (Postgres):
    courses(id uuid pk, slug text unique, title text, summary text,
            cover_image_url text, price_cents int, currency text,
            stripe_price_id text, published bool)
    lessons(id uuid pk, course_id uuid, slug text, position int,
            title text, body_markdown text, video_url text,
            estimated_minutes int)
    quizzes(id uuid pk, lesson_id uuid, questions jsonb, passing_score int)
    enrollments(id uuid pk, user_id uuid, course_id uuid,
                enrolled_at timestamptz, paid_via_stripe_session_id text,
                unique (user_id, course_id))
    progress(user_id uuid, lesson_id uuid, started_at timestamptz,
             completed_at timestamptz, quiz_score int,
             primary key (user_id, lesson_id))
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
