import { beforeEach, describe, expect, it } from "vitest";

import type { Env } from "../src/env";
import { checkRateLimit } from "../src/ratelimit";

// --- Minimal in-memory KV stub -----------------------------------------------
// Supports get / put / list. list matches the shape KV returns
// (list_complete + keys[].name) so checkRateLimit's list-based count
// works under tests the same way it does against real KV.
interface StoredValue { value: string; expiresAt?: number }

class FakeKV {
  store = new Map<string, StoredValue>();

  async get(key: string): Promise<string | null> {
    const entry = this.store.get(key);
    if (!entry) return null;
    if (entry.expiresAt !== undefined && entry.expiresAt <= Date.now()) {
      this.store.delete(key);
      return null;
    }
    return entry.value;
  }

  async put(
    key: string,
    value: string,
    options?: { expirationTtl?: number },
  ): Promise<void> {
    const expiresAt = options?.expirationTtl
      ? Date.now() + options.expirationTtl * 1000
      : undefined;
    this.store.set(key, { value, expiresAt });
  }

  async list(
    options: { prefix?: string; limit?: number } = {},
  ): Promise<{ keys: Array<{ name: string }>; list_complete: boolean }> {
    const prefix = options.prefix ?? "";
    const out: Array<{ name: string }> = [];
    for (const [k, v] of this.store) {
      if (v.expiresAt !== undefined && v.expiresAt <= Date.now()) continue;
      if (prefix && !k.startsWith(prefix)) continue;
      out.push({ name: k });
      if (options.limit && out.length >= options.limit) break;
    }
    return { keys: out, list_complete: true };
  }
}

function mkEnv(): { env: Env; kv: FakeKV } {
  const kv = new FakeKV();
  const env = {
    FEEDBACK_RATELIMIT: kv as unknown as KVNamespace,
    FEEDBACK_IDEMP: new FakeKV() as unknown as KVNamespace,
    FEEDBACK_ATTACHMENTS: {} as unknown as R2Bucket,
    GITHUB_APP_ID: "12345",
    GITHUB_INSTALLATION_ID: "67890",
    GITHUB_REPO_ID: "R_kg",
    GITHUB_APP_PRIVATE_KEY: "-----BEGIN PRIVATE KEY-----\n",
    R2_PUBLIC_URL_PREFIX: "https://assets.test",
  } satisfies Env;
  return { env, kv };
}

// Deterministic suffix generator so tests can predict per-request keys.
function suffixSeq(): () => string {
  let i = 0;
  return () => {
    i += 1;
    return `s${String(i).padStart(4, "0")}`;
  };
}

describe("checkRateLimit", () => {
  let env: Env;

  beforeEach(() => {
    ({ env } = mkEnv());
  });

  it("allows the first 5 requests, blocks the 6th", async () => {
    const now = () => new Date("2026-04-24T08:30:00Z");
    const suffix = suffixSeq();
    for (let i = 1; i <= 5; i++) {
      const res = await checkRateLimit(env, "1.2.3.4", { now, suffix });
      expect(res.allowed).toBe(true);
      expect(res.count).toBe(i);
    }
    const sixth = await checkRateLimit(env, "1.2.3.4", { now, suffix });
    expect(sixth.allowed).toBe(false);
    expect(sixth.count).toBe(5);
    expect(sixth.retryAfterSeconds).toBeGreaterThan(0);
    expect(sixth.retryAfterSeconds).toBeLessThanOrEqual(3600);
  });

  it("computes retryAfter as seconds until the next UTC hour", async () => {
    const now = () => new Date("2026-04-24T08:30:00Z"); // 30 min past hour
    const suffix = suffixSeq();
    for (let i = 0; i < 5; i++) await checkRateLimit(env, "1.2.3.4", { now, suffix });
    const res = await checkRateLimit(env, "1.2.3.4", { now, suffix });
    expect(res.allowed).toBe(false);
    // 30 minutes = 1800 seconds remaining to 09:00Z.
    expect(res.retryAfterSeconds).toBe(1800);
  });

  it("resets the window when the UTC hour rolls over", async () => {
    const firstHour = () => new Date("2026-04-24T08:59:00Z");
    const suffix = suffixSeq();
    for (let i = 0; i < 5; i++) await checkRateLimit(env, "1.2.3.4", { now: firstHour, suffix });
    const blocked = await checkRateLimit(env, "1.2.3.4", { now: firstHour, suffix });
    expect(blocked.allowed).toBe(false);

    const nextHour = () => new Date("2026-04-24T09:00:00Z");
    const freshAllowed = await checkRateLimit(env, "1.2.3.4", { now: nextHour, suffix });
    expect(freshAllowed.allowed).toBe(true);
    expect(freshAllowed.count).toBe(1);
  });

  it("keeps counters per distinct IP independently", async () => {
    const now = () => new Date("2026-04-24T08:00:00Z");
    const suffix = suffixSeq();
    for (let i = 0; i < 5; i++) await checkRateLimit(env, "1.2.3.4", { now, suffix });
    const otherIp = await checkRateLimit(env, "9.9.9.9", { now, suffix });
    expect(otherIp.allowed).toBe(true);
    expect(otherIp.count).toBe(1);
  });

  // Regression guard for the codex P1 on bf7e8fc/549b988: the prior
  // read-modify-write counter pattern could be defeated by
  // concurrent requests reading the same count and each writing back
  // +1, effectively losing N-1 increments and letting >LIMIT
  // submissions through. With unique-key-per-request PUTs, two
  // concurrent requests write distinct keys and the (LIMIT+1)th
  // request's list() strictly reflects both — no overwrite leak.
  it("does not leak extra submissions when two requests race", async () => {
    const now = () => new Date("2026-04-24T08:00:00Z");
    const suffix = suffixSeq();
    // Kick off 6 requests concurrently (no awaited sequencing).
    const results = await Promise.all(
      Array.from({ length: 6 }, () => checkRateLimit(env, "1.2.3.4", { now, suffix })),
    );
    const allowed = results.filter((r) => r.allowed).length;
    // At most LIMIT should succeed. Under the old pattern all 6
    // could end up with allowed=true because the count reads were
    // happening before any put landed.
    expect(allowed).toBeLessThanOrEqual(5);
  });
});
