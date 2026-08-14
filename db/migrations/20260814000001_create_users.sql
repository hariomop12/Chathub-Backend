-- migrate:up
CREATE TABLE users (
    id       TEXT PRIMARY KEY,
    username TEXT NOT NULL DEFAULT 'Anonymous',
    email    TEXT NOT NULL DEFAULT '',
    avatar   TEXT
);

-- migrate:down
DROP TABLE IF EXISTS users;
