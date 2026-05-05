// Fixed-window per-IP rate limit backed by Cloudflare KV.
// Each request writes its own unique key under the per-IP/per-hour
// prefix so the count is the number of keys returned by KV.list().
// The prior read-modify-write counter pattern was racy under bursts:
// concurrent requests could read the same value and overwrite each
// other with the same next count, silently allowing more than LIMIT
// submissions per window. The unique-key scheme avoids overwrites, and
// a per-isolate prefix lock serializes list/put decisions inside one
// Worker isolate. KV is still eventually consistent across isolates;
// full cross-isolate linearizability would require Durable Objects.
//
// Key format: rl:<ip-hash>:<UTC-hour>:<random-suffix>
// Value:      "1" (the count is the number of matching keys).
// TTL:        3600s — KV expires rows on its own at hour rollover.
//
// Decision comes from specs/003-feedback-backend-worker/research.md §R3.

import type { Env } from "./env";

const LIMIT = 5;
const WINDOW_SECONDS = 3600;
const prefixLocks = new Map<string, Promise<void>>();

export interface RateLimitResult {
  allowed: boolean;
  count: number;
  retryAfterSeconds?: number;
}

export async function hashIp(ip: string, salt: string): Promise<string> {
  const data = new TextEncoder().encode(`${ip}:${salt}`);
  const digest = await crypto.subtle.digest("SHA-256", data);
  const bytes = new Uint8Array(digest);
  let hex = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    hex += bytes[i]!.toString(16).padStart(2, "0");
  }
  return hex;
}

function currentHourKey(now: Date): string {
  const y = now.getUTCFullYear();
  const m = String(now.getUTCMonth() + 1).padStart(2, "0");
  const d = String(now.getUTCDate()).padStart(2, "0");
  const h = String(now.getUTCHours()).padStart(2, "0");
  return `${y}-${m}-${d}T${h}`;
}

function secondsToNextHour(now: Date): number {
  const next = new Date(now);
  next.setUTCMinutes(0, 0, 0);
  next.setUTCHours(next.getUTCHours() + 1);
  return Math.max(1, Math.ceil((next.getTime() - now.getTime()) / 1000));
}

function randomSuffix(): string {
  const bytes = new Uint8Array(12);
  crypto.getRandomValues(bytes);
  let hex = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    hex += bytes[i]!.toString(16).padStart(2, "0");
  }
  return hex;
}

export interface RateLimitDeps {
  now?: () => Date;
  suffix?: () => string;
}

async function withPrefixLock<T>(prefix: string, fn: () => Promise<T>): Promise<T> {
  const previous = prefixLocks.get(prefix) ?? Promise.resolve();
  let release!: () => void;
  const current = new Promise<void>((resolve) => {
    release = resolve;
  });
  const pending = previous.then(() => current);
  prefixLocks.set(prefix, pending);
  await previous;
  try {
    return await fn();
  } finally {
    release();
    if (prefixLocks.get(prefix) === pending) {
      prefixLocks.delete(prefix);
    }
  }
}

export async function checkRateLimit(
  env: Env,
  ip: string,
  deps: RateLimitDeps = {},
): Promise<RateLimitResult> {
  const now = deps.now ? deps.now() : new Date();
  // Salt is already a secret (App ID is private enough for a salt — see prompt).
  const salt = env.GITHUB_APP_ID ?? "no-salt";
  const ipHash = await hashIp(ip, salt);
  const prefix = `rl:${ipHash}:${currentHourKey(now)}:`;

  return withPrefixLock(prefix, async () => {
    // Count existing keys under the bucket prefix. KV.list caps per-page
    // results at 1000; LIMIT is 5 so one page is always enough.
    const listed = await env.FEEDBACK_RATELIMIT.list({ prefix, limit: LIMIT + 1 });
    const existing = listed.keys.length;

    if (existing >= LIMIT) {
      return {
        allowed: false,
        count: existing,
        retryAfterSeconds: secondsToNextHour(now),
      };
    }

    // Write our own unique key; no overwrite of other concurrent
    // requests' keys, so the count is monotonic per bucket.
    const suffix = deps.suffix ? deps.suffix() : randomSuffix();
    await env.FEEDBACK_RATELIMIT.put(prefix + suffix, "1", {
      expirationTtl: WINDOW_SECONDS,
    });

    return { allowed: true, count: existing + 1 };
  });
}
