// GitHub App authentication + GraphQL client for creating issues.
// Research decision: hand-rolled client over octokit (see research.md §R1).

import { SignJWT, importPKCS8 } from "jose";

import type { Env } from "./env";

const GITHUB_API = "https://api.github.com";
const USER_AGENT = "my-gather-feedback-worker";
const LABEL_CACHE_TTL_SECONDS = 86_400;

// Per FR-014 + contracts/api.md §"504 Gateway Timeout": every GitHub
// fetch (token exchange, label resolution, createIssue mutation) is
// bounded by an AbortController so a stalled GitHub side never hangs
// the Worker isolate past Cloudflare's platform timeout. 10 s sits
// inside the browser's 15 s wall-clock budget (FR-008) so a single
// slow GitHub call surfaces as a clean Worker-side 504 within the
// browser's window, not a browser AbortError.
const GITHUB_FETCH_TIMEOUT_MS = 10_000;

export interface CreateIssueInput {
  repositoryId: string;
  title: string;
  body: string;
  labelIds: string[];
}

export interface CreatedIssue {
  id: string;
  number: number;
  url: string;
}

export class GitHubError extends Error {
  readonly status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "GitHubError";
    this.status = status;
  }
}

// GitHubTimeoutError is intentionally NOT a subclass of GitHubError
// so the index.ts handler can branch cleanly on
// `instanceof GitHubTimeoutError` for a 504 response BEFORE the
// existing `instanceof GitHubError` check that returns 503.
export class GitHubTimeoutError extends Error {
  readonly status: number;
  readonly timeoutMs: number;
  constructor(timeoutMs: number = GITHUB_FETCH_TIMEOUT_MS) {
    super(`GitHub fetch exceeded ${timeoutMs} ms.`);
    this.name = "GitHubTimeoutError";
    this.status = 504;
    this.timeoutMs = timeoutMs;
  }
}

