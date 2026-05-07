import { describe, expect, it } from "vitest";

import type { Env } from "../src/env";
import { feedbackContract } from "../src/feedback-contract";
import worker from "../src/index";

class FakeKV {
  store = new Map<string, string>();

  async get(key: string): Promise<string | null> {
    return this.store.get(key) ?? null;
  }

  async put(key: string, value: string, options?: { expirationTtl?: number }): Promise<void> {
    if (options?.expirationTtl !== undefined && options.expirationTtl < 60) {
      throw new Error(`KV PUT failed: Invalid expiration_ttl of ${options.expirationTtl}`);
    }
    this.store.set(key, value);
  }

  async list(options?: { prefix?: string; limit?: number }): Promise<{ keys: Array<{ name: string }> }> {
    const prefix = options?.prefix ?? "";
    const limit = options?.limit ?? Number.POSITIVE_INFINITY;
    const keys = Array.from(this.store.keys())
      .filter((name) => name.startsWith(prefix))
      .slice(0, limit)
      .map((name) => ({ name }));
    return { keys };
  }
}

class FakeR2 {
  objects = new Map<string, Uint8Array>();
  deleted: string[] = [];
  putKeys: string[] = [];

  async head(key: string): Promise<unknown | null> {
    return this.objects.has(key) ? { key } : null;
  }

  async put(key: string, value: Uint8Array): Promise<void> {
    this.objects.set(key, value);
    this.putKeys.push(key);
  }

  async delete(key: string): Promise<void> {
    this.objects.delete(key);
    this.deleted.push(key);
  }
}

function base64(bytes: number[]): string {
  return Buffer.from(Uint8Array.from(bytes)).toString("base64");
}

async function sha256Hex(bytes: Uint8Array): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Buffer.from(new Uint8Array(digest)).toString("hex");
}

async function imageKey(bytes: Uint8Array): Promise<string> {
  return `attachments/${await sha256Hex(bytes)}.png`;
}

async function voiceKey(bytes: Uint8Array): Promise<string> {
  return `attachments/${await sha256Hex(bytes)}.webm`;
}

function mkEnv(r2: FakeR2): Env {
  return {
    FEEDBACK_RATELIMIT: new FakeKV() as unknown as KVNamespace,
    FEEDBACK_IDEMP: new FakeKV() as unknown as KVNamespace,
    FEEDBACK_ATTACHMENTS: r2 as unknown as R2Bucket,
    GITHUB_APP_ID: "424242",
    GITHUB_INSTALLATION_ID: "9999",
    GITHUB_REPO_ID: "R_kgDOTestRepo",
    GITHUB_APP_PRIVATE_KEY: "-----BEGIN RSA PRIVATE KEY-----\ninvalid\n-----END RSA PRIVATE KEY-----\n",
    R2_PUBLIC_URL_PREFIX: "https://assets.test",
  };
}

function feedbackRequest(
  imageBytes: number[],
  idempotencyKey: string,
  voiceBytes?: number[],
): Request {
  return new Request("https://worker.test/feedback", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "CF-Connecting-IP": "203.0.113.9",
    },
    body: JSON.stringify({
      title: "Parser feedback",
      author: "Test Author",
      body: "Please inspect this sample.",
      image: {
        mime: "image/png",
        base64: base64(imageBytes),
      },
      voice: voiceBytes
        ? {
            mime: "audio/webm",
            base64: base64(voiceBytes),
          }
        : undefined,
      idempotencyKey,
      reportVersion: "v0.3.1-test",
    }),
  });
}

function minimalFeedbackRequest(idempotencyKey: string): Request {
  return new Request("https://worker.test/feedback", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "CF-Connecting-IP": "203.0.113.9",
    },
    body: JSON.stringify({
      title: "Parser feedback",
      author: "Test Author",
      body: "Please inspect this sample.",
      idempotencyKey,
      reportVersion: "v0.3.1-test",
    }),
  });
}

