// Idempotency cache backed by Cloudflare KV.
//
// Reservation pattern — rationale
// -------------------------------
// The original Codex concern was that a plain KV lookup followed by
// a post-create write let two concurrent same-key submissions both
// observe the cache miss and both create GitHub issues. Mitigation
// without Durable Objects: write a reservation BEFORE any side
// effect. A concurrent request that arrives after the reservation
// propagates sees it and returns a deterministic "duplicate in
// flight" signal instead of creating a second issue. The
// reservation TTL is short (60 s) so a crashed worker can't lock
// the key out forever — a retry after that window re-reserves and
// proceeds.
//
// Residual race (accepted, not closed)
// ------------------------------------
// reserveResponse is read-then-put — KV exposes no compare-and-put
// (CAS) primitive, so two concurrent same-key submissions arriving
// before either PUT completes can both observe a miss and both
// write "inflight", then both create issues. Closing this race
// requires Durable Objects (single-writer coordination). The
// project deliberately sticks to KV — see spec 003 §"Scale/Scope"
// and the constitution Principle X (minimal dependencies / minimal
// infra surface). The practical risk is narrow:
//
//   * The dialog mints a fresh `crypto.randomUUID()` per Submit
//     click (render/assets/app.js generateIdempotencyKey), so
//     cross-tab / double-click races use DIFFERENT keys and don't
//     collide here.
//   * Two HTTP clients POSTing the SAME idempotencyKey within the
//     ~100 ms KV propagation window are the only code path that
//     hits the race. In practice this means a deliberately-crafted
//     retry racing itself (e.g., a client-side retry loop with
//     `fetch` that doesn't `await` the first response).
//   * Even if both proceed, the rate-limit (5 per IP per hour)
//     bounds the blast radius to the same client.
//
// If the residual risk ever becomes material, the migration path is
// a Durable Object wrapping this file's three entry points; the
// calling code in index.ts would not change.
//
// Why there is no releaseReservation
// ----------------------------------
// A previous iteration of this file exposed a releaseReservation
// helper that pre-create failure paths called to let the client
// retry immediately. That helper was fundamentally racy: KV has no
// compare-and-delete primitive, so the unavoidable get → delete
// gap allowed a cross-request clobber — request A reads its own
// "inflight" marker, request B concurrently writes `{done, …}`,
// and A's subsequent delete wipes B's successful cached response,
// letting a later retry create a DUPLICATE issue. A per-call
// token narrowed the window but could not close it (last-write
// wins on KV put).
//
// Rather than ship a mitigation that still admits duplicates
// under realistic races, this file now lets the 60-second
// inflight TTL handle cleanup unconditionally. The UX cost is
// modest:
//
//   * A client retry within 60 s of a pre-create failure or
//     rate-limit rejection gets 409 duplicate_inflight instead of
//     a fresh attempt. After the TTL, a fresh attempt proceeds.
//   * A truly-stuck isolate (server-side crash mid-handler)
//     also self-heals after 60 s.
//
// The correctness win is that no code path can ever wipe a
// successful `{done, …}` record; the 5-minute idempotency replay
// contract is preserved on every race.
//
// Key format: idem:<idempotencyKey>
// Inflight value: {status: "inflight"}                     TTL: 60s
// Done value:     {status: "done", issueUrl, issueNumber}  TTL: 300s

import type { Env } from "./env";

export interface CachedResponse {
  issueUrl: string;
  issueNumber: number;
}

const INFLIGHT_TTL_SECONDS = 60;
const DONE_TTL_SECONDS = 300;

function keyFor(idempotencyKey: string): string {
  return `idem:${idempotencyKey}`;
}

// Outcome of reserveResponse: either a prior successful response is
// already cached (kind: "done"); or another request is still
// processing this key (kind: "inflight"); or the slot was free and
// we wrote a fresh reservation (kind: "reserved" — caller proceeds
// and eventually calls cacheResponse on success; nothing to do on
// failure because the 60 s TTL handles cleanup).
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
// handler. Returns "done" (cached response) / "inflight" (another
// request is processing this key) / "reserved" (this caller now
// owns the slot and must eventually call cacheResponse to upgrade
// the marker to "done").
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

// cacheResponse records the final issue details once the GitHub
// call has succeeded. Overwrites the prior "inflight" reservation
// we planted in reserveResponse — "last-write wins" semantics are
// exactly what we want here: even if a concurrent racer's failed
// path wrote nothing, this put still lands.
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
