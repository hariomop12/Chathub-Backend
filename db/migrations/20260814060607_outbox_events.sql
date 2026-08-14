-- migrate:up
CREATE TABLE outbox_events (
    id              bigserial PRIMARY KEY,
    message_id      uuid NOT NULL,
    chat_id         uuid NOT NULL,
    status          text NOT NULL DEFAULT 'pending',
    attempts        integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    published_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_outbox_message_id ON outbox_events (message_id);
CREATE INDEX idx_outbox_pending ON outbox_events (status, next_attempt_at);
CREATE INDEX idx_outbox_chat_id ON outbox_events (chat_id);

-- migrate:down
DROP TABLE IF EXISTS outbox_events;

