import { mkdirSync } from "node:fs";
import { dirname } from "node:path";
import { randomUUID } from "node:crypto";
import { DatabaseSync } from "node:sqlite";

export const REVIEW_STATES = ["queued", "claimed", "visiting", "completed", "retryable_failed", "permanently_failed", "expired"] as const;
export type ReviewState = (typeof REVIEW_STATES)[number];

export type ReviewJob = {
  submissionId: string;
  targetId: string;
  reason: string;
  reviewUrl: string;
  createdAt: number;
  updatedAt: number;
  expiresAt: number;
  availableAt: number;
  attempts: number;
  maxAttempts: number;
  leaseOwner: string | null;
  leaseExpiresAt: number | null;
  lastErrorCategory: string | null;
  completedAt: number | null;
  state: ReviewState;
};

type SqlRow = Record<string, string | number | null>;

export class ReviewQueue {
  private readonly db: DatabaseSync;

  constructor(readonly databasePath: string) {
    mkdirSync(dirname(databasePath), { recursive: true });
    this.db = new DatabaseSync(databasePath);
    this.db.exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;");
    this.migrate();
  }

  close() {
    this.db.close();
  }

  createJob(input: { targetId: string; reason: string; reviewUrl: string; retentionMs: number; maxAttempts: number }, now = Date.now()) {
    const submissionId = randomUUID();
    this.db.prepare(`INSERT INTO review_jobs (
      submission_id, target_id, reason, review_url, created_at, updated_at, expires_at, available_at,
      attempts, max_attempts, state
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, 'queued')`).run(
      submissionId, input.targetId, input.reason, input.reviewUrl, now, now, now + input.retentionMs, now, input.maxAttempts
    );
    return this.getJob(submissionId)!;
  }

  replaceReviewUrl(submissionId: string, reviewUrl: string, now = Date.now()) {
    this.db.prepare("UPDATE review_jobs SET review_url = ?, updated_at = ? WHERE submission_id = ? AND state = 'queued'").run(reviewUrl, now, submissionId);
    return this.getJob(submissionId);
  }

  getJob(submissionId: string) {
    return mapJob(this.db.prepare("SELECT * FROM review_jobs WHERE submission_id = ?").get(submissionId) as SqlRow | undefined);
  }

  claimNext(workerId: string, leaseMs: number, now = Date.now()): ReviewJob | null {
    this.db.exec("BEGIN IMMEDIATE");
    try {
      this.recoverStaleLeases(now);
      this.expireJobs(now);
      const row = this.db.prepare(`SELECT submission_id FROM review_jobs
        WHERE state IN ('queued', 'retryable_failed') AND available_at <= ? AND expires_at > ? AND attempts < max_attempts
        ORDER BY created_at ASC LIMIT 1`).get(now, now) as SqlRow | undefined;
      if (!row) {
        this.db.exec("COMMIT");
        return null;
      }
      const submissionId = String(row.submission_id);
      const result = this.db.prepare(`UPDATE review_jobs SET state = 'claimed', attempts = attempts + 1,
        lease_owner = ?, lease_expires_at = ?, updated_at = ?
        WHERE submission_id = ? AND state IN ('queued', 'retryable_failed')`).run(workerId, now + leaseMs, now, submissionId);
      this.db.exec("COMMIT");
      return Number(result.changes) === 1 ? this.getJob(submissionId) : null;
    } catch (error) {
      this.db.exec("ROLLBACK");
      throw error;
    }
  }

  markVisiting(submissionId: string, workerId: string, leaseMs: number, now = Date.now()) {
    return this.transitionOwned(submissionId, workerId, "claimed", "visiting", now, now + leaseMs);
  }

  complete(submissionId: string, workerId: string, now = Date.now()) {
    const result = this.db.prepare(`UPDATE review_jobs SET state = 'completed', completed_at = ?, updated_at = ?,
      lease_owner = NULL, lease_expires_at = NULL WHERE submission_id = ? AND lease_owner = ? AND state IN ('claimed', 'visiting')`)
      .run(now, now, submissionId, workerId);
    const current = this.getJob(submissionId);
    return Number(result.changes) === 1 || current?.state === "completed";
  }

  fail(submissionId: string, workerId: string, category: string, now = Date.now(), baseBackoffMs = 1_000) {
    const job = this.getJob(submissionId);
    if (!job || job.leaseOwner !== workerId || !["claimed", "visiting"].includes(job.state)) return false;
    const terminal = job.attempts >= job.maxAttempts || job.expiresAt <= now;
    const state: ReviewState = job.expiresAt <= now ? "expired" : terminal ? "permanently_failed" : "retryable_failed";
    const backoff = Math.min(60_000, baseBackoffMs * 2 ** Math.max(0, job.attempts - 1));
    this.db.prepare(`UPDATE review_jobs SET state = ?, available_at = ?, updated_at = ?, lease_owner = NULL,
      lease_expires_at = NULL, last_error_category = ? WHERE submission_id = ? AND lease_owner = ?`)
      .run(state, now + backoff, now, category.slice(0, 80), submissionId, workerId);
    return true;
  }

  cleanup(now = Date.now()) {
    this.expireJobs(now);
    this.db.prepare("DELETE FROM collector_results WHERE expires_at <= ?").run(now);
    const deleted = this.db.prepare("DELETE FROM review_jobs WHERE expires_at <= ? AND state IN ('completed', 'permanently_failed', 'expired')").run(now - 60_000);
    this.db.prepare("DELETE FROM review_rate_limits WHERE window_start < ?").run(now - 5 * 60_000);
    return Number(deleted.changes);
  }

