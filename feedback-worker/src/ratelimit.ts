// Fixed-window per-IP rate limit backed by Cloudflare KV.
// Key format: rl:<ip-hash>:<UTC-hour>   Value: counter (1..5)   TTL: 3600s.
// Decision comes from specs/003-feedback-backend-worker/research.md §R3.

import type { Env } from "./env";

const LIMIT = 5;
const WINDOW_SECONDS = 3600;

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

export interface RateLimitDeps {
  now?: () => Date;
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
  const key = `rl:${ipHash}:${currentHourKey(now)}`;

  const existingRaw = await env.FEEDBACK_RATELIMIT.get(key);
  const existing = existingRaw ? parseInt(existingRaw, 10) : 0;
  const safeExisting = Number.isFinite(existing) && existing >= 0 ? existing : 0;

  if (safeExisting >= LIMIT) {
    return {
      allowed: false,
      count: safeExisting,
      retryAfterSeconds: secondsToNextHour(now),
    };
  }

  const nextCount = safeExisting + 1;
  // TTL is always window-sized; Cloudflare expires the key on its own.
  await env.FEEDBACK_RATELIMIT.put(key, String(nextCount), {
    expirationTtl: WINDOW_SECONDS,
  });

  return { allowed: true, count: nextCount };
}