// fetchWithTimeout wraps the global fetch with an AbortController
// scheduled to fire at `timeoutMs`. On abort, the underlying fetch
// rejects with an AbortError DOMException; we re-throw that as a
// GitHubTimeoutError so callers can branch on a typed error.
//
// IMPORTANT — the timeout MUST cover the body-read phase too.
// `fetch()` resolves as soon as response headers arrive; the body is
// streamed lazily. If we returned the raw Response and let callers
// call `.text()` / `.json()` after `clearTimeout(timer)` ran, a
// GitHub side that delivered headers in 50 ms but then dribbled the
// body for tens of seconds would slip past the 10 s budget without
// raising GitHubTimeoutError. Codex / Copilot review on PR #30 caught
// this. The fix: drain the body to an ArrayBuffer INSIDE the timeout
// window (so an abort during body delivery is still raised as
// AbortError → GitHubTimeoutError), then return a fresh Response
// carrying the buffered body so the caller's `.text()`/`.json()` is
// pure CPU and cannot stall on the network. The doubled buffer cost
// is negligible — GitHub responses we consume are kilobytes of JSON.
//
// Callers MUST NOT pass their own `signal` in `init` — the helper
// owns the signal slot. None of the call sites in this file do.
async function fetchWithTimeout(
  url: string,
  init: RequestInit = {},
  timeoutMs: number = GITHUB_FETCH_TIMEOUT_MS,
): Promise<Response> {
  // Make the "callers MUST NOT pass their own signal" rule above
  // mechanical: a future call site that supplies one would have its
  // cancellation silently swallowed by the spread below, which is
  // exactly the footgun Copilot flagged on PR #30 round 2. Refuse
  // up-front so the mistake surfaces at runtime instead of as a
  // silent stall. If a caller ever genuinely needs to compose a
  // user-cancellation signal with this 10 s budget, that's a
  // dedicated change to fetchWithTimeout (probably via
  // AbortSignal.any or an explicit caller-cancel parameter), not a
  // surprise in the existing helper's contract.
  if (init.signal !== undefined && init.signal !== null) {
    throw new TypeError(
      "fetchWithTimeout owns the AbortSignal slot; callers MUST NOT pass init.signal.",
    );
  }
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(url, { ...init, signal: controller.signal });
    const buf = await res.arrayBuffer();
    return new Response(buf, {
      status: res.status,
      statusText: res.statusText,
      headers: res.headers,
    });
  } catch (err) {
    if ((err as { name?: string } | undefined)?.name === "AbortError") {
      throw new GitHubTimeoutError(timeoutMs);
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

// ---------- JWT / installation token ----------

async function signAppJwt(env: Env): Promise<string> {
  // The App private key must be PKCS#8 (header: "BEGIN PRIVATE KEY").
  // Operators who downloaded a PKCS#1 key ("BEGIN RSA PRIVATE KEY") from the
  // App settings page must convert it once:
  //   openssl pkcs8 -topk8 -in raw.pem -nocrypt -out pkcs8.pem
  // Then upload pkcs8.pem via `wrangler secret put GITHUB_APP_PRIVATE_KEY`.
  const pem = env.GITHUB_APP_PRIVATE_KEY;
  if (!pem.includes("BEGIN PRIVATE KEY")) {
    throw new GitHubError(
      "GITHUB_APP_PRIVATE_KEY must be PKCS#8 (BEGIN PRIVATE KEY); convert with openssl pkcs8 -topk8.",
      500,
    );
  }
  const key = await importPKCS8(pem, "RS256");

  const now = Math.floor(Date.now() / 1000);
  return await new SignJWT({})
    .setProtectedHeader({ alg: "RS256", typ: "JWT" })
    .setIssuer(env.GITHUB_APP_ID)
    .setIssuedAt(now - 60)
    .setExpirationTime(now + 540)
    .sign(key);
}

export async function getInstallationToken(env: Env): Promise<string> {
  const jwt = await signAppJwt(env);
  const url = `${GITHUB_API}/app/installations/${env.GITHUB_INSTALLATION_ID}/access_tokens`;
  const res = await fetchWithTimeout(url, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${jwt}`,
      Accept: "application/vnd.github+json",
      "User-Agent": USER_AGENT,
      "X-GitHub-Api-Version": "2022-11-28",
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new GitHubError(
      `Installation token exchange failed: ${res.status} ${text.slice(0, 200)}`,
      res.status,
    );
  }
  const json = (await res.json()) as { token?: string };
  if (!json.token) {
    throw new GitHubError("Installation token response missing token field.", 502);
  }
  return json.token;
}

// ---------- GraphQL helpers ----------

interface GraphQLResponse<T> {
  data?: T;
  errors?: Array<{ message: string; type?: string }>;
}

async function graphql<T>(token: string, query: string, variables: Record<string, unknown>): Promise<T> {
  const res = await fetchWithTimeout(`${GITHUB_API}/graphql`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
      "User-Agent": USER_AGENT,
      "Content-Type": "application/json",
      "X-GitHub-Api-Version": "2022-11-28",
    },
    body: JSON.stringify({ query, variables }),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new GitHubError(`GitHub GraphQL HTTP ${res.status}: ${text.slice(0, 200)}`, res.status);
  }
  const json = (await res.json()) as GraphQLResponse<T>;
  if (json.errors && json.errors.length > 0) {
    throw new GitHubError(
      `GitHub GraphQL error: ${json.errors.map((e) => e.message).join("; ").slice(0, 300)}`,
      502,
    );
  }
  if (!json.data) {
    throw new GitHubError("GitHub GraphQL response missing data.", 502);
  }
  return json.data;
}

// ---------- Label resolution with KV cache ----------

interface RepoLabelsQuery {
  node: {
    labels: {
      nodes: Array<{ name: string; id: string }>;
      pageInfo: { hasNextPage: boolean; endCursor: string | null };
    };
  } | null;
}

async function fetchAllRepoLabels(
  token: string,
  repoId: string,
): Promise<Record<string, string>> {
  const labels: Record<string, string> = {};
  let cursor: string | null = null;

  const query = `
    query Labels($repoId: ID!, $cursor: String) {
      node(id: $repoId) {
        ... on Repository {
          labels(first: 100, after: $cursor) {
            nodes { name id }
            pageInfo { hasNextPage endCursor }
          }
        }
      }
    }
  `;

  // Hard cap to prevent infinite loops if the repo had >10k labels (defensive).
  for (let page = 0; page < 100; page++) {
    const data: RepoLabelsQuery = await graphql<RepoLabelsQuery>(token, query, {
      repoId,
      cursor,
    });
    const labelsPage = data.node?.labels;
    if (!labelsPage) break;
    for (const node of labelsPage.nodes) {
      labels[node.name] = node.id;
    }
    if (!labelsPage.pageInfo.hasNextPage) break;
    cursor = labelsPage.pageInfo.endCursor as string | null;
  }
  return labels;
}

export async function resolveLabelIds(
  env: Env,
  token: string,
  names: string[],
): Promise<string[]> {
  if (names.length === 0) return [];

  const cacheKey = `labels:${env.GITHUB_REPO_ID}`;
  let map: Record<string, string> | null = null;

  const cached = await env.FEEDBACK_IDEMP.get(cacheKey);
  if (cached) {
    try {
      const parsed = JSON.parse(cached) as unknown;
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        map = parsed as Record<string, string>;
      }
    } catch {
      // fall through to refetch
    }
  }

  if (!map) {
    map = await fetchAllRepoLabels(token, env.GITHUB_REPO_ID);
    await env.FEEDBACK_IDEMP.put(cacheKey, JSON.stringify(map), {
      expirationTtl: LABEL_CACHE_TTL_SECONDS,
    });
  }

  // Graceful: labels that don't exist in the repo are skipped silently,
  // the issue still posts with whatever labels did resolve.
  //
  // Case-insensitive match: GitHub labels are stored verbatim in
  // whatever case the maintainer created them (`area/ui`, `area/UI`,
  // `Area/UI` are all valid and distinct at the API level but the
  // UI treats them loosely). Clients that send a canonical
  // lowercase form shouldn't miss a label just because the repo
  // shipped with a mixed-case version, and vice-versa. Build a
  // lowercase index on the side so the lookup tolerates case drift
  // without requiring every caller to know the repo's exact
  // casing.
  const byLower: Record<string, string> = {};
  for (const k of Object.keys(map)) {
    // First occurrence wins; GitHub doesn't actually let you
    // create two labels whose lowercased forms collide, so there
    // should only ever be one.
    const lower = k.toLowerCase();
    if (byLower[lower] === undefined) {
      byLower[lower] = map[k] as string;
    }
  }
  const ids: string[] = [];
  for (const name of names) {
    const id = map[name] ?? byLower[name.toLowerCase()];
    if (id) ids.push(id);
  }
  return ids;
}

// ---------- createIssue mutation ----------

interface CreateIssueResponse {
  createIssue: {
    issue: { id: string; number: number; url: string };
  };
}

export async function createIssue(
  _env: Env,
  token: string,
  input: CreateIssueInput,
): Promise<CreatedIssue> {
  const mutation = `
    mutation CreateFeedbackIssue($input: CreateIssueInput!) {
      createIssue(input: $input) {
        issue { id number url }
      }
    }
  `;
  const data = await graphql<CreateIssueResponse>(token, mutation, {
    input: {
      repositoryId: input.repositoryId,
      title: input.title,
      body: input.body,
      labelIds: input.labelIds,
    },
  });
  return data.createIssue.issue;
}
