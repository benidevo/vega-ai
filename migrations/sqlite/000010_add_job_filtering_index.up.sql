-- Add composite index for common job filtering pattern:
-- WHERE user_id = ? AND status IN (...) ORDER BY match_score DESC
-- This optimizes the most common query pattern in job listings
CREATE INDEX IF NOT EXISTS idx_jobs_user_status_score ON jobs(user_id, status, match_score DESC);
