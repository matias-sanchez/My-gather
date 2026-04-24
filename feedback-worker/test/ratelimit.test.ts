import { beforeEach, describe, expect, it } from "vitest";

import type { Env } from "../src/env";
import { checkRateLimit } from "../src/ratelimit";

// --- Minimal in-memory KV stub -----------------------------------------------
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

describe("checkRateLimit", () => {
  let env: Env;

  beforeEach(() => {
    ({ env } = mkEnv());
  });

  it("allows the first 5 requests, blocks the 6th", async () => {
    const now = () => new Date("2026-04-24T08:30:00Z");
    for (let i = 1; i <= 5; i++) {
      const res = await checkRateLimit(env, "1.2.3.4", { now });
      expect(res.allowed).toBe(true);
      expect(res.count).toBe(i);
    }
    const sixth = await checkRateLimit(env, "1.2.3.4", { now });
    expect(sixth.allowed).toBe(false);
    expect(sixth.count).toBe(5);
    expect(sixth.retryAfterSeconds).toBeGreaterThan(0);
    expect(sixth.retryAfterSeconds).toBeLessThanOrEqual(3600);
  });

  it("computes retryAfter as seconds until the next UTC hour", async () => {
    const now = () => new Date("2026-04-24T08:30:00Z"); // 30 min past hour
    for (let i = 0; i < 5; i++) await checkRateLimit(env, "1.2.3.4", { now });
    const res = await checkRateLimit(env, "1.2.3.4", { now });
    expect(res.allowed).toBe(false);
    // 30 minutes = 1800 seconds remaining to 09:00Z.
    expect(res.retryAfterSeconds).toBe(1800);
  });

  it("resets the window when the UTC hour rolls over", async () => {
    const firstHour = () => new Date("2026-04-24T08:59:00Z");
    for (let i = 0; i < 5; i++) await checkRateLimit(env, "1.2.3.4", { now: firstHour });
    const blocked = await checkRateLimit(env, "1.2.3.4", { now: firstHour });
    expect(blocked.allowed).toBe(false);

    const nextHour = () => new Date("2026-04-24T09:00:00Z");
    const freshAllowed = await checkRateLimit(env, "1.2.3.4", { now: nextHour });
    expect(freshAllowed.allowed).toBe(true);
    expect(freshAllowed.count).toBe(1);
  });

  it("keeps counters per distinct IP independently", async () => {
    const now = () => new Date("2026-04-24T08:00:00Z");
    for (let i = 0; i < 5; i++) await checkRateLimit(env, "1.2.3.4", { now });
    const otherIp = await checkRateLimit(env, "9.9.9.9", { now });
    expect(otherIp.allowed).toBe(true);
    expect(otherIp.count).toBe(1);
  });
});