describe("feedback Worker route handling", () => {
  it("returns health metadata on /health", async () => {
    const res = await worker.fetch(new Request("https://worker.test/health"), mkEnv(new FakeR2()));

    expect(res.status).toBe(200);
    expect(await res.json()).toMatchObject({ ok: true, version: "0.1.0" });
    expect(res.headers.get("Cache-Control")).toBe("no-store");
  });

  it("returns JSON 404 for unknown routes", async () => {
    const res = await worker.fetch(new Request("https://worker.test/nope"), mkEnv(new FakeR2()));

    expect(res.status).toBe(404);
    expect(await res.json()).toMatchObject({ ok: false, error: "not_found" });
  });

  it("rejects malformed JSON through the /feedback fetch route", async () => {
    const res = await worker.fetch(
      new Request("https://worker.test/feedback", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{",
      }),
      mkEnv(new FakeR2()),
    );

    expect(res.status).toBe(400);
    expect(await res.json()).toMatchObject({ ok: false, error: "malformed_payload" });
  });

  it("rejects oversized requests before parsing JSON", async () => {
    const res = await worker.fetch(
      new Request("https://worker.test/feedback", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": String(feedbackContract.limits.requestMaxBytes + 1),
        },
        body: "{}",
      }),
      mkEnv(new FakeR2()),
    );

    expect(res.status).toBe(413);
    expect(await res.json()).toMatchObject({ ok: false, error: "payload_too_large" });
  });

  it("replays completed idempotency responses at the route level", async () => {
    const env = mkEnv(new FakeR2());
    await env.FEEDBACK_IDEMP.put(
      "idem:33333333-3333-4333-8333-333333333333",
      JSON.stringify({
        status: "done",
        issueUrl: "https://github.com/matias-sanchez/My-gather/issues/333",
        issueNumber: 333,
      }),
    );

    const res = await worker.fetch(
      minimalFeedbackRequest("33333333-3333-4333-8333-333333333333"),
      env,
    );

    expect(res.status).toBe(200);
    expect(await res.json()).toMatchObject({
      ok: true,
      issueUrl: "https://github.com/matias-sanchez/My-gather/issues/333",
      issueNumber: 333,
    });
  });

  it("returns duplicate_inflight for reserved idempotency keys at the route level", async () => {
    const env = mkEnv(new FakeR2());
    await env.FEEDBACK_IDEMP.put(
      "idem:44444444-4444-4444-8444-444444444444",
      JSON.stringify({ status: "inflight" }),
    );

    const res = await worker.fetch(
      minimalFeedbackRequest("44444444-4444-4444-8444-444444444444"),
      env,
    );

    expect(res.status).toBe(409);
    expect(await res.json()).toMatchObject({ ok: false, error: "duplicate_inflight" });
  });
});

describe("feedback Worker shared response headers", () => {
  it("returns no-store on CORS preflight responses", async () => {
    const res = await worker.fetch(
      new Request("https://worker.test/feedback", { method: "OPTIONS" }),
      mkEnv(new FakeR2()),
    );

    expect(res.status).toBe(204);
    expect(res.headers.get("Cache-Control")).toBe("no-store");
    expect(res.headers.get("Access-Control-Max-Age")).toBe("0");
    expect(res.headers.get("Access-Control-Allow-Origin")).toBe("*");
    expect(res.headers.get("Vary")).toBe("Origin");
  });
});

describe("feedback Worker attachment cleanup", () => {
  it("deletes newly-created R2 attachments when GitHub pre-create fails", async () => {
    const r2 = new FakeR2();
    const imageBytes = [0x89, 0x50, 0x4e, 0x47, 0x01];
    const voiceBytes = [0x1a, 0x45, 0xdf, 0xa3, 0x01];
    const image = await imageKey(Uint8Array.from(imageBytes));
    const voice = await voiceKey(Uint8Array.from(voiceBytes));
    const res = await worker.fetch(
      feedbackRequest(imageBytes, "11111111-1111-4111-8111-111111111111", voiceBytes),
      mkEnv(r2),
    );

    expect(res.status).toBe(503);
    expect(await res.json()).toMatchObject({ ok: false, error: "github_api_error" });
    expect(r2.putKeys).toEqual([image, voice]);
    expect(r2.deleted).toEqual([image, voice]);
    expect(r2.objects.has(image)).toBe(false);
    expect(r2.objects.has(voice)).toBe(false);
  });

  it("does not delete a pre-existing content-hash R2 attachment on GitHub pre-create failure", async () => {
    const r2 = new FakeR2();
    const bytes = [0x89, 0x50, 0x4e, 0x47, 0x02];
    const key = await imageKey(Uint8Array.from(bytes));
    r2.objects.set(key, Uint8Array.from(bytes));

    const res = await worker.fetch(
      feedbackRequest(bytes, "22222222-2222-4222-8222-222222222222"),
      mkEnv(r2),
    );

    expect(res.status).toBe(503);
    expect(await res.json()).toMatchObject({ ok: false, error: "github_api_error" });
    expect(r2.putKeys).toEqual([]);
    expect(r2.deleted).toEqual([]);
    expect(r2.objects.has(key)).toBe(true);
  });
});
