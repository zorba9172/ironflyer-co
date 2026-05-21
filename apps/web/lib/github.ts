'use client';

import { auth } from './auth';

const base = '/api/orchestrator';

export interface GitHubStatus {
  connected: boolean;
  login?: string;
  scope?: string;
}

export interface GitHubRepo {
  id: number;
  name: string;
  fullName: string;
  description?: string;
  private: boolean;
  defaultBranch: string;
  htmlUrl: string;
  updatedAt?: string;
}

async function jget<T>(path: string): Promise<T> {
  const res = await fetch(`${base}${path}`, {
    headers: { ...auth.authHeader() },
    cache: 'no-store',
  });
  if (res.status === 503) throw new Error('github-disabled');
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

/** Public URL the browser navigates to in order to sign in with GitHub. */
export const githubLoginStartURL = `${base}/auth/github/login/start`;

export const githubApi = {
  /** Status + login of the connected GitHub account, if any. */
  me: () => jget<GitHubStatus>('/integrations/github/me'),

  /** Repos accessible to the authenticated GitHub user. */
  repos: () => jget<GitHubRepo[]>('/integrations/github/repos'),

  /** Open a top-level browser navigation to the GitHub consent screen. */
  async startConnect(): Promise<void> {
    // The `?redirect=true` form makes the orchestrator 302 the browser to
    // github.com directly, which is what most users expect.
    const url = `${base}/auth/github/start?redirect=true`;
    // We need the Authorization header to identify the user — but a plain
    // window.location.href won't carry it. Fetch the URL form first, then
    // navigate. (The orchestrator memorises the state→userID mapping.)
    const res = await fetch(`${base}/auth/github/start`, {
      headers: { ...auth.authHeader() },
      cache: 'no-store',
    });
    if (res.status === 503) throw new Error('github-disabled');
    if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
    const { authUrl } = await res.json();
    window.location.href = authUrl;
    void url; // referenced for the dev reader; the redirect form is unused on web
  },

  async disconnect(): Promise<void> {
    const res = await fetch(`${base}/integrations/github`, {
      method: 'DELETE',
      headers: { ...auth.authHeader() },
    });
    if (!res.ok && res.status !== 204) throw new Error(await res.text());
  },

  async connectRepo(projectId: string, repo: GitHubRepo): Promise<void> {
    const res = await fetch(`${base}/projects/${projectId}/connect-github`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...auth.authHeader() },
      body: JSON.stringify({
        owner: repo.fullName.split('/')[0],
        repo: repo.name,
        fullName: repo.fullName,
        defaultBranch: repo.defaultBranch,
        htmlUrl: repo.htmlUrl,
      }),
    });
    if (!res.ok) throw new Error(await res.text());
  },

  async disconnectRepo(projectId: string): Promise<void> {
    const res = await fetch(`${base}/projects/${projectId}/connect-github`, {
      method: 'DELETE',
      headers: { ...auth.authHeader() },
    });
    if (!res.ok) throw new Error(await res.text());
  },

  /** Clone the project's linked repo into the given workspace. */
  async cloneIntoWorkspace(projectId: string, workspaceId: string, ref?: string, subdir?: string): Promise<void> {
    const res = await fetch(`${base}/projects/${projectId}/clone-into-workspace`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...auth.authHeader() },
      body: JSON.stringify({ workspaceId, ref, subdir }),
    });
    if (!res.ok) throw new Error(await res.text());
  },
};
