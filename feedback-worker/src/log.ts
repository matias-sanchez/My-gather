// Structured request log. Must never contain user text (title, body, blobs).
// Shape matches specs/003-feedback-backend-worker/contracts/api.md §observability,
// adapted to Issues (issue_id / issue_number) since this worker creates Issues.

export interface RequestLogFields {
  status: number;
  error: string | null;
  duration_ms: number;
  ip_hash: string;
  issue_id?: string | number | null;
  issue_number?: number | null;
  report_version?: string | null;
  rate_limit_count?: number | null;
  has_image?: boolean;
  has_voice?: boolean;
}

export function logRequest(fields: RequestLogFields): void {
  const record = {
    timestamp: new Date().toISOString(),
    status: fields.status,
    error: fields.error,
    duration_ms: fields.duration_ms,
    ip_hash: fields.ip_hash,
    issue_id: fields.issue_id ?? null,
    issue_number: fields.issue_number ?? null,
    report_version: fields.report_version ?? null,
    rate_limit_count: fields.rate_limit_count ?? null,
    has_image: fields.has_image ?? false,
    has_voice: fields.has_voice ?? false,
  };
  console.log(JSON.stringify(record));
}
