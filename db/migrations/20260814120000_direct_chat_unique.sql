-- migrate:up
-- enforce: at most one direct chat per user pair
-- adds member_a/member_b (sorted user id pair) on chats, backfills existing
-- direct chats, removes any duplicates (keeping the oldest), then locks it in
-- with a partial unique index.

ALTER TABLE chats ADD COLUMN IF NOT EXISTS member_a TEXT;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS member_b TEXT;

-- backfill pair from the two members of each existing direct chat
UPDATE chats c
SET member_a = m.low, member_b = m.high
FROM (
    SELECT chat_id, MIN(user_id) AS low, MAX(user_id) AS high
    FROM chat_members
    GROUP BY chat_id
    HAVING COUNT(*) = 2
) m
WHERE m.chat_id = c.id AND c.is_group = false;

-- de-duplicate existing direct chats, keeping the oldest per pair
-- (messages + chat_members are removed via ON DELETE CASCADE)
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY member_a, member_b ORDER BY created_at ASC) AS rn
    FROM chats
    WHERE is_group = false AND member_a IS NOT NULL
)
DELETE FROM chats WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

CREATE UNIQUE INDEX IF NOT EXISTS uq_direct_chats_pair
    ON chats (member_a, member_b)
    WHERE is_group = false;

-- migrate:down
DROP INDEX IF EXISTS uq_direct_chats_pair;
ALTER TABLE chats DROP COLUMN IF EXISTS member_a;
ALTER TABLE chats DROP COLUMN IF EXISTS member_b;

