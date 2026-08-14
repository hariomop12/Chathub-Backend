# Message Send Flow (Phase 1)

> Live source of truth: mirrors the implemented design.
> Redis (Upstash) + transactional outbox = reliable fan-out.

## Core principle

- DB = source of truth
- Redis + WS = live cache

Kuch bhi choote toh history API (keyset pagination) ya resync se milta hai.
Koi message permanently lose nahi ho sakta.

## Message send — full flow

    Client -> send -> Handler / WS Hub
      -> MessageService.SendMessage
         1. Membership check (chat_members)
         2. Transaction start
            a. INSERT message (idempotent on (chat_id, client_message_id), ON CONFLICT DO NOTHING)
            b. INSERT outbox_events (same tx, unique on message_id)
         3. Commit
      -> isNew=false: existing message returned as-is
      -> isNew=true: OutboxWorker (poll ~200ms) claims row
         -> Redis PUBLISH chat:{chatId}
         -> sabhi instances (lazy subscribe) -> local WS clients in room
            -> receive-message event

### Fan-out decision

- Redis configured: sender's own instance bhi Redis se hi leta hai (client ne
  join-room kiya hai -> instance subscribed). No in-process broadcast.
  Latency ~200ms (poll interval).
- Redis NOT configured (degraded single-instance mode): direct in-process
  BroadcastToRoom. Local dev me chalta hai, multi-instance me unreliable.
- HTTP POST /messages/{chatId}: sender ko message synchronously response me
  milta hai, WS ki zaroorat nahi.

## Reliability guarantees

| Failure | Kya hota hai | Fix |
|---|---|---|
| Crash after DB write, before publish | Outbox row pending rehti hai | Worker restart pe utha leta hai |
| 2 workers same row | FOR UPDATE SKIP LOCKED | Ek row ek hi worker |
| Redis down | Publish fail -> MarkFailed + exponential backoff | Message DB me safe, retry hota hai |
| Retry same message | clientMessageId conflict | ON CONFLICT DO NOTHING -> purana message return |
| Worker crash after publish, before mark | Duplicate publish | At-least-once; clients seq se reorder |
| Message ordering | Worker ORDER BY id claim | Client seq se reorder |

## Keyset pagination (history)

    GET /api/v1/messages/{chatId}?cursor=<seq>&limit=30

    WHERE chat_id = ? AND seq < ?    -- cursor = last message ka seq
    ORDER BY seq DESC
    LIMIT ?

Response:

    {
      "messages": [...],
      "nextCursor": "42"    // tabhi hai jab aur page bache
    }

Cursor string hai, `seq` (bigserial) — timestamp nahi, isliye no duplicates.

## WS resync (mobile reconnect)

    Client -> {"type":"resync","chatId":"...","afterSeq":123}
    Server -> {"type":"resync-messages","data":{"chatId":"...","messages":[...]}}

    WHERE chat_id = ? AND seq > ? ORDER BY seq ASC LIMIT 200

Choota hua kuch nahi, duplicate bhi nahi.

## Idempotent send

    {"type":"send-message","data":{"chatId":"...","content":"hi","clientMessageId":"<uuid>"}}

- Unique index: (chat_id, client_message_id) WHERE client_message_id IS NOT NULL
- Retry pe wahi ID -> existing message return, nayi row nahi banti.

## WS events (client API)

Client -> server:

| Event | Data |
|---|---|
| register-user | {userId} (fallback; recommended: JWT via ?token=) |
| join-room | {chatId} (membership checked) |
| leave-room | {chatId} |
| send-message | {chatId, content, clientMessageId?, fileUrl?...} |
| resync | {chatId, afterSeq} |
| typing / stop-typing | {chatId, ...} |
| register-peer / get-peer-id / call events | voice/video signaling |

Server -> client:

| Event |
|---|
| receive-message {message} |
| resync-messages {chatId, messages} |
| send-message-error {error} |
| user-presence {userId, online} |
| user-typing / user-stop-typing |
| peer-id-response, incoming-call, user-busy, call events |

## Auth & hardening

- WS upgrade: Clerk JWT via ?token= -> userID bound to connection.
  register-user fallback abhi bhi supported (backward compat).
- send-message without userID -> send-message-error.
- join-room -> chat_members membership check, warna join-room-error.
- Read limit 64KB, ping/pong heartbeat (30s ping, 60s deadline).
- No "broadcast to ALL clients" fallback — receive-message sirf room members ko.

## Outbox table

    CREATE TABLE outbox_events (
      id              bigserial PRIMARY KEY,
      message_id      uuid NOT NULL,        -- UNIQUE -> duplicate enqueue impossible
      chat_id         uuid NOT NULL,
      status          text NOT NULL DEFAULT 'pending',  -- pending | published | failed
      attempts        int  NOT NULL DEFAULT 0,
      next_attempt_at timestamptz NOT NULL DEFAULT now(),
      published_at    timestamptz,
      created_at      timestamptz NOT NULL DEFAULT now()
    );
    CREATE UNIQUE INDEX idx_outbox_message_id ON outbox_events (message_id);
    CREATE INDEX idx_outbox_pending ON outbox_events (status, next_attempt_at);
    CREATE INDEX idx_outbox_chat_id ON outbox_events (chat_id);

Worker: claim `FOR UPDATE SKIP LOCKED`, publish, mark published; failures ->
MarkFailed with exponential backoff; attempts >= 10 -> status = failed (manual review).
