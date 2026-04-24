// GitHub App authentication + GraphQL client for creating issues.
// Research decision: hand-rolled client over octokit (see research.md §R1).

import { SignJWT, importPKCS8 } from "jose";

import type { Env } from "./env";

const GITHUB_API = "https://api.github.com";
const USER_AGENT = "my-gather-feedback-worker";
const LABEL_CACHE_TTL_SECONDS = 86_400;

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
  const res = await fetch(url, {
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
  const res = await fetch(`${GITHUB_API}/graphql`, {
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
