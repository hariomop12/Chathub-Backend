-- migrate:up
CREATE TABLE messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id    UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id  TEXT NOT NULL,
    content    TEXT NOT NULL DEFAULT '',
    file_url   TEXT,
    file_name  TEXT,
    file_type  TEXT,
    file_size  BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_chat_created ON messages (chat_id, created_at, id);

-- migrate:down
DROP TABLE IF EXISTS messages;
