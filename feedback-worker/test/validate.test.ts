import { describe, expect, it } from "vitest";

import { validatePayload } from "../src/validate";

const UUID = "550e8400-e29b-41d4-a716-446655440000";

function base64Of(bytes: number[]): string {
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}

function bigBase64(size: number): string {
  // Generates a base64 string whose decoded size is exactly `size` bytes.
  const bytes = new Array<number>(size).fill(0x61);
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}

function goodPayload(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    title: "A sensible title",
    body: "Some body text",
    idempotencyKey: UUID,
    reportVersion: "v0.3.1-54-g29734aa",
    ...overrides,
  };
}

describe("validatePayload", () => {
  it("accepts a minimal valid payload", () => {
    const res = validatePayload(goodPayload());
    expect(res.ok).toBe(true);
    if (res.ok) {
      expect(res.data.title).toBe("A sensible title");
      expect(res.data.body).toBe("Some body text");
      expect(res.data.category).toBeUndefined();
      expect(res.data.image).toBeUndefined();
      expect(res.data.voice).toBeUndefined();
    }
  });

  it("rejects non-object payloads as malformed_payload", () => {
    const res = validatePayload("nope" as unknown);
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("malformed_payload");
  });

  it("rejects missing title as title_required", () => {
    const res = validatePayload(goodPayload({ title: undefined }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("title_required");
  });

  it("rejects whitespace-only title as title_required", () => {
    const res = validatePayload(goodPayload({ title: "   " }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("title_required");
  });

  it("rejects title longer than 200 chars as title_too_long", () => {
    const res = validatePayload(goodPayload({ title: "x".repeat(201) }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("title_too_long");
  });

  it("rejects body over 10240 UTF-8 bytes as body_too_long", () => {
    // "é" is 2 bytes UTF-8 → 5121 copies = 10242 bytes.
    const res = validatePayload(goodPayload({ body: "é".repeat(5121) }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("body_too_long");
  });

  it("accepts a body right at 10240 bytes", () => {
    const res = validatePayload(goodPayload({ body: "a".repeat(10240) }));
    expect(res.ok).toBe(true);
  });

  it("treats missing body as empty string", () => {
    const res = validatePayload(goodPayload({ body: undefined }));
    expect(res.ok).toBe(true);
    if (res.ok) expect(res.data.body).toBe("");
  });

  it("rejects non-string body as malformed_payload", () => {
    const res = validatePayload(goodPayload({ body: 42 }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("malformed_payload");
  });

  it("accepts a valid category", () => {
    const res = validatePayload(goodPayload({ category: "UI" }));
    expect(res.ok).toBe(true);
    if (res.ok) expect(res.data.category).toBe("UI");
  });

  it("rejects an unknown category as category_invalid", () => {
    const res = validatePayload(goodPayload({ category: "ui" }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("category_invalid");
  });

  it("rejects an image with disallowed mime as image_bad_mime", () => {
    const res = validatePayload(
      goodPayload({
        image: { mime: "image/tiff", base64: base64Of([1, 2, 3]) },
      }),
    );
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("image_bad_mime");
  });

  it("rejects an image over 5MB decoded as image_too_large", () => {
    const res = validatePayload(
      goodPayload({
        image: { mime: "image/png", base64: bigBase64(5_242_881) },
      }),
    );
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("image_too_large");
  });

  it("accepts an image at exactly 5MB", () => {
    const res = validatePayload(
      goodPayload({
        image: { mime: "image/png", base64: bigBase64(5_242_880) },
      }),
    );
    expect(res.ok).toBe(true);
    if (res.ok) expect(res.data.image?.bytes.byteLength).toBe(5_242_880);
  });

  it("rejects a voice with disallowed mime as voice_bad_mime", () => {
    const res = validatePayload(
      goodPayload({
        voice: { mime: "audio/flac", base64: base64Of([1]) },
      }),
    );
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("voice_bad_mime");
  });

  it("accepts audio/webm;codecs=opus as a valid voice mime", () => {
    const res = validatePayload(
      goodPayload({
        voice: { mime: "audio/webm;codecs=opus", base64: base64Of([1, 2]) },
      }),
    );
    expect(res.ok).toBe(true);
    if (res.ok) expect(res.data.voice?.mime).toBe("audio/webm;codecs=opus");
  });

  it("rejects a voice over 15MB decoded as voice_too_large", () => {
    const res = validatePayload(
      goodPayload({
        voice: { mime: "audio/mp4", base64: bigBase64(15_728_641) },
      }),
    );
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("voice_too_large");
  });

  it("rejects bad base64 as malformed_payload", () => {
    const res = validatePayload(
      goodPayload({
        image: { mime: "image/png", base64: "not base64 !!!" },
      }),
    );
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("malformed_payload");
  });

  it("rejects a missing idempotencyKey as idempotency_key_invalid", () => {
    const res = validatePayload(goodPayload({ idempotencyKey: undefined }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("idempotency_key_invalid");
  });

  it("rejects a malformed idempotencyKey as idempotency_key_invalid", () => {
    const res = validatePayload(goodPayload({ idempotencyKey: "not-a-uuid" }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("idempotency_key_invalid");
  });

  // Regression guard for the Copilot finding: the prior loose regex
  // `/^[0-9a-f-]{36}$/i` accepted 36 hyphens, 36 hex digits with no
  // hyphens at all, and non-v4 UUIDs. The tightened regex pins the
  // 8-4-4-4-12 grouping AND the RFC 4122 v4 version / variant
  // nibbles, matching the browser-side generateIdempotencyKey.
  it("rejects non-v4 UUID shapes even when they look plausible", () => {
    const bad = [
      "------------------------------------",                   // 36 hyphens
      "00000000000000000000000000000000",                       // 32 hex, no dashes
      "00000000-0000-0000-0000-000000000000",                   // RFC 4122 "nil" UUID (v0/variant 0)
      "550e8400-e29b-31d4-a716-446655440000",                   // v3, not v4
      "550e8400-e29b-51d4-a716-446655440000",                   // v5, not v4
      "550e8400-e29b-41d4-0716-446655440000",                   // bad variant nibble (0 instead of 8-b)
      "550e8400-e29b-41d4-c716-446655440000",                   // bad variant nibble (c)
      "550e8400e29b41d4a716446655440000",                       // 32 hex, no dashes
    ];
    for (const key of bad) {
      const res = validatePayload(goodPayload({ idempotencyKey: key }));
      expect(res.ok, `expected ${JSON.stringify(key)} to be rejected`).toBe(false);
      if (!res.ok) expect(res.error).toBe("idempotency_key_invalid");
    }
  });

  it("rejects an empty reportVersion as report_version_invalid", () => {
    const res = validatePayload(goodPayload({ reportVersion: "" }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("report_version_invalid");
  });

  it("rejects reportVersion longer than 64 chars as report_version_invalid", () => {
    const res = validatePayload(goodPayload({ reportVersion: "x".repeat(65) }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toBe("report_version_invalid");
  });
});
