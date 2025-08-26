CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    job_id INTEGER NOT NULL,
    document_type TEXT NOT NULL CHECK(document_type IN ('cover_letter', 'resume')),
    content TEXT NOT NULL,
    format TEXT DEFAULT 'html',
    size_bytes INTEGER CHECK(size_bytes <= 2097152),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
    UNIQUE(user_id, job_id, document_type)
);

CREATE INDEX idx_documents_user_job ON documents(user_id, job_id);
CREATE INDEX idx_documents_type_updated ON documents(user_id, document_type, updated_at DESC);

