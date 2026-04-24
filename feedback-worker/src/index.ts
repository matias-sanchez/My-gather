// POST /feedback — validate → rate-limit → idempotency → R2 upload → GitHub issue.
// Full flow per Agent W task prompt (Fase 2). Supersedes the 501 skeleton.

import type { Env } from "./env";
import { buildIssueBody } from "./body";
import {
  createIssue,
  getInstallationToken,
  GitHubError,
  resolveLabelIds,
} from "./github-app";
import { cacheResponse, releaseReservation, reserveResponse } from "./idempotency";
import { logRequest } from "./log";
import { checkRateLimit, hashIp } from "./ratelimit";
import { uploadAttachment } from "./r2-upload";
import { validatePayload } from "./validate";

export type { Env } from "./env";

const WORKER_VERSION = "0.1.0";
const MAX_REQUEST_BYTES = 17_000_000; // see contracts/api.md §"Request size limit"

const CORS_HEADERS: Record<string, string> = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "POST, OPTIONS, GET",
  "Access-Control-Allow-Headers": "Content-Type",
  "Access-Control-Max-Age": "86400",
  Vary: "Origin",
};

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json; charset=utf-8");
  headers.set("Cache-Control", "no-store");
  for (const [k, v] of Object.entries(CORS_HEADERS)) headers.set(k, v);
  return new Response(JSON.stringify(body), { ...init, headers });
}

function errorResponse(
  status: number,
  error: string,
  message: string,
  extra: Record<string, unknown> = {},
  headers: Record<string, string> = {},
): Response {
  const init: ResponseInit = { status, headers };
  return jsonResponse({ ok: false, error, message, ...extra }, init);
}

function readClientIp(req: Request): string {
  return req.headers.get("CF-Connecting-IP") ?? req.headers.get("X-Forwarded-For") ?? "unknown";
}

function categoryLabel(category: string | undefined): string | null {
  // Labels in the repo live as lowercase (area/ui, area/parser, …). The
  // payload uses Title-case enum values (UI, Parser, …) for human
  // readability; normalise here so label resolution is case-consistent.
  if (!category) return null;
  return `area/${category.toLowerCase()}`;
}

