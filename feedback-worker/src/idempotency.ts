// Idempotency cache backed by Cloudflare KV.
//
// Reservation pattern — rationale
// -------------------------------
// The issue Codex originally flagged: doing a KV lookup and only
// writing the final {issueUrl, issueNumber} AFTER the GitHub call
// means two concurrent requests with the same idempotencyKey can
// both observe the cache miss and both create GitHub issues.
//
// Mitigation without Durable Objects: write a reservation BEFORE
// any side effect. A concurrent request that arrives after the
// reservation propagates sees it and returns a deterministic
// "duplicate in flight" signal instead of creating a second issue.
// The reservation TTL is short (30 s) so a crashed worker can't
// lock the key out forever — a retry after that window will
// re-reserve and proceed.
//
// Per-reservation tokens
// ----------------------
// Under KV's eventual consistency, a truly-simultaneous race can
// still leave TWO requests each thinking they reserved the slot.
// releaseReservation was originally written as an unconditional
// delete, which created a second failure mode: if one of the two
// racers completes the full happy path and writes
// `{status: "done"}` while the other later fails pre-create,
// calling delete would WIPE the successful cached response — and
// the user's retry would create a DUPLICATE issue instead of
// replaying the original success.
//
// Each reservation now carries a unique token. releaseReservation
// takes that token and only deletes when the current stored state
// is still the caller's own "inflight" reservation; a "done"
// state, or an "inflight" with someone else's token, is left
// alone. This doesn't need to be linearizable — it just has to
// prevent the cross-request clobber that Codex flagged.
//
// Key format: idem:<idempotencyKey>
// Inflight value: {status: "inflight", token: "<hex>"}    TTL: 30s
// Done value:     {status: "done", issueUrl, issueNumber} TTL: 300s

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
// a fresh reservation (kind: "reserved" with the token the caller
// needs to pass back to releaseReservation); or a prior successful
// response is already cached (kind: "done"); or another request is
// still processing this same key (kind: "inflight").
export type ReservationResult =
  | { kind: "reserved"; token: string }
  | { kind: "done"; response: CachedResponse }
  | { kind: "inflight" };

interface StoredInflight {
  status: "inflight";
  token: string;
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
      const token = (parsed as { token?: unknown }).token;
      return { status: "inflight", token: typeof token === "string" ? token : "" };
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

function newReservationToken(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  let hex = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    hex += bytes[i]!.toString(16).padStart(2, "0");
  }
  return hex;
}

// reserveResponse is the single entry point used by the happy-path
// handler. Returns "done" (cached response) / "inflight" (another
// request is processing this key) / "reserved" (this caller now
// owns the slot and must eventually call cacheResponse, or
// releaseReservation with the returned token on pre-create
// failure).
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
  const token = newReservationToken();
  const reservation: StoredInflight = { status: "inflight", token };
  await env.FEEDBACK_IDEMP.put(key, JSON.stringify(reservation), {
    expirationTtl: INFLIGHT_TTL_SECONDS,
  });
  return { kind: "reserved", token };
}

// cacheResponse records the final issue details once the GitHub
// call has succeeded. Overwrites any prior "inflight" reservation
// we planted in reserveResponse — "last-write wins" semantics are
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
// GitHub returned a transient error the caller will retry). Only
// deletes when the current stored state is still an "inflight"
// record whose token matches the one returned by reserveResponse;
// this prevents a failed-racer's release from wiping a different
// request's successful "done" state (see the per-reservation-tokens
// block comment above).
export async function releaseReservation(
  env: Env,
  idempotencyKey: string,
  token: string,
): Promise<void> {
  const key = keyFor(idempotencyKey);
  const state = parseState(await env.FEEDBACK_IDEMP.get(key));
  if (!state) return; // already expired or deleted by someone else
  if (state.status === "done") return; // a concurrent racer succeeded; keep it
  if (state.status === "inflight" && state.token !== token) return; // not ours
  await env.FEEDBACK_IDEMP.delete(key);
}
