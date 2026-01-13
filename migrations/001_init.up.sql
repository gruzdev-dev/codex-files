CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id VARCHAR(255) NOT NULL,
    s3_path VARCHAR(512) NOT NULL,
    size BIGINT NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_files_owner_id ON files(owner_id);
CREATE INDEX idx_files_status ON files(status);
CREATE INDEX idx_files_is_deleted ON files(is_deleted);