async function handleFeedback(req: Request, env: Env, startedAt: number): Promise<Response> {
  const ip = readClientIp(req);
  // Pre-compute ip_hash for logging even when we short-circuit on size / JSON.
  const ipHash = await hashIp(ip, env.GITHUB_APP_ID ?? "no-salt").catch(() => "unknown");

  // --- Size gate --------------------------------------------------------------
  // Content-Length alone is not enough: chunked / non-browser clients
  // can omit the header entirely. Fast-reject on the header when
  // present (cheap pre-flight), then always verify the actual body
  // byte length before JSON decode so a missing/lying Content-Length
  // can't slip past the cap and exhaust isolate memory on this
  // public endpoint.
  const contentLength = req.headers.get("Content-Length");
  if (contentLength !== null) {
    const n = Number.parseInt(contentLength, 10);
    if (Number.isFinite(n) && n > MAX_REQUEST_BYTES) {
      logRequest({
        status: 413,
        error: "payload_too_large",
        duration_ms: Date.now() - startedAt,
        ip_hash: ipHash,
      });
      return errorResponse(413, "payload_too_large", "Request body exceeds 17 MB.");
    }
  }

  // --- Read body bytes (authoritative size check) -----------------------------
  let bodyBuf: ArrayBuffer;
  try {
    bodyBuf = await req.arrayBuffer();
  } catch {
    logRequest({
      status: 400,
      error: "malformed_payload",
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
    });
    return errorResponse(400, "malformed_payload", "Request body could not be read.");
  }
  if (bodyBuf.byteLength > MAX_REQUEST_BYTES) {
    logRequest({
      status: 413,
      error: "payload_too_large",
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
    });
    return errorResponse(413, "payload_too_large", "Request body exceeds 17 MB.");
  }

  // --- JSON parse --------------------------------------------------------------
  let raw: unknown;
  try {
    raw = JSON.parse(new TextDecoder().decode(bodyBuf));
  } catch {
    logRequest({
      status: 400,
      error: "malformed_payload",
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
    });
    return errorResponse(400, "malformed_payload", "Request body is not valid JSON.");
  }

  // --- Validate ---------------------------------------------------------------
  const validation = validatePayload(raw);
  if (!validation.ok) {
    logRequest({
      status: 400,
      error: validation.error,
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
    });
    return errorResponse(400, validation.error, validation.message);
  }
  const payload = validation.data;

  // --- Idempotency reservation (must run BEFORE the rate limit) ---------------
  // A retry of an already-successful submission MUST replay the
  // cached 200 regardless of whether the caller has since hit their
  // hourly quota, and an inflight duplicate MUST return 409 without
  // consuming rate-limit budget. So we check idempotency first and
  // only charge the rate-limit for genuinely new submissions. See
  // idempotency.ts for the KV-consistency trade-off on the
  // reservation write.
  const reservation = await reserveResponse(env, payload.idempotencyKey);
  if (reservation.kind === "done") {
    logRequest({
      status: 200,
      error: null,
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
      issue_number: reservation.response.issueNumber,
      report_version: payload.reportVersion,
      has_image: !!payload.image,
      has_voice: !!payload.voice,
    });
    return jsonResponse({
      ok: true,
      issueUrl: reservation.response.issueUrl,
      issueNumber: reservation.response.issueNumber,
    });
  }
  if (reservation.kind === "inflight") {
    logRequest({
      status: 409,
      error: "duplicate_inflight",
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
      report_version: payload.reportVersion,
      has_image: !!payload.image,
      has_voice: !!payload.voice,
    });
    return errorResponse(
      409,
      "duplicate_inflight",
      "A previous submission with the same idempotency key is still being processed. Retry in a few seconds.",
    );
  }

  // --- Rate limit -------------------------------------------------------------
  // Only genuinely new submissions (reservation.kind === "reserved")
  // reach this point, so an abusive burst is still bounded but
  // innocent retries of a successful key don't consume quota.
  const rl = await checkRateLimit(env, ip);
  if (!rl.allowed) {
    // Release the reservation we just planted so a later retry (after
    // the hour rolls over) can re-enter instead of getting stuck on
    // "duplicate_inflight" until the inflight TTL expires.
    await releaseReservation(env, payload.idempotencyKey).catch(() => undefined);
    const retryAfter = rl.retryAfterSeconds ?? 3600;
    logRequest({
      status: 429,
      error: "rate_limit",
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
      report_version: payload.reportVersion,
      rate_limit_count: rl.count,
      has_image: !!payload.image,
      has_voice: !!payload.voice,
    });
    const minutes = Math.max(1, Math.ceil(retryAfter / 60));
    return errorResponse(
      429,
      "rate_limit",
      `Rate limit reached. Try again in ${minutes} minutes.`,
      { retryAfterSeconds: retryAfter },
      { "Retry-After": String(retryAfter) },
    );
  }

  // --- Uploads + GitHub issue creation ----------------------------------------
  // Structure:
  //   1. pre-create phase (uploads + GraphQL call). Failures here MUST
  //      release the reservation so the client can retry and
  //      eventually create the issue.
  //   2. post-create phase (cacheResponse + log). The issue already
  //      exists on GitHub; a failure here MUST NOT release the
  //      reservation — if it did, the client's retry would create a
  //      duplicate ticket, exactly the failure window idempotency is
  //      meant to prevent. We return 200 with the real URL and
  //      swallow the cache error; the inflight marker stays until
  //      its TTL expires.
  let issue: { id: string; number: number; url: string };
  try {
    let imageUrl: string | undefined;
    let voiceUrl: string | undefined;
    if (payload.image) {
      const up = await uploadAttachment(env, {
        mime: payload.image.mime,
        bytes: payload.image.bytes,
      });
      imageUrl = up.publicUrl;
    }
    if (payload.voice) {
      const up = await uploadAttachment(env, {
        mime: payload.voice.mime,
        bytes: payload.voice.bytes,
      });
      voiceUrl = up.publicUrl;
    }

    const token = await getInstallationToken(env);

    const labelNames = ["user-feedback", "needs-triage"];
    const catLabel = categoryLabel(payload.category);
    if (catLabel) labelNames.push(catLabel);
    const labelIds = await resolveLabelIds(env, token, labelNames);

    const body = buildIssueBody({ payload, imageUrl, voiceUrl });
    issue = await createIssue(env, token, {
      repositoryId: env.GITHUB_REPO_ID,
      title: payload.title,
      body,
      labelIds,
    });
  } catch (err) {
    // Pre-create failure — nothing landed on GitHub yet, so it's
    // safe to release the reservation and let the client retry.
    await releaseReservation(env, payload.idempotencyKey).catch(() => undefined);
    if (err instanceof GitHubError) {
      logRequest({
        status: 503,
        error: "github_api_error",
        duration_ms: Date.now() - startedAt,
        ip_hash: ipHash,
        report_version: payload.reportVersion,
        rate_limit_count: rl.count,
        has_image: !!payload.image,
        has_voice: !!payload.voice,
      });
      return errorResponse(
        503,
        "github_api_error",
        "GitHub API unavailable. Please retry or use the fallback link.",
      );
    }
    logRequest({
      status: 500,
      error: "worker_error",
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
      report_version: payload.reportVersion,
      rate_limit_count: rl.count,
      has_image: !!payload.image,
      has_voice: !!payload.voice,
    });
    return errorResponse(500, "worker_error", "Internal error. Please try again later.");
  }

  // Issue now exists on GitHub. Best-effort cache of the response;
  // failures here must NOT release the reservation (see block comment
  // above) and must NOT turn a success into a 500 — the client has
  // an actionable issue URL either way.
  await cacheResponse(env, payload.idempotencyKey, {
    issueUrl: issue.url,
    issueNumber: issue.number,
  }).catch(() => undefined);

  logRequest({
    status: 200,
    error: null,
    duration_ms: Date.now() - startedAt,
    ip_hash: ipHash,
    issue_id: issue.id,
    issue_number: issue.number,
    report_version: payload.reportVersion,
    rate_limit_count: rl.count,
    has_image: !!payload.image,
    has_voice: !!payload.voice,
  });
  return jsonResponse({ ok: true, issueUrl: issue.url, issueNumber: issue.number });
}

export default {
  async fetch(req: Request, env: Env): Promise<Response> {
    const started = Date.now();
    const url = new URL(req.url);

    if (req.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: CORS_HEADERS });
    }

    if (req.method === "GET" && url.pathname === "/health") {
      return jsonResponse({ ok: true, version: WORKER_VERSION });
    }

    if (req.method === "POST" && url.pathname === "/feedback") {
      try {
        return await handleFeedback(req, env, started);
      } catch (err) {
        // Final catch-all so a surprise throw never crashes the isolate.
        // The client-facing message MUST stay generic — leaking raw
        // exception text exposes runtime internals and violates the
        // documented 500 contract in contracts/api.md.
        void err;
        logRequest({
          status: 500,
          error: "worker_error",
          duration_ms: Date.now() - started,
          ip_hash: "unknown",
        });
        return errorResponse(500, "worker_error", "Internal error. Please try again later.");
      }
    }

    return jsonResponse({ ok: false, error: "not_found", message: "Not found." }, { status: 404 });
  },
};