  consumeRateLimit(key: string, limit: number, windowMs: number, now = Date.now()) {
    const windowStart = Math.floor(now / windowMs) * windowMs;
    this.db.exec("BEGIN IMMEDIATE");
    try {
      const row = this.db.prepare("SELECT count FROM review_rate_limits WHERE key = ? AND window_start = ?").get(key, windowStart) as SqlRow | undefined;
      const count = Number(row?.count ?? 0);
      if (count >= limit) {
        this.db.exec("COMMIT");
        return false;
      }
      this.db.prepare(`INSERT INTO review_rate_limits (key, window_start, count) VALUES (?, ?, 1)
        ON CONFLICT(key, window_start) DO UPDATE SET count = count + 1`).run(key, windowStart);
      this.db.exec("COMMIT");
      return true;
    } catch (error) {
      this.db.exec("ROLLBACK");
      throw error;
    }
  }

  storeCollectorValue(submissionId: string, value: string, retentionMs: number, now = Date.now()) {
    const job = this.getJob(submissionId);
    if (!job || job.expiresAt <= now) return false;
    this.db.prepare(`INSERT INTO collector_results (submission_id, value, received_at, expires_at) VALUES (?, ?, ?, ?)
      ON CONFLICT(submission_id) DO UPDATE SET value = excluded.value, received_at = excluded.received_at,
      expires_at = excluded.expires_at`).run(submissionId, value, now, Math.min(job.expiresAt, now + retentionMs));
    return true;
  }

  getCollectorValue(submissionId: string, now = Date.now()) {
    const row = this.db.prepare(`SELECT c.submission_id, c.value, c.received_at FROM collector_results c
      JOIN review_jobs j ON j.submission_id = c.submission_id
      WHERE c.submission_id = ? AND c.expires_at > ? AND j.expires_at > ?`).get(submissionId, now, now) as SqlRow | undefined;
    return row ? { submissionId: String(row.submission_id), value: String(row.value), receivedAt: Number(row.received_at) } : null;
  }

  private transitionOwned(submissionId: string, workerId: string, from: ReviewState, to: ReviewState, now: number, leaseExpiry: number) {
    const result = this.db.prepare("UPDATE review_jobs SET state = ?, updated_at = ?, lease_expires_at = ? WHERE submission_id = ? AND lease_owner = ? AND state = ?")
      .run(to, now, leaseExpiry, submissionId, workerId, from);
    return Number(result.changes) === 1;
  }

  private recoverStaleLeases(now: number) {
    this.db.prepare(`UPDATE review_jobs SET
      state = CASE WHEN attempts >= max_attempts THEN 'permanently_failed' ELSE 'retryable_failed' END,
      available_at = ?, updated_at = ?, lease_owner = NULL, lease_expires_at = NULL,
      last_error_category = 'lease_expired'
      WHERE state IN ('claimed', 'visiting') AND lease_expires_at <= ?`).run(now, now, now);
  }

  private expireJobs(now: number) {
    this.db.prepare(`UPDATE review_jobs SET state = 'expired', updated_at = ?, lease_owner = NULL, lease_expires_at = NULL
      WHERE expires_at <= ? AND state NOT IN ('completed', 'permanently_failed', 'expired')`).run(now, now);
  }

  private migrate() {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS review_jobs (
        submission_id TEXT PRIMARY KEY, target_id TEXT NOT NULL, reason TEXT NOT NULL, review_url TEXT NOT NULL,
        created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, available_at INTEGER NOT NULL,
        attempts INTEGER NOT NULL DEFAULT 0, max_attempts INTEGER NOT NULL, lease_owner TEXT, lease_expires_at INTEGER,
        last_error_category TEXT, completed_at INTEGER, state TEXT NOT NULL CHECK (state IN ('queued','claimed','visiting','completed','retryable_failed','permanently_failed','expired'))
      );
      CREATE INDEX IF NOT EXISTS review_jobs_claimable ON review_jobs(state, available_at, created_at);
      CREATE TABLE IF NOT EXISTS collector_results (
        submission_id TEXT PRIMARY KEY REFERENCES review_jobs(submission_id) ON DELETE CASCADE,
        value TEXT NOT NULL, received_at INTEGER NOT NULL, expires_at INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS review_rate_limits (
        key TEXT NOT NULL, window_start INTEGER NOT NULL, count INTEGER NOT NULL,
        PRIMARY KEY (key, window_start)
      );
    `);
  }
}

function mapJob(row: SqlRow | undefined): ReviewJob | null {
  if (!row) return null;
  return {
    submissionId: String(row.submission_id), targetId: String(row.target_id), reason: String(row.reason), reviewUrl: String(row.review_url),
    createdAt: Number(row.created_at), updatedAt: Number(row.updated_at), expiresAt: Number(row.expires_at), availableAt: Number(row.available_at),
    attempts: Number(row.attempts), maxAttempts: Number(row.max_attempts), leaseOwner: nullableString(row.lease_owner),
    leaseExpiresAt: row.lease_expires_at == null ? null : Number(row.lease_expires_at), lastErrorCategory: nullableString(row.last_error_category),
    completedAt: row.completed_at == null ? null : Number(row.completed_at), state: String(row.state) as ReviewState
  };
}

function nullableString(value: string | number | null) {
  return value == null ? null : String(value);
}
