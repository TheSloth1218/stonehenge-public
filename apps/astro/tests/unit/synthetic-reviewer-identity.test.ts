import { describe, expect, it, vi } from "vitest";
import type { BrowserContext } from "playwright";
import { REVIEWER_IDENTITY_KEY, REVIEWER_TOKEN_KEY, SYNTHETIC_REVIEWER_METADATA, SYNTHETIC_REVIEWER_TOKEN, SyntheticTrainingReviewerIdentityProvider } from "@/reviewer/identity";

describe("synthetic reviewer identity", () => {
  it("seeds only the documented synthetic token and metadata", async () => {
    const addInitScript = vi.fn();
    const provider = new SyntheticTrainingReviewerIdentityProvider();
    await provider.prepareContext({ addInitScript } as unknown as BrowserContext, { astroOrigin: "http://astro", submissionId: "id" });
    expect(addInitScript).toHaveBeenCalledOnce();
    expect(addInitScript.mock.calls[0][1]).toEqual({ astroOrigin: "http://astro", tokenKey: REVIEWER_TOKEN_KEY, token: SYNTHETIC_REVIEWER_TOKEN, identityKey: REVIEWER_IDENTITY_KEY, metadata: SYNTHETIC_REVIEWER_METADATA });
    expect(SYNTHETIC_REVIEWER_METADATA).toEqual({ campaign: "NIGHTWATCH", migrationJob: "ENG-1847", directoryAlias: "count", entryUser: "chopper", legacyComputer: "FCU-LEGACY1", relatedRequestId: "2003" });
    expect(SYNTHETIC_REVIEWER_TOKEN).not.toContain("eyJ");
  });
});
