import rawFeedbackContract from "../../render/assets/feedback-contract.json";

interface FeedbackContract {
  githubUrl: string;
  workerUrl: string;
  categories: readonly string[];
  limits: {
    titleMaxChars: number;
    bodyMaxBytes: number;
    imageMaxBytes: number;
    voiceMaxBytes: number;
    reportVersionMaxChars: number;
    legacyUrlMaxChars: number;
    workerTimeoutMs: number;
    requestMaxBytes: number;
    authorMaxChars: number;
  };
}

function assertPositiveInteger(value: number, field: string): void {
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error(`feedback contract ${field} must be a positive integer`);
  }
}

function assertFeedbackContract(contract: FeedbackContract): FeedbackContract {
  if (!contract.githubUrl || !contract.workerUrl) {
    throw new Error("feedback contract URLs are required");
  }
  if (!Array.isArray(contract.categories) || contract.categories.length === 0) {
    throw new Error("feedback contract categories are required");
  }
  for (const category of contract.categories) {
    if (typeof category !== "string" || category.length === 0) {
      throw new Error("feedback contract categories must be non-empty strings");
    }
  }
  for (const [field, value] of Object.entries(contract.limits)) {
    assertPositiveInteger(value, `limits.${field}`);
  }
  return contract;
}

export const feedbackContract = assertFeedbackContract(rawFeedbackContract);

export type Category = string;

export function isFeedbackCategory(value: string): value is Category {
  return feedbackContract.categories.includes(value);
}
