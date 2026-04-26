import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { decodeJwt, decodeProtectedHeader } from "jose";

import type { Env } from "../src/env";
import {
  createIssue,
  getInstallationToken,
  GitHubTimeoutError,
  resolveLabelIds,
} from "../src/github-app";

// ---------- in-memory KV mock for the label cache ---------------------------
class FakeKV {
  store = new Map<string, string>();
  async get(key: string): Promise<string | null> {
    return this.store.get(key) ?? null;
  }
  async put(key: string, value: string): Promise<void> {
    this.store.set(key, value);
  }
}

// ---------- one-shot PKCS#8 test private key (PEM) --------------------------
async function makePkcs8Pem(): Promise<string> {
  const pair = (await crypto.subtle.generateKey(
    {
      name: "RSASSA-PKCS1-v1_5",
      modulusLength: 2048,
      publicExponent: new Uint8Array([0x01, 0x00, 0x01]),
      hash: "SHA-256",
    },
    true,
    ["sign", "verify"],
  )) as CryptoKeyPair;
  const pkcs8 = (await crypto.subtle.exportKey("pkcs8", pair.privateKey)) as ArrayBuffer;
  const b64 = Buffer.from(new Uint8Array(pkcs8)).toString("base64");
  const lines = b64.match(/.{1,64}/g)!.join("\n");
  return `-----BEGIN PRIVATE KEY-----\n${lines}\n-----END PRIVATE KEY-----\n`;
}

function mkEnv(pem: string): Env {
  return {
    FEEDBACK_RATELIMIT: new FakeKV() as unknown as KVNamespace,
    FEEDBACK_IDEMP: new FakeKV() as unknown as KVNamespace,
    FEEDBACK_ATTACHMENTS: {} as unknown as R2Bucket,
    GITHUB_APP_ID: "424242",
    GITHUB_INSTALLATION_ID: "9999",
    GITHUB_REPO_ID: "R_kgDOTestRepo",
    GITHUB_APP_PRIVATE_KEY: pem,
    R2_PUBLIC_URL_PREFIX: "https://assets.test",
  };
}

// ---------- fetch mock helpers ----------
interface Recorded { url: string; init: RequestInit }

function installFetchMock(responder: (rec: Recorded) => Response | Promise<Response>) {
  const calls: Recorded[] = [];
  const mock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const rec: Recorded = { url, init: init ?? {} };
    calls.push(rec);
    return await responder(rec);
  });
  vi.stubGlobal("fetch", mock);
  return { mock, calls };
}

