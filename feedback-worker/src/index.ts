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
import { cacheResponse, getCachedResponse } from "./idempotency";
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
  if (!category) return null;
  return `area/${category}`;
}

async function handleFeedback(req: Request, env: Env, startedAt: number): Promise<Response> {
  const ip = readClientIp(req);
  // Pre-compute ip_hash for logging even when we short-circuit on size / JSON.
  const ipHash = await hashIp(ip, env.GITHUB_APP_ID ?? "no-salt").catch(() => "unknown");

  // --- Size gate (Content-Length, when present) -------------------------------
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

  // --- JSON parse --------------------------------------------------------------
  let raw: unknown;
  try {
    raw = await req.json();
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

  // --- Rate limit -------------------------------------------------------------
  const rl = await checkRateLimit(env, ip);
  if (!rl.allowed) {
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

  // --- Idempotency cache hit --------------------------------------------------
  const cached = await getCachedResponse(env, payload.idempotencyKey);
  if (cached) {
    logRequest({
      status: 200,
      error: null,
      duration_ms: Date.now() - startedAt,
      ip_hash: ipHash,
      issue_number: cached.issueNumber,
      report_version: payload.reportVersion,
      rate_limit_count: rl.count,
      has_image: !!payload.image,
      has_voice: !!payload.voice,
    });
    return jsonResponse({ ok: true, issueUrl: cached.issueUrl, issueNumber: cached.issueNumber });
  }

  // --- Uploads + GitHub issue creation ----------------------------------------
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
    const issue = await createIssue(env, token, {
      repositoryId: env.GITHUB_REPO_ID,
      title: payload.title,
      body,
      labelIds,
    });

    await cacheResponse(env, payload.idempotencyKey, {
      issueUrl: issue.url,
      issueNumber: issue.number,
    });

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
  } catch (err) {
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
        const message = err instanceof Error ? err.message : "unknown";
        logRequest({
          status: 500,
          error: "worker_error",
          duration_ms: Date.now() - started,
          ip_hash: "unknown",
        });
        return errorResponse(500, "worker_error", `Internal error: ${message.slice(0, 120)}`);
      }
    }

    return jsonResponse({ ok: false, error: "not_found", message: "Not found." }, { status: 404 });
  },
};
