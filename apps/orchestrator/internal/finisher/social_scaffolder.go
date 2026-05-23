// SocialScaffolder — feed / posts / likes / follows on Next.js. The
// pack assumes Supabase auth is the identity source (matching the
// AuthScaffolder default) and that a Postgres-shaped DB is reachable
// through process.env.DATABASE_URL.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type SocialScaffolder struct{}

func (SocialScaffolder) Name() string { return "social-feed" }

func (SocialScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "social") {
		return true
	}
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	if strings.Contains(desc, "social network") || strings.Contains(desc, "social media") ||
		strings.Contains(desc, "community") || strings.Contains(desc, "newsfeed") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "feed") || strings.Contains(body, "follow") ||
			strings.Contains(body, "like ") || strings.Contains(body, "comment") ||
			strings.Contains(body, "share") || strings.Contains(body, "profile") ||
			strings.Contains(body, "post ") || strings.HasSuffix(body, "post") ||
			strings.Contains(body, "posts") {
			return true
		}
	}
	for _, e := range p.Spec.DataModel {
		n := strings.ToLower(e.Name)
		if n == "post" || n == "comment" || n == "like" || n == "follow" || n == "feed" {
			return true
		}
	}
	return false
}

func (SocialScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"lib/social/types.ts": `// Canonical social-graph types. The DB columns mirror these field
// names so the helpers in lib/social/feed.ts can map rows 1:1.

export interface Post {
  id: string;
  authorId: string;
  body: string;            // markdown allowed, sanitise on render
  mediaUrl?: string;
  createdAt: string;       // ISO 8601, server-set
  // Denormalised counters — kept in sync by the like/comment/share
  // routes so the feed query stays a single SELECT.
  likeCount: number;
  commentCount: number;
  shareCount: number;
}

export interface Comment {
  id: string;
  postId: string;
  authorId: string;
  body: string;
  createdAt: string;
}

export interface Like {
  postId: string;
  userId: string;
  createdAt: string;
}

export interface Follow {
  followerId: string;
  followeeId: string;
  createdAt: string;
}

export interface FeedPage {
  posts: Post[];
  // Opaque cursor for the next page; null when the feed is exhausted.
  nextCursor: string | null;
}
`,
		"lib/social/feed.ts": `// Server-side feed helpers. The default implementation talks to the
// project's Postgres via the standard pg client; swap the driver
// out for Drizzle / Prisma / Supabase client without changing the
// public function signatures.
import { Pool } from 'pg';
import type { Post, FeedPage } from './types';

let pool: Pool | null = null;
function db() {
  if (!pool) pool = new Pool({ connectionString: process.env.DATABASE_URL });
  return pool;
}

const PAGE_SIZE = 20;

// Cursor is "<ISO timestamp>|<post id>"; we sort newest-first so the
// cursor encodes the createdAt of the last row of the previous page,
// with id as the tiebreaker.
function encodeCursor(p: Post): string {
  return Buffer.from(p.createdAt + '|' + p.id).toString('base64url');
}
function decodeCursor(c: string | null): { ts: string; id: string } | null {
  if (!c) return null;
  const raw = Buffer.from(c, 'base64url').toString('utf8');
  const [ts, id] = raw.split('|');
  if (!ts || !id) return null;
  return { ts, id };
}

export async function getHomeFeed(userId: string, cursor: string | null): Promise<FeedPage> {
  const cur = decodeCursor(cursor);
  const sqlWithCursor =
    "SELECT * FROM posts " +
    "WHERE author_id IN (SELECT followee_id FROM follows WHERE follower_id = $1) " +
    "AND (created_at, id) < ($2::timestamptz, $3) " +
    "ORDER BY created_at DESC, id DESC LIMIT $4";
  const sqlNoCursor =
    "SELECT * FROM posts " +
    "WHERE author_id IN (SELECT followee_id FROM follows WHERE follower_id = $1) " +
    "ORDER BY created_at DESC, id DESC LIMIT $2";
  const rows = await db().query<Post>(
    cur ? sqlWithCursor : sqlNoCursor,
    cur ? [userId, cur.ts, cur.id, PAGE_SIZE + 1] : [userId, PAGE_SIZE + 1],
  );
  const posts = rows.rows.slice(0, PAGE_SIZE);
  const nextCursor = rows.rows.length > PAGE_SIZE ? encodeCursor(posts[posts.length - 1]) : null;
  return { posts, nextCursor };
}

export async function createPost(authorId: string, body: string, mediaUrl?: string): Promise<Post> {
  const insertSql =
    "INSERT INTO posts (author_id, body, media_url, created_at, like_count, comment_count, share_count) " +
    "VALUES ($1, $2, $3, now(), 0, 0, 0) RETURNING *";
  const r = await db().query<Post>(
    insertSql,
    [authorId, body, mediaUrl ?? null],
  );
  return r.rows[0];
}

export async function toggleLike(postId: string, userId: string): Promise<{ liked: boolean; likeCount: number }> {
  const client = await db().connect();
  try {
    await client.query('BEGIN');
    const existing = await client.query('SELECT 1 FROM likes WHERE post_id = $1 AND user_id = $2', [postId, userId]);
    let liked: boolean;
    if (existing.rowCount && existing.rowCount > 0) {
      await client.query('DELETE FROM likes WHERE post_id = $1 AND user_id = $2', [postId, userId]);
      await client.query('UPDATE posts SET like_count = greatest(like_count - 1, 0) WHERE id = $1', [postId]);
      liked = false;
    } else {
      await client.query('INSERT INTO likes (post_id, user_id, created_at) VALUES ($1, $2, now())', [postId, userId]);
      await client.query('UPDATE posts SET like_count = like_count + 1 WHERE id = $1', [postId]);
      liked = true;
    }
    const c = await client.query<{ like_count: number }>('SELECT like_count FROM posts WHERE id = $1', [postId]);
    await client.query('COMMIT');
    return { liked, likeCount: c.rows[0]?.like_count ?? 0 };
  } catch (e) {
    await client.query('ROLLBACK');
    throw e;
  } finally {
    client.release();
  }
}
`,
		"app/api/feed/route.ts": `// GET /api/feed?cursor=<opaque> — paginated home feed for the
// signed-in user. Auth comes from the existing supabase auth scaffold.
import { NextRequest, NextResponse } from 'next/server';
import { createClient } from '../../../lib/supabase/server';
import { getHomeFeed } from '../../../lib/social/feed';

export async function GET(req: NextRequest) {
  const supabase = await createClient();
  const { data: { user } } = await supabase.auth.getUser();
  if (!user) return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });
  const cursor = req.nextUrl.searchParams.get('cursor');
  const page = await getHomeFeed(user.id, cursor);
  return NextResponse.json(page);
}
`,
		"app/api/posts/route.ts": `// POST /api/posts — create a new post. Body { body, mediaUrl? }.
import { NextRequest, NextResponse } from 'next/server';
import { createClient } from '../../../lib/supabase/server';
import { createPost } from '../../../lib/social/feed';

export async function POST(req: NextRequest) {
  const supabase = await createClient();
  const { data: { user } } = await supabase.auth.getUser();
  if (!user) return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });
  const { body, mediaUrl } = (await req.json()) as { body?: string; mediaUrl?: string };
  if (!body || body.trim().length === 0) {
    return NextResponse.json({ error: 'body is required' }, { status: 400 });
  }
  if (body.length > 4000) {
    return NextResponse.json({ error: 'body too long' }, { status: 400 });
  }
  const post = await createPost(user.id, body.trim(), mediaUrl);
  return NextResponse.json({ post }, { status: 201 });
}
`,
		"app/api/posts/[id]/like/route.ts": `// POST /api/posts/[id]/like — toggle a like for the current user.
// Returns { liked: boolean, likeCount: number }.
import { NextRequest, NextResponse } from 'next/server';
import { createClient } from '../../../../../lib/supabase/server';
import { toggleLike } from '../../../../../lib/social/feed';

export async function POST(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  const supabase = await createClient();
  const { data: { user } } = await supabase.auth.getUser();
  if (!user) return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });
  const result = await toggleLike(id, user.id);
  return NextResponse.json(result);
}
`,
	}
	contract := `Social scaffold: Next.js (app router) + Postgres + Supabase auth.

Already provisioned:
- /lib/social/types.ts                  → Post / Comment / Like / Follow
- /lib/social/feed.ts                   → server helpers (pg)
- /app/api/feed/route.ts                → GET cursor-paginated feed
- /app/api/posts/route.ts               → POST create
- /app/api/posts/[id]/like/route.ts     → POST toggle like

Pagination:
- Cursor is base64url("<createdAt>|<id>"), newest-first. The (createdAt,
  id) composite is the stable sort key — id breaks ties when two posts
  share a timestamp.

Counters:
- likeCount / commentCount / shareCount are denormalised onto posts so
  the feed query stays a single SELECT. Every mutation that creates or
  deletes a Like/Comment/Share MUST update the counter in the same
  transaction (see toggleLike for the pattern).

Auth:
- The existing supabase server.ts (see .ironflyer/auth.md) is the
  identity source. Do NOT reimplement sessions here.

Required schema (Postgres):
    posts(id uuid pk, author_id uuid, body text, media_url text,
          created_at timestamptz, like_count int, comment_count int,
          share_count int)
    comments(id uuid pk, post_id uuid, author_id uuid, body text, created_at timestamptz)
    likes(post_id uuid, user_id uuid, created_at timestamptz, primary key (post_id, user_id))
    follows(follower_id uuid, followee_id uuid, created_at timestamptz, primary key (follower_id, followee_id))
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
