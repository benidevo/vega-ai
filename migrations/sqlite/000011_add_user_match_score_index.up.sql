-- Add composite index for top-matches query pattern:
-- WHERE user_id = ? AND match_score >= ? ORDER BY match_score DESC
CREATE INDEX IF NOT EXISTS idx_jobs_user_match_score ON jobs(user_id, match_score DESC);
