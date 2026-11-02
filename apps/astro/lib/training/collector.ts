export const MAX_COLLECTOR_BODY_BYTES = 4096;
export const MAX_COLLECTOR_VALUE_LENGTH = 1024;
export const COLLECTOR_FIELD_NAMES = ["campaign", "migrationJob", "directoryAlias", "entryUser", "legacyComputer", "relatedRequestId"] as const;
export type CollectorFieldName = (typeof COLLECTOR_FIELD_NAMES)[number];
export type CollectedFields = Partial<Record<CollectorFieldName, string>>;
const EXPECTED_SYNTHETIC_VALUES: Record<CollectorFieldName, string> = {
  campaign: "NIGHTWATCH",
  migrationJob: "ENG-1847",
  directoryAlias: "count",
  entryUser: "chopper",
  legacyComputer: "FCU-LEGACY1",
  relatedRequestId: "2003"
};

export function validateCollectedFields(value: unknown, astroApiKey = process.env.STONEHENGE_ASTRO_API_KEY ?? ""): CollectedFields {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("Collector fields are invalid");
  const entries = Object.entries(value);
  if (entries.length === 0 || entries.some(([name]) => !COLLECTOR_FIELD_NAMES.includes(name as CollectorFieldName))) throw new Error("Collector fields are not allowlisted");
  return Object.fromEntries(entries.map(([name, fieldValue]) => {
    const validated = validateCollectedValue(fieldValue, astroApiKey);
    if (validated !== EXPECTED_SYNTHETIC_VALUES[name as CollectorFieldName]) throw new Error("Collector value does not match synthetic metadata");
    return [name, validated];
  })) as CollectedFields;
}

export function validateCollectedValue(value: unknown, astroApiKey = process.env.STONEHENGE_ASTRO_API_KEY ?? "") {
  if (typeof value !== "string" || value.length === 0 || value.length > MAX_COLLECTOR_VALUE_LENGTH) throw new Error("Collector value is invalid");
  const lowered = value.toLowerCase();
  if ((astroApiKey && value.includes(astroApiKey)) || lowered.includes("astro-dev-key") || lowered.includes("stonehenge_api_key") || lowered.includes("api_key") || lowered.includes("api-key") || lowered.includes("client_secret") || lowered.includes("client-secret") || lowered.includes("authorization:") || lowered.includes("bearer ") || lowered.includes("password=") || lowered.includes("credential=") || lowered.includes("cookie:") || /^eyj[a-z0-9_-]+\./i.test(value)) {
    throw new Error("Credential-like values are not accepted by the training collector");
  }
  return value;
}

export function collectorDetection(submissionId: string, fields: CollectedFields, correlationId = submissionId) {
  return { findingId: "FCU-ASTRO-001", caller: "same-origin-training-collector", job: submissionId, fieldNames: Object.keys(fields).sort(), correlationId };
}