function jsonResp(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// ---------- tests ----------

describe("getInstallationToken", () => {
  let pem: string;
  beforeEach(async () => {
    pem = await makePkcs8Pem();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("signs an RS256 JWT with correct claims and hits the installation endpoint", async () => {
    const env = mkEnv(pem);
    const { calls } = installFetchMock(() => jsonResp({ token: "ghs_installtoken" }));

    const token = await getInstallationToken(env);
    expect(token).toBe("ghs_installtoken");

    expect(calls.length).toBe(1);
    const call = calls[0]!;
    expect(call.url).toBe(
      `https://api.github.com/app/installations/${env.GITHUB_INSTALLATION_ID}/access_tokens`,
    );
    expect(call.init.method).toBe("POST");
    const headers = call.init.headers as Record<string, string>;
    expect(headers["Accept"]).toBe("application/vnd.github+json");
    expect(headers["User-Agent"]).toBeDefined();

    const auth = headers["Authorization"]!;
    expect(auth.startsWith("Bearer ")).toBe(true);
    const jwt = auth.slice("Bearer ".length);

    const header = decodeProtectedHeader(jwt);
    expect(header.alg).toBe("RS256");
    expect(header.typ).toBe("JWT");

    const claims = decodeJwt(jwt);
    expect(claims.iss).toBe(env.GITHUB_APP_ID);
    const now = Math.floor(Date.now() / 1000);
    // iat is backdated by 60s, exp is ~540s ahead (with a sign-time jitter).
    expect(claims.iat!).toBeLessThanOrEqual(now - 59);
    expect(claims.iat!).toBeGreaterThan(now - 120);
    expect(claims.exp! - claims.iat!).toBe(600);
  });

  it("rejects a non-PKCS#8 private key with a clear error", async () => {
    const env = mkEnv("-----BEGIN RSA PRIVATE KEY-----\nXXX\n-----END RSA PRIVATE KEY-----\n");
    installFetchMock(() => jsonResp({ token: "nope" }));
    await expect(getInstallationToken(env)).rejects.toThrow(/PKCS#8/);
  });

  it("surfaces non-2xx responses as GitHubError", async () => {
    const env = mkEnv(pem);
    installFetchMock(() => new Response("boom", { status: 500 }));
    await expect(getInstallationToken(env)).rejects.toThrow(/Installation token exchange failed/);
  });

  it("passes an AbortController signal to fetch (FR-014 timeout wired up)", async () => {
    const env = mkEnv(pem);
    const { calls } = installFetchMock(() => jsonResp({ token: "ghs_x" }));
    await getInstallationToken(env);
    // The 10s AbortController owned by fetchWithTimeout MUST be passed
    // through to fetch; without it the timeout has no effect on the
    // actual network call.
    expect(calls[0]!.init.signal).toBeDefined();
  });

  it("converts an AbortError DOMException into a GitHubTimeoutError (504 path)", async () => {
    const env = mkEnv(pem);
    installFetchMock(() => {
      // Simulate the runtime aborting the fetch when the 10s timer
      // fires. The underlying fetch rejects with a DOMException
      // whose .name === "AbortError" — fetchWithTimeout MUST translate
      // that into the typed GitHubTimeoutError so index.ts can branch
      // to a 504 response (per FR-014 + contracts/api.md §504).
      throw new DOMException("The operation was aborted.", "AbortError");
    });
    await expect(getInstallationToken(env)).rejects.toBeInstanceOf(GitHubTimeoutError);
    await expect(getInstallationToken(env)).rejects.toThrow(/exceeded.*ms/);
  });
});

describe("resolveLabelIds", () => {
  let pem: string;
  beforeEach(async () => {
    pem = await makePkcs8Pem();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("queries labels via GraphQL and caches the result", async () => {
    const env = mkEnv(pem);
    const { calls } = installFetchMock(() =>
      jsonResp({
        data: {
          node: {
            labels: {
              nodes: [
                { name: "user-feedback", id: "L_uf" },
                { name: "needs-triage", id: "L_nt" },
                { name: "area/UI", id: "L_ui" },
              ],
              pageInfo: { hasNextPage: false, endCursor: null },
            },
          },
        },
      }),
    );

    const ids = await resolveLabelIds(env, "tok", [
      "user-feedback",
      "needs-triage",
      "area/UI",
      "does-not-exist",
    ]);
    // Missing labels are skipped silently.
    expect(ids).toEqual(["L_uf", "L_nt", "L_ui"]);
    expect(calls.length).toBe(1);

    // Second call re-uses KV cache; no additional fetch.
    const ids2 = await resolveLabelIds(env, "tok", ["user-feedback"]);
    expect(ids2).toEqual(["L_uf"]);
    expect(calls.length).toBe(1);
  });

  it("returns [] for an empty name list without hitting the network", async () => {
    const env = mkEnv(pem);
    const { calls } = installFetchMock(() => jsonResp({}));
    const ids = await resolveLabelIds(env, "tok", []);
    expect(ids).toEqual([]);
    expect(calls.length).toBe(0);
  });

  it("resolves labels case-insensitively when repo casing differs from caller's", async () => {
    // Regression guard for the case-mismatch codex flagged: client
    // code lowercases "UI" → "area/ui", but the repo ships
    // "area/UI". A strict-equality lookup missed the label and
    // created issues without their intended triage tag. The
    // resolver now matches case-insensitively against the
    // GitHub-returned names, so either direction works.
    const env = mkEnv(pem);
    installFetchMock(() =>
      jsonResp({
        data: {
          node: {
            labels: {
              nodes: [
                { name: "area/UI", id: "L_ui" },    // repo stores mixed case
                { name: "area/parser", id: "L_pa" }, // repo stores lowercase
              ],
              pageInfo: { hasNextPage: false, endCursor: null },
            },
          },
        },
      }),
    );

    // Caller passes lowercased form; repo has "area/UI".
    const lc = await resolveLabelIds(env, "tok", ["area/ui"]);
    expect(lc).toEqual(["L_ui"]);

    // Caller passes mixed case; repo has "area/parser" (lowercase).
    const uc = await resolveLabelIds(env, "tok", ["area/PARSER"]);
    expect(uc).toEqual(["L_pa"]);
  });
});

describe("createIssue", () => {
  let pem: string;
  beforeEach(async () => {
    pem = await makePkcs8Pem();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("posts the createIssue mutation with the expected variables", async () => {
    const env = mkEnv(pem);
    const { calls } = installFetchMock(() =>
      jsonResp({
        data: {
          createIssue: {
            issue: {
              id: "I_abc",
              number: 42,
              url: "https://github.com/matias-sanchez/My-gather/issues/42",
            },
          },
        },
      }),
    );

    const created = await createIssue(env, "tok", {
      repositoryId: env.GITHUB_REPO_ID,
      title: "Title",
      body: "Body",
      labelIds: ["L_uf", "L_nt"],
    });

    expect(created).toEqual({
      id: "I_abc",
      number: 42,
      url: "https://github.com/matias-sanchez/My-gather/issues/42",
    });

    expect(calls.length).toBe(1);
    const call = calls[0]!;
    expect(call.url).toBe("https://api.github.com/graphql");
    const headers = call.init.headers as Record<string, string>;
    expect(headers["Authorization"]).toBe("Bearer tok");
    const body = JSON.parse(String(call.init.body)) as {
      query: string;
      variables: { input: Record<string, unknown> };
    };
    expect(body.query).toContain("createIssue");
    expect(body.variables.input).toEqual({
      repositoryId: env.GITHUB_REPO_ID,
      title: "Title",
      body: "Body",
      labelIds: ["L_uf", "L_nt"],
    });
  });

  it("throws GitHubError on GraphQL errors", async () => {
    const env = mkEnv(pem);
    installFetchMock(() => jsonResp({ errors: [{ message: "nope" }] }));
    await expect(
      createIssue(env, "tok", {
        repositoryId: env.GITHUB_REPO_ID,
        title: "x",
        body: "y",
        labelIds: [],
      }),
    ).rejects.toThrow(/GitHub GraphQL error/);
  });
});
