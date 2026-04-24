// Idempotency cache backed by Cloudflare KV.
//
// Reservation pattern — rationale
// -------------------------------
// The issue Codex flagged: doing a KV lookup and only writing the
// final {issueUrl, issueNumber} AFTER the GitHub call means two
// concurrent requests with the same idempotencyKey can both observe
// the cache miss and both create GitHub issues.
//
// Mitigation without Durable Objects: write a `{status: "inflight"}`
// reservation BEFORE any side effects. A concurrent request that
// arrives after the reservation propagates sees it and returns a
// deterministic "duplicate in flight" signal instead of creating a
// second issue. The reservation TTL is short (30 s) so a crashed
// worker can't lock the key out forever — a retry after that window
// will re-reserve and proceed.
//
// This does NOT eliminate a perfectly-simultaneous race where both
// requests' KV.get see a miss before either KV.put lands; KV is
// eventually consistent and two PUTs on the same key simply have
// "last-write wins" semantics. Closing that last window would
// require a strongly-consistent store (Durable Objects). For the
// single-user double-submit/retry patterns this endpoint actually
// sees in the wild (hundreds of ms apart, not perfectly concurrent)
// the reservation is enough.
//
// Key format: idem:<idempotencyKey>
// Value:      JSON — either {status: "inflight"} during processing
//             or {status: "done", issueUrl, issueNumber} on success.
// Inflight TTL: 30s (generous upper bound on the GitHub round-trip).
// Done TTL:     300s (specs/003-feedback-backend-worker/research.md §R4).

import type { Env } from "./env";

export interface CachedResponse {
  issueUrl: string;
  issueNumber: number;
}

const INFLIGHT_TTL_SECONDS = 30;
const DONE_TTL_SECONDS = 300;

function keyFor(idempotencyKey: string): string {
  return `idem:${idempotencyKey}`;
}

// Outcome of reserveResponse: either the slot was free and we wrote
// a fresh reservation ("reserved"); or a prior successful response
// is already cached and we return it ("done"); or another request is
// still processing this same key and the caller should bail with a
// "duplicate in flight" signal ("inflight").
export type ReservationResult =
  | { kind: "reserved" }
  | { kind: "done"; response: CachedResponse }
  | { kind: "inflight" };

interface StoredInflight {
  status: "inflight";
}

interface StoredDone {
  status: "done";
  issueUrl: string;
  issueNumber: number;
}

type StoredState = StoredInflight | StoredDone;

function parseState(raw: string | null): StoredState | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") return null;
    const status = (parsed as { status?: unknown }).status;
    if (status === "inflight") {
      return { status: "inflight" };
    }
    if (status === "done") {
      const url = (parsed as { issueUrl?: unknown }).issueUrl;
      const num = (parsed as { issueNumber?: unknown }).issueNumber;
      if (typeof url === "string" && typeof num === "number") {
        return { status: "done", issueUrl: url, issueNumber: num };
      }
    }
  } catch {
    // fallthrough
  }
  return null;
}

// reserveResponse is the single entry point used by the happy-path
// handler. It returns "done" (cached response) / "inflight" (another
// request is processing this key) / "reserved" (this caller now
// owns the slot and must eventually call cacheResponse).
export async function reserveResponse(
  env: Env,
  idempotencyKey: string,
): Promise<ReservationResult> {
  const key = keyFor(idempotencyKey);
  const state = parseState(await env.FEEDBACK_IDEMP.get(key));
  if (state) {
    if (state.status === "done") {
      return {
        kind: "done",
        response: { issueUrl: state.issueUrl, issueNumber: state.issueNumber },
      };
    }
    return { kind: "inflight" };
  }
  const reservation: StoredInflight = { status: "inflight" };
  await env.FEEDBACK_IDEMP.put(key, JSON.stringify(reservation), {
    expirationTtl: INFLIGHT_TTL_SECONDS,
  });
  return { kind: "reserved" };
}

// cacheResponse records the final issue details once the GitHub call
// has succeeded. Overwrites any prior "inflight" reservation we
// planted in reserveResponse — "last-write wins" semantics are
// exactly what we want here.
export async function cacheResponse(
  env: Env,
  idempotencyKey: string,
  response: CachedResponse,
): Promise<void> {
  const done: StoredDone = {
    status: "done",
    issueUrl: response.issueUrl,
    issueNumber: response.issueNumber,
  };
  await env.FEEDBACK_IDEMP.put(keyFor(idempotencyKey), JSON.stringify(done), {
    expirationTtl: DONE_TTL_SECONDS,
  });
}

// releaseReservation clears an inflight reservation when the
// handler bails before producing a final cached response (e.g.
// GitHub returned a transient error the caller will retry). Safe to
// call unconditionally: if the key already holds a "done" state the
// delete undoes it, so callers MUST only invoke this on the error
// branch of their own reservation.
export async function releaseReservation(
  env: Env,
  idempotencyKey: string,
): Promise<void> {
  await env.FEEDBACK_IDEMP.delete(keyFor(idempotencyKey));
}
