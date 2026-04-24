// Idempotency cache backed by Cloudflare KV.
// Key format: idem:<idempotencyKey>
// Value: JSON {issueUrl, issueNumber}
// TTL: 300 seconds (see specs/003-feedback-backend-worker/research.md §R4).

import type { Env } from "./env";

export interface CachedResponse {
  issueUrl: string;
  issueNumber: number;
}

const TTL_SECONDS = 300;

function keyFor(idempotencyKey: string): string {
  return `idem:${idempotencyKey}`;
}

export async function getCachedResponse(
  env: Env,
  idempotencyKey: string,
): Promise<CachedResponse | null> {
  const raw = await env.FEEDBACK_IDEMP.get(keyFor(idempotencyKey));
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (
      parsed &&
      typeof parsed === "object" &&
      typeof (parsed as CachedResponse).issueUrl === "string" &&
      typeof (parsed as CachedResponse).issueNumber === "number"
    ) {
      return parsed as CachedResponse;
    }
    return null;
  } catch {
    return null;
  }
}

export async function cacheResponse(
  env: Env,
  idempotencyKey: string,
  response: CachedResponse,
): Promise<void> {
  await env.FEEDBACK_IDEMP.put(keyFor(idempotencyKey), JSON.stringify(response), {
    expirationTtl: TTL_SECONDS,
  });
}
