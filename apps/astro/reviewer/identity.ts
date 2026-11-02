import type { BrowserContext } from "playwright";

export const SYNTHETIC_REVIEWER_TOKEN = "stonehenge-training-employee-session-v1";
export const REVIEWER_TOKEN_KEY = "sca_reviewer_access_token";
export const REVIEWER_IDENTITY_KEY = "sca_reviewer_identity";
export const SYNTHETIC_REVIEWER_METADATA = {
  campaign: "NIGHTWATCH",
  migrationJob: "ENG-1847",
  directoryAlias: "count",
  entryUser: "chopper",
  legacyComputer: "FCU-LEGACY1",
  relatedRequestId: "2003"
} as const;

export interface ReviewerIdentityProvider {
  readonly name: string;
  prepareContext(context: BrowserContext, options: { astroOrigin: string; submissionId: string }): Promise<void>;
  verifyContext?(context: BrowserContext, options: { astroOrigin: string }): Promise<boolean>;
}

export class SyntheticTrainingReviewerIdentityProvider implements ReviewerIdentityProvider {
  readonly name = "synthetic-training";

  async prepareContext(context: BrowserContext, options: { astroOrigin: string; submissionId: string }) {
    await context.addInitScript(
      ({ astroOrigin, tokenKey, token, identityKey, metadata }) => {
        if (window.location.origin !== astroOrigin) return;
        localStorage.setItem(tokenKey, token);
        localStorage.setItem(identityKey, JSON.stringify({ username: "sca.mapreview", roles: ["employee.basic"], mode: "training-synthetic", metadata }));
      },
      { astroOrigin: options.astroOrigin, tokenKey: REVIEWER_TOKEN_KEY, token: SYNTHETIC_REVIEWER_TOKEN, identityKey: REVIEWER_IDENTITY_KEY, metadata: SYNTHETIC_REVIEWER_METADATA }
    );
  }

  async verifyContext(context: BrowserContext, options: { astroOrigin: string }) {
    const page = context.pages()[0];
    if (!page || new URL(page.url()).origin !== options.astroOrigin) return false;
    return page.evaluate(
      ({ tokenKey, token, identityKey, metadata }) => {
        const identity = JSON.parse(localStorage.getItem(identityKey) ?? "null") as { username?: string; roles?: string[]; mode?: string; metadata?: unknown } | null;
        return localStorage.getItem(tokenKey) === token && identity?.username === "sca.mapreview" && identity.mode === "training-synthetic" && identity.roles?.includes("employee.basic") === true && JSON.stringify(identity.metadata) === JSON.stringify(metadata);
      },
      { tokenKey: REVIEWER_TOKEN_KEY, token: SYNTHETIC_REVIEWER_TOKEN, identityKey: REVIEWER_IDENTITY_KEY, metadata: SYNTHETIC_REVIEWER_METADATA }
    );
  }
}

/*
Future identity boundary:
AD -> Dex over LDAPS -> OIDC identity -> isolated reviewer context.
A DexReviewerIdentityProvider will maintain refreshed storage state for the low-privilege
sca.mapreview / employee.basic identity and implement this interface. Queue and worker code remain unchanged.
*/
