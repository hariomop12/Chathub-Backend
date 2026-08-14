-- migrate:up
ALTER TABLE messages ADD COLUMN seq bigserial NOT NULL;
ALTER TABLE messages ADD COLUMN client_message_id text;

CREATE UNIQUE INDEX idx_messages_seq ON messages (seq);
CREATE INDEX idx_messages_chat_seq ON messages (chat_id, seq DESC);
CREATE UNIQUE INDEX idx_messages_client_msg ON messages (chat_id, client_message_id)
    WHERE client_message_id IS NOT NULL;

-- migrate:down
DROP INDEX IF EXISTS idx_messages_client_msg;
DROP INDEX IF EXISTS idx_messages_chat_seq;
DROP INDEX IF EXISTS idx_messages_seq;
ALTER TABLE messages DROP COLUMN IF EXISTS client_message_id;
ALTER TABLE messages DROP COLUMN IF EXISTS seq;
