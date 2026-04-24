// Payload validation for POST /feedback.
// Every constraint comes from specs/003-feedback-backend-worker/contracts/api.md.

export type Category = "UI" | "Parser" | "Advisor" | "Other";

export interface ValidatedImage {
  mime: string;
  base64: string;
  bytes: Uint8Array;
}

export interface ValidatedVoice {
  mime: string;
  base64: string;
  bytes: Uint8Array;
}

export interface ValidatedPayload {
  title: string;
  body: string;
  category?: Category;
  image?: ValidatedImage;
  voice?: ValidatedVoice;
  idempotencyKey: string;
  reportVersion: string;
}

export type ValidationError =
  | "malformed_payload"
  | "title_required"
  | "title_too_long"
  | "body_too_long"
  | "category_invalid"
  | "image_bad_mime"
  | "image_too_large"
  | "voice_bad_mime"
  | "voice_too_large"
  | "idempotency_key_invalid"
  | "report_version_invalid";

export type ValidationResult =
  | { ok: true; data: ValidatedPayload }
  | { ok: false; error: ValidationError; message: string };

const TITLE_MAX = 200;
const BODY_MAX_BYTES = 10_240;
const IMAGE_MAX_BYTES = 5_242_880; // 5 MB
const VOICE_MAX_BYTES = 15_728_640; // 15 MB — 10 min Opus at the browser's default bitrate fits with margin
const REPORT_VERSION_MAX = 64;

const CATEGORIES: readonly Category[] = ["UI", "Parser", "Advisor", "Other"];
const IMAGE_MIME_RE = /^image\/(png|jpeg|gif|webp)$/;
const VOICE_MIME_RE = /^audio\/(webm|mp4|ogg|mpeg)(;.*)?$/;
// RFC 4122 v4: 8-4-4-4-12 hex groups with the version nibble fixed
// to 4 and the variant nibble in {8, 9, a, b}. The prior
// /^[0-9a-f-]{36}$/i accepted strings like 36 hyphens or 36 hex
// digits with no dashes, which let a caller inject malformed
// values into KV keys without being caught here.
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function fail(error: ValidationError, message: string): ValidationResult {
  return { ok: false, error, message };
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function utf8ByteLength(s: string): number {
  return new TextEncoder().encode(s).byteLength;
}

function decodeBase64(b64: string): Uint8Array | null {
  try {
    // atob is available in Workers. Strips whitespace proactively.
    const cleaned = b64.replace(/\s+/g, "");
    const bin = atob(cleaned);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  } catch {
    return null;
  }
}

function validateBlob(
  label: "image" | "voice",
  raw: unknown,
  mimeRe: RegExp,
  maxBytes: number,
): ValidationResult | { ok: true; blob: { mime: string; base64: string; bytes: Uint8Array } } {
  if (!isPlainObject(raw)) {
    return fail("malformed_payload", `${label} must be an object with mime + base64.`);
  }
  const mime = raw["mime"];
  const base64 = raw["base64"];
  if (typeof mime !== "string" || !mimeRe.test(mime)) {
    return fail(`${label}_bad_mime` as ValidationError, `${label} has unsupported MIME type.`);
  }
  if (typeof base64 !== "string" || base64.length === 0) {
    return fail("malformed_payload", `${label}.base64 must be a non-empty string.`);
  }
  const bytes = decodeBase64(base64);
  if (bytes === null) {
    return fail("malformed_payload", `${label}.base64 is not valid base64.`);
  }
  if (bytes.byteLength > maxBytes) {
    const code: ValidationError = label === "image" ? "image_too_large" : "voice_too_large";
    // Derive the human-readable limit from the configured cap so a
    // future bump to IMAGE_MAX_BYTES / VOICE_MAX_BYTES doesn't leave
    // the error message stale.
    const mb = Math.round(maxBytes / 1_048_576);
    return fail(code, `${label[0]?.toUpperCase()}${label.slice(1)} exceeds ${mb} MB.`);
  }
  return { ok: true, blob: { mime, base64, bytes } };
}

export function validatePayload(raw: unknown): ValidationResult {
  if (!isPlainObject(raw)) {
    return fail("malformed_payload", "Request body must be a JSON object.");
  }

  // Title
  const titleRaw = raw["title"];
  if (typeof titleRaw !== "string") {
    return fail("title_required", "Title is required.");
  }
  const title = titleRaw.trim();
  if (title.length === 0) {
    return fail("title_required", "Title is required.");
  }
  if (title.length > TITLE_MAX) {
    return fail("title_too_long", `Title exceeds ${TITLE_MAX} characters.`);
  }

  // Body (may be empty).
  const bodyRaw = raw["body"];
  let body = "";
  if (typeof bodyRaw === "string") {
    body = bodyRaw;
  } else if (bodyRaw !== undefined && bodyRaw !== null) {
    return fail("malformed_payload", "Body must be a string.");
  }
  if (utf8ByteLength(body) > BODY_MAX_BYTES) {
    return fail("body_too_long", `Body exceeds ${BODY_MAX_BYTES} UTF-8 bytes.`);
  }

  // Category (optional)
  let category: Category | undefined;
  const catRaw = raw["category"];
  if (catRaw !== undefined && catRaw !== null && catRaw !== "") {
    if (typeof catRaw !== "string" || !CATEGORIES.includes(catRaw as Category)) {
      return fail("category_invalid", "Category is not one of the allowed values.");
    }
    category = catRaw as Category;
  }

  // Image (optional)
  let image: ValidatedImage | undefined;
  if (raw["image"] !== undefined && raw["image"] !== null) {
    const res = validateBlob("image", raw["image"], IMAGE_MIME_RE, IMAGE_MAX_BYTES);
    if ("ok" in res && res.ok === false) return res;
    if ("blob" in res) image = res.blob;
  }

  // Voice (optional)
  let voice: ValidatedVoice | undefined;
  if (raw["voice"] !== undefined && raw["voice"] !== null) {
    const res = validateBlob("voice", raw["voice"], VOICE_MIME_RE, VOICE_MAX_BYTES);
    if ("ok" in res && res.ok === false) return res;
    if ("blob" in res) voice = res.blob;
  }

  // Idempotency key
  const keyRaw = raw["idempotencyKey"];
  if (typeof keyRaw !== "string" || !UUID_RE.test(keyRaw)) {
    return fail("idempotency_key_invalid", "idempotencyKey must be a UUID string.");
  }

  // Report version
  const verRaw = raw["reportVersion"];
  if (typeof verRaw !== "string" || verRaw.length === 0 || verRaw.length > REPORT_VERSION_MAX) {
    return fail("report_version_invalid", `reportVersion must be a 1-${REPORT_VERSION_MAX} char string.`);
  }

  return {
    ok: true,
    data: {
      title,
      body,
      category,
      image,
      voice,
      idempotencyKey: keyRaw,
      reportVersion: verRaw,
    },
  };
}
