-- Add content_hash column for duplicate detection
ALTER TABLE jobs ADD COLUMN content_hash TEXT;

-- Create unique index for source URL deduplication
CREATE UNIQUE INDEX idx_jobs_user_source_url 
ON jobs(user_id, source_url) 
WHERE source_url IS NOT NULL AND source_url != '';

-- Create index for content-based similarity searches
CREATE INDEX idx_jobs_content_hash 
ON jobs(user_id, content_hash) 
WHERE content_hash IS NOT NULL;