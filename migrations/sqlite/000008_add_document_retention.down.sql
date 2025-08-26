-- Migration: 000008_add_document_retention.down.sql
-- Rollback document retention feature

DROP TRIGGER IF EXISTS update_documents_updated_at;
DROP INDEX IF EXISTS idx_documents_type_updated;
DROP INDEX IF EXISTS idx_documents_user_job;
DROP TABLE IF EXISTS documents;